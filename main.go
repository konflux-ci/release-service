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
	"fmt"
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

type ControllerFlags struct {
	metricsAddr              string
	enableHttp2              bool
	enableLeaderElection     bool
	leaderRenewDeadline      time.Duration
	leaseDuration            time.Duration
	leaderElectorRetryPeriod time.Duration
	probeAddr                string
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appstudiov1alpha1.AddToScheme(scheme))
	utilruntime.Must(applicationapiv1alpha1.AddToScheme(scheme))
	utilruntime.Must(ecapiv1alpha1.AddToScheme(scheme))
	utilruntime.Must(tektonv1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

// setupManager creates and returns a new ctrl.Manager instance
func setupManager() ctrl.Manager {
	cf := ControllerFlags{}
	readControllerFlags(&cf)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&tektonv1.PipelineRun{}: cache.ByObject{
					Label: labels.SelectorFromSet(labels.Set{metadata.ServiceNameLabel: metadata.ServiceName}),
				},
			},
		},
		HealthProbeBindAddress: cf.probeAddr,
		LeaderElection:         cf.enableLeaderElection,
		LeaderElectionID:       "f3d4c01a.redhat.com",
		RenewDeadline:          &cf.leaderRenewDeadline,
		LeaseDuration:          &cf.leaseDuration,
		RetryPeriod:            &cf.leaderElectorRetryPeriod,
		Metrics: server.Options{
			BindAddress: cf.metricsAddr,
		},
		WebhookServer: crwebhook.NewServer(crwebhook.Options{
			Port: 9443,
			TLSOpts: []func(*tls.Config){
				func(c *tls.Config) {
					if !cf.enableHttp2 {
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

	return mgr
}

// readControllerFlags reads the command line arguments, binds them to the controller, parses and sets up the controller
// logger
func readControllerFlags(c *ControllerFlags) {
	flag.StringVar(&c.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&c.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&c.enableHttp2, "enable-http2", false, "Enable HTTP/2 for the metrics and webhook servers.")
	flag.BoolVar(&c.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&c.leaderRenewDeadline, "leader-renew-deadline", 10*time.Second,
		"Leader RenewDeadline is the duration that the acting controlplane "+
			"will retry refreshing leadership before giving up.")
	flag.DurationVar(&c.leaseDuration, "lease-duration", 15*time.Second,
		"Lease Duration is the duration that non-leader candidates will wait to force acquire leadership.")
	flag.DurationVar(&c.leaderElectorRetryPeriod, "leader-elector-retry-period", 2*time.Second, "RetryPeriod is the duration the "+
		"LeaderElector clients should wait between tries of actions.")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
}

// loadVariableFromEnv loads an environment variable setting a default value in case of empty or unset
func loadVariableFromEnv(name, defaultValue string) {
	if os.Getenv(name) == "" {
		err := os.Setenv(name, defaultValue)
		if err != nil {
			setupLog.Error(err, fmt.Sprintf("unable to setup %s environment variable", name))
			os.Exit(1)
		}
	}
	setupLog.Info(fmt.Sprintf("loaded env var `%s` with value `%s`", name, os.Getenv(name)))
}

func main() {
	mgr := setupManager()

	loadVariableFromEnv("DEFAULT_RELEASE_PVC", "release-pvc")
	loadVariableFromEnv("DEFAULT_RELEASE_WORKSPACE_NAME", "release-workspace")
	loadVariableFromEnv("DEFAULT_RELEASE_WORKSPACE_SIZE", "1Gi")
	loadVariableFromEnv("ENTERPRISE_CONTRACT_CONFIG_MAP", "enterprise-contract-service/ec-defaults")

	setUpControllers(mgr)
	setUpWebhooks(mgr)

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
