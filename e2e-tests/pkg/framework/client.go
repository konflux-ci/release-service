package framework

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ecpApi "github.com/conforma/crds/api/v1alpha1"
	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// KubeClient wraps a controller-runtime client with helper methods.
type KubeClient struct {
	kubeClient client.Client
	kubeRest   *rest.Config
}

// KubeRest returns the Kubernetes client.
func (c *KubeClient) KubeRest() client.Client {
	return c.kubeClient
}

// newKubeClient creates a new KubeClient.
func newKubeClient() (*KubeClient, error) {
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = releaseApi.AddToScheme(scheme)
	_ = appstudioApi.AddToScheme(scheme)
	_ = tektonv1.AddToScheme(scheme)
	_ = ecpApi.AddToScheme(scheme)

	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	return &KubeClient{
		kubeClient: k8sClient,
		kubeRest:   restConfig,
	}, nil
}

// CreateTestNamespace creates a namespace for testing.
func (c *KubeClient) CreateTestNamespace(name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := c.kubeClient.Create(context.Background(), ns)
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return nil, err
	}
	return ns, nil
}

// DeleteNamespace deletes a namespace.
func (c *KubeClient) DeleteNamespace(name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return c.kubeClient.Delete(context.Background(), ns)
}

// GetSecret gets a secret.
func (c *KubeClient) GetSecret(namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := c.kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, secret)
	return secret, err
}

// CreateRegistryAuthSecret creates a docker registry auth secret.
func (c *KubeClient) CreateRegistryAuthSecret(name, namespace, authJson string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(authJson),
		},
	}
	err := c.kubeClient.Create(context.Background(), secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// CreateServiceAccount creates a service account.
func (c *KubeClient) CreateServiceAccount(name, namespace string, secrets []corev1.ObjectReference, imagePullSecrets []corev1.LocalObjectReference) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Secrets:          secrets,
		ImagePullSecrets: imagePullSecrets,
	}
	err := c.kubeClient.Create(context.Background(), sa)
	if err != nil {
		return nil, err
	}
	return sa, nil
}

// LinkSecretToServiceAccount links a secret to a service account.
func (c *KubeClient) LinkSecretToServiceAccount(namespace, secretName, serviceAccountName string, addImagePullSecret bool) error {
	sa := &corev1.ServiceAccount{}
	err := c.kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: serviceAccountName}, sa)
	if err != nil {
		return err
	}

	sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: secretName})
	if addImagePullSecret {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
	}

	return c.kubeClient.Update(context.Background(), sa)
}

// CreateRole creates a role.
func (c *KubeClient) CreateRole(name, namespace string, rules map[string][]string) (*rbacv1.Role, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: rules["apiGroupsList"],
				Resources: rules["roleResources"],
				Verbs:     rules["roleVerbs"],
			},
		},
	}
	err := c.kubeClient.Create(context.Background(), role)
	if err != nil {
		return nil, err
	}
	return role, nil
}

// CreateRoleBinding creates a role binding.
func (c *KubeClient) CreateRoleBinding(name, namespace, subjectKind, subjectName, subjectNamespace, roleKind, roleName, roleAPIGroup string) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      subjectKind,
				Name:      subjectName,
				Namespace: subjectNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     roleKind,
			Name:     roleName,
			APIGroup: roleAPIGroup,
		},
	}
	err := c.kubeClient.Create(context.Background(), rb)
	if err != nil {
		return nil, err
	}
	return rb, nil
}
