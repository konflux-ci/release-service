package framework

import (
	"context"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KonfluxApiController handles Konflux API resources: Application and Component.
type KonfluxApiController struct {
	*KubeClient
}

// NewKonfluxApiController creates a new KonfluxApiController.
func NewKonfluxApiController(kubeClient *KubeClient) *KonfluxApiController {
	return &KonfluxApiController{
		KubeClient: kubeClient,
	}
}

// =============================================================================
// Application Operations
// =============================================================================

// CreateApplication creates an Application CR.
func (k *KonfluxApiController) CreateApplication(name, namespace string) (*appstudioApi.Application, error) {
	app := &appstudioApi.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appstudioApi.ApplicationSpec{
			DisplayName: name,
		},
	}

	err := k.kubeClient.Create(context.Background(), app)
	if err != nil {
		return nil, err
	}
	return app, nil
}
