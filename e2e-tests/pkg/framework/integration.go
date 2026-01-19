package framework

import (
	"context"
	"fmt"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	"github.com/konflux-ci/release-service/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IntegrationController handles Snapshot operations (integration-service domain).
type IntegrationController struct {
	*KubeClient
}

// NewIntegrationController creates a new IntegrationController.
func NewIntegrationController(kubeClient *KubeClient) *IntegrationController {
	return &IntegrationController{
		KubeClient: kubeClient,
	}
}

// =============================================================================
// Snapshot Operations
// =============================================================================

// CreateSnapshotWithImageSource creates a Snapshot with image and git source information.
// Pass empty strings for componentName2/containerImage2 to create a single-component snapshot.
func (i *IntegrationController) CreateSnapshotWithImageSource(
	componentName, applicationName, namespace,
	containerImage, gitSourceURL, gitSourceRevision,
	componentName2, containerImage2, gitSourceURL2, gitSourceRevision2 string,
) (*appstudioApi.Snapshot, error) {
	snapshotComponents := []appstudioApi.SnapshotComponent{
		{
			Name:           componentName,
			ContainerImage: containerImage,
			Source: appstudioApi.ComponentSource{
				ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision,
						URL:      gitSourceURL,
					},
				},
			},
		},
	}

	// Add second component if provided
	if componentName2 != "" && containerImage2 != "" {
		snapshotComponents = append(snapshotComponents, appstudioApi.SnapshotComponent{
			Name:           componentName2,
			ContainerImage: containerImage2,
			Source: appstudioApi.ComponentSource{
				ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision2,
						URL:      gitSourceURL2,
					},
				},
			},
		})
	}

	snapshotName := "snapshot-sample-" + utils.GenerateRandomString(4)

	snapshot := &appstudioApi.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: namespace,
			Labels: map[string]string{
				"test.appstudio.openshift.io/type":                             "component",
				fmt.Sprintf("%s/component", metadata.RhtapDomain):              componentName,
				fmt.Sprintf("%s/event-type", metadata.PipelinesAsCodePrefix):   "push",
				fmt.Sprintf("%s/sender", metadata.PipelinesAsCodePrefix):       "release-service-e2e",
				metadata.AuthorLabel:                                           "release-service-e2e",
			},
			Annotations: map[string]string{
				fmt.Sprintf("%s/url-repository", metadata.PipelinesAsCodePrefix): gitSourceURL,
				fmt.Sprintf("%s/sha", metadata.PipelinesAsCodePrefix):            gitSourceRevision,
			},
		},
		Spec: appstudioApi.SnapshotSpec{
			Application: applicationName,
			Components:  snapshotComponents,
		},
	}

	err := i.kubeClient.Create(context.Background(), snapshot)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}
