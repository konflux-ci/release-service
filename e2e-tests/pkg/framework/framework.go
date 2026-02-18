package framework

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// ControllerHub aggregates all controller clients.
type ControllerHub struct {
	CommonController      *KubeClient
	ReleaseController     *ReleaseController
	KonfluxApiController  *KonfluxApiController
	IntegrationController *IntegrationController
	TektonController      *TektonController
}

// Framework is the main entry point for e2e tests.
type Framework struct {
	AsKubeAdmin   *ControllerHub
	UserNamespace string
	UserName      string
}

// NewFramework creates a new Framework instance with default timeout.
func NewFramework(userName string) (*Framework, error) {
	return NewFrameworkWithTimeout(userName, time.Second*60)
}

// NewFrameworkWithTimeout creates a new Framework instance with custom timeout.
func NewFrameworkWithTimeout(userName string, timeout time.Duration) (*Framework, error) {
	if userName == "" {
		return nil, fmt.Errorf("userName cannot be empty when initializing a new framework instance")
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %v", err)
	}

	controllerHub := &ControllerHub{
		CommonController:      kubeClient,
		ReleaseController:     NewReleaseController(kubeClient),
		KonfluxApiController:  &KonfluxApiController{KubeClient: kubeClient},
		IntegrationController: &IntegrationController{KubeClient: kubeClient},
		TektonController:      NewTektonController(kubeClient),
	}

	nsName := os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV)
	if nsName == "" {
		nsName = userName
		_, err := kubeClient.CreateTestNamespace(userName)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("failed to create test namespace %s: %+v", nsName, err)
		}
	}

	return &Framework{
		AsKubeAdmin:   controllerHub,
		UserNamespace: nsName,
		UserName:      nsName,
	}, nil
}

// ReportFailure returns an AfterEach hook function that reports test failures.
func ReportFailure(fw **Framework) func() {
	return func() {
		if CurrentSpecReport().Failed() {
			GinkgoWriter.Printf("Test failed: %s\n", CurrentSpecReport().FullText())
			if fw != nil && *fw != nil {
				GinkgoWriter.Printf("Namespace: %s\n", (*fw).UserNamespace)
			}
		}
	}
}
