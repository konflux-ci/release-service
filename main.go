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
	"flag"
	"github.com/redhat-appstudio/operator-toolkit/controller"
	"github.com/redhat-appstudio/operator-toolkit/webhook"
	"github.com/redhat-appstudio/release-service/api/v1alpha1/webhooks"
	"github.com/redhat-appstudio/release-service/controllers"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
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
	utilruntime.Must(tektonv1beta1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	mgr := createManager()
	setEnvironmentVariablesDefaults()
	setUpControllers(mgr)
	setUpWebhooks(mgr)

	//+kubebuilder:scaffold:builder

	setHealthAndReadyChecks(mgr)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func createManager() ctrl.Manager {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f3d4c01a.redhat.com",
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	return mgr
}

// setEnvironmentVariablesDefaults set defaults values for the declared environment variables
func setEnvironmentVariablesDefaults() {
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

	// Set a default value for the ENTERPRISE_CONTRACT_CONFIG_MAP environment variable
	if os.Getenv("ENTERPRISE_CONTRACT_CONFIG_MAP") == "" {
		err := os.Setenv("ENTERPRISE_CONTRACT_CONFIG_MAP", "enterprise-contract-service/ec-defaults")
		if err != nil {
			setupLog.Error(err, "unable to setup ENTERPRISE_CONTRACT_CONFIG_MAP environment variable")
			os.Exit(1)
		}
	}
}

// setHealthAndReadyChecks sets health and ready checks for the given manager.
func setHealthAndReadyChecks(mgr ctrl.Manager) {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
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
