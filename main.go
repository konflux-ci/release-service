/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"time"

	"github.com/konflux-ci/operator-toolkit/controller"
	"github.com/konflux-ci/operator-toolkit/webhook"
	"github.com/konflux-ci/release-service/api/v1alpha1/webhooks"
	"github.com/konflux-ci/release-service/metadata"

	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	appstudiov1alpha1 "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appstudiov1alpha1.AddToScheme(scheme))
	utilruntime.Must(applicationapiv1alpha1.AddToScheme(scheme))
	utilruntime.Must(ecapiv1alpha1.AddToScheme(scheme))
	utilruntime.Must(tektonv1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		enableHTTP2              bool
		enableLeaderElection     bool
		leaderElectorRetryPeriod time.Duration
		leaderRenewDeadline      time.Duration
		leaseDuration            time.Duration
		metricsAddr              string
		probeAddr                string
		secureMetrics            bool
		tlsOpts                  []func(*tls.Config)
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&leaderRenewDeadline, "leader-renew-deadline", 10*time.Second,
		"Leader RenewDeadline is the duration that the acting controlplane "+
			"will retry refreshing leadership before giving up.")
	flag.DurationVar(&leaseDuration, "lease-duration", 15*time.Second,
		"Lease Duration is the duration that non-leader candidates will wait to force acquire leadership.")
	flag.DurationVar(&leaderElectorRetryPeriod, "leader-elector-retry-period", 2*time.Second, "RetryPeriod is the duration the "+
		"LeaderElector clients should wait between tries of actions.")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := crwebhook.NewServer(crwebhook.Options{
		TLSOpts: tlsOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		// TODO(user): TLSOpts is used to allow configuring the TLS config used for the server. If certificates are
		// not provided, self-signed certificates will be generated by default. This option is not recommended for
		// production environments as self-signed certificates do not offer the same level of trust and security
		// as certificates issued by a trusted Certificate Authority (CA). The primary risk is potentially allowing
		// unauthorized access to sensitive metrics data. Consider replacing with CertDir, CertName, and KeyName
		// to provide certificates, ensuring the server communicates using trusted and secure certificates.
		TLSOpts: tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				// we want to cache PipelineRuns only created by this operator.
				&tektonv1.PipelineRun{}: cache.ByObject{
					Label: labels.SelectorFromSet(labels.Set{metadata.ServiceNameLabel: metadata.ServiceName}),
				},
				// also cache other watched objects, but no filter is required.
				&appstudiov1alpha1.Release{}:              {},
				&appstudiov1alpha1.ReleasePlan{}:          {},
				&appstudiov1alpha1.ReleasePlanAdmission{}: {},
				// objects that the operator does not watch, but are used by it.
				&appstudiov1alpha1.ReleaseServiceConfig{}: {},
				&applicationapiv1alpha1.Snapshot{}:        {},
				&applicationapiv1alpha1.Application{}:     {},
			},
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f3d4c01a.redhat.com",
		LeaseDuration:          &leaseDuration,
		Metrics:                metricsServerOptions,
		RenewDeadline:          &leaderRenewDeadline,
		RetryPeriod:            &leaderElectorRetryPeriod,
		Scheme:                 scheme,
		WebhookServer:          webhookServer,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Set a default value for the DEFAULT_RELEASE_PVC environment variable
	if os.Getenv("DEFAULT_RELEASE_PVC") == "" {
		err := os.Setenv("DEFAULT_RELEASE_PVC", "release-pvc")
		if err != nil {
			setupLog.Error(err, "unable to setup DEFAULT_RELEASE_PVC environment variable")
			os.Exit(1)
		}
	}

	// Set a default value for the DEFAULT_RELEASE_WORKSPACE_NAME environment variable
	if os.Getenv("DEFAULT_RELEASE_WORKSPACE_NAME") == "" {
		err := os.Setenv("DEFAULT_RELEASE_WORKSPACE_NAME", "release-workspace")
		if err != nil {
			setupLog.Error(err, "unable to setup DEFAULT_RELEASE_WORKSPACE_NAME environment variable")
			os.Exit(1)
		}
	}

	// Set a default value for the DEFAULT_RELEASE_WORKSPACE_SIZE environment variable
	if os.Getenv("DEFAULT_RELEASE_WORKSPACE_SIZE") == "" {
		err := os.Setenv("DEFAULT_RELEASE_WORKSPACE_SIZE", "1Gi")
		if err != nil {
			setupLog.Error(err, "unable to setup DEFAULT_RELEASE_WORKSPACE_SIZE environment variable")
			os.Exit(1)
		}
	}

	setUpControllers(mgr)
	setUpWebhooks(mgr)

	err = os.Setenv("ENTERPRISE_CONTRACT_CONFIG_MAP", "enterprise-contract-service/ec-defaults")
	if err != nil {
		setupLog.Error(err, "unable to setup ENTERPRISE_CONTRACT_CONFIG_MAP environment variable")
		os.Exit(1)
	}

	err = os.Setenv("MOBSTER_CONFIG_MAP", "mobster/mobster-defaults")
	if err != nil {
		setupLog.Error(err, "unable to setup MOBSTER_CONFIG_MAP environment variable")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// setUpControllers sets up controllers.
func setUpControllers(mgr ctrl.Manager) {
	err := controller.SetupControllers(mgr, nil, controllers.EnabledControllers...)
	if err != nil {
		setupLog.Error(err, "unable to setup controllers")
		os.Exit(1)
	}
}

// setUpWebhooks sets up webhooks.
func setUpWebhooks(mgr ctrl.Manager) {
	if os.Getenv("ENABLE_WEBHOOKS") == "false" {
		return
	}

	err := webhook.SetupWebhooks(mgr, webhooks.EnabledWebhooks...)
	if err != nil {
		setupLog.Error(err, "unable to setup webhooks")
		os.Exit(1)
	}
}
