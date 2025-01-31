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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

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
		metricsAddr              string
		enableHttp2              bool
		enableLeaderElection     bool
		probeAddr                string
		leaderRenewDeadline      time.Duration
		leaseDuration            time.Duration
		leaderElectorRetryPeriod time.Duration
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableHttp2, "enable-http2", false, "Enable HTTP/2 for the metrics and webhook servers.")
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
		RenewDeadline:          &leaderRenewDeadline,
		LeaseDuration:          &leaseDuration,
		RetryPeriod:            &leaderElectorRetryPeriod,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: crwebhook.NewServer(crwebhook.Options{
			Port: 9443,
			TLSOpts: []func(*tls.Config){
				func(c *tls.Config) {
					if !enableHttp2 {
						c.NextProtos = []string{"http/1.1"}
					}
				},
			},
		}),
		Scheme: scheme,
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
