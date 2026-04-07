package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"

	// Import test packages
	_ "github.com/konflux-ci/release-service/e2e-tests/tests/release/service"

	"flag"

	ecpApi "github.com/conforma/crds/api/v1alpha1"
	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	klog.SetLogger(ginkgo.GinkgoLogr)

	verbosity := 1
	if v, err := strconv.ParseUint(os.Getenv("KLOG_VERBOSITY"), 10, 8); err == nil {
		verbosity = int(v)
	}

	flags := &flag.FlagSet{}
	klog.InitFlags(flags)
	if err := flags.Set("v", fmt.Sprintf("%d", verbosity)); err != nil {
		panic(err)
	}
}

var requiredEnvVars = []string{
	"QUAY_TOKEN",
	"RELEASE_CATALOG_TA_QUAY_TOKEN",
}

var _ = ginkgo.BeforeSuite(func() {
	var missingEnvs []string
	for _, env := range requiredEnvVars {
		if os.Getenv(env) == "" {
			missingEnvs = append(missingEnvs, env)
		}
	}
	gomega.Expect(missingEnvs).To(gomega.BeEmpty(), "Missing required environment variables: %v", missingEnvs)

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to load kubeconfig")

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	_, err = discoveryClient.ServerVersion()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to connect to cluster")

	scheme := runtime.NewScheme()
	_ = releaseApi.AddToScheme(scheme)
	_ = appstudioApi.AddToScheme(scheme)
	_ = tektonv1.AddToScheme(scheme)
	_ = ecpApi.AddToScheme(scheme)

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// Verify all required APIs
	requiredAPIs := []client.ObjectList{
		&releaseApi.ReleaseList{},
		&releaseApi.ReleasePlanList{},
		&releaseApi.ReleasePlanAdmissionList{},
		&appstudioApi.SnapshotList{},
		&appstudioApi.ApplicationList{},
		&tektonv1.PipelineRunList{},
		&ecpApi.EnterpriseContractPolicyList{},
	}
	for _, api := range requiredAPIs {
		gomega.Expect(k8sClient.List(context.Background(), api, client.Limit(1))).To(gomega.Succeed(),
			"API not available: %T", api)
	}
})

func TestE2E(t *testing.T) {
	klog.Info("Starting Release Service e2e tests...")
	gomega.RegisterFailHandler(ginkgo.Fail)

	config, _ := ginkgo.GinkgoConfiguration()
	if config.DryRun {
		reports := ginkgo.PreviewSpecs("Release Service E2E tests")
		for _, spec := range reports.SpecReports {
			if spec.State.Is(types.SpecStatePassed) {
				klog.Info(spec.FullText())
			}
		}
	} else {
		ginkgo.RunSpecs(t, "Release Service E2E tests")
	}
}
