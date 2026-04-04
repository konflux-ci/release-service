package kubearchive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	apiURLConfigMapName = "kubearchive-api-url"
	apiURLConfigMapKey  = "URL"
)

const kaNamespace = "product-kubearchive"

// Client provides access to the KubeArchive API for retrieving archived Kubernetes resources.
// It lazily discovers the KubeArchive API URL on first use and caches it for subsequent calls.
// Authentication and TLS are derived from the in-cluster REST configuration.
type Client struct {
	mu         sync.Mutex
	apiURL     string
	resolved   bool
	httpClient *http.Client
}

// NewClient creates a new KubeArchive client. The API URL is discovered lazily on the first
// Get call using the Kubernetes client to locate the KubeArchive service.
func NewClient() *Client {
	return &Client{}
}

// Get retrieves an archived resource from KubeArchive by its GroupVersionResource, namespace,
// and name. The Kubernetes client is used for one-time API URL discovery and is not needed for
// subsequent calls. The result is unmarshaled into obj, which should be a pointer to the
// expected resource type.
//
// When multiple archived versions of the same resource exist, the KubeArchive single-resource
// endpoint returns HTTP 500. In that case, Get falls back to the list endpoint and selects the
// item with the highest resourceVersion (the most recently persisted version).
func (c *Client) Get(ctx context.Context, cli client.Client, gvr schema.GroupVersionResource,
	namespace, name string, obj client.Object) error {

	apiURL, httpClient, err := c.resolve(ctx, cli)
	if err != nil {
		return err
	}

	logger := ctrl.LoggerFrom(ctx)
	body, statusCode, err := c.doGet(ctx, httpClient, apiURL+buildAPIPath(gvr, namespace, name))
	if err != nil {
		return err
	}

	if statusCode == http.StatusOK {
		logger.Info("Retrieved resource from KubeArchive",
			"gvr", gvr.String(), "namespace", namespace, "name", name,
			"responseBytes", len(body))
		return json.Unmarshal(body, obj)
	}

	if statusCode == http.StatusNotFound {
		logger.Info("Resource not found in KubeArchive",
			"gvr", gvr.String(), "namespace", namespace, "name", name)
		return apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	if statusCode == http.StatusInternalServerError && strings.Contains(string(body), "more than one resource found") {
		logger.Info("Multiple archived versions found, falling back to list endpoint",
			"gvr", gvr.String(), "namespace", namespace, "name", name)
		return c.getFromList(ctx, httpClient, apiURL, gvr, namespace, name, obj)
	}

	return fmt.Errorf("kubearchive returned HTTP %d for %s/%s: %s",
		statusCode, namespace, name, truncate(string(body), 200))
}

// doGet performs an HTTP GET and returns the body, status code, and any transport error.
func (c *Client) doGet(ctx context.Context, httpClient *http.Client, url string) ([]byte, int, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Fetching resource from KubeArchive", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("building kubearchive request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("kubearchive request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("reading kubearchive response: %w", err)
	}
	return body, resp.StatusCode, nil
}

// getFromList queries the list endpoint with a fieldSelector to retrieve only archived
// versions matching the given name, then picks the one with the highest resourceVersion.
func (c *Client) getFromList(ctx context.Context, httpClient *http.Client,
	apiURL string, gvr schema.GroupVersionResource,
	namespace, name string, obj client.Object) error {

	listURL := apiURL + buildListAPIPath(gvr, namespace) +
		"?fieldSelector=" + url.QueryEscape("metadata.name="+name)
	body, statusCode, err := c.doGet(ctx, httpClient, listURL)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("kubearchive list returned HTTP %d for %s: %s",
			statusCode, listURL, truncate(string(body), 200))
	}

	var raw struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("parsing kubearchive list response: %w", err)
	}

	logger := ctrl.LoggerFrom(ctx)
	var bestRaw json.RawMessage
	bestRV := int64(-1)
	for _, item := range raw.Items {
		var meta struct {
			Metadata struct {
				Name            string `json:"name"`
				ResourceVersion string `json:"resourceVersion"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(item, &meta); err != nil {
			logger.Info("Skipping unparseable item in KubeArchive list response", "error", err)
			continue
		}
		if meta.Metadata.Name != name {
			continue
		}
		rv, _ := strconv.ParseInt(meta.Metadata.ResourceVersion, 10, 64)
		if rv > bestRV {
			bestRV = rv
			bestRaw = item
		}
	}

	if bestRaw == nil {
		logger.Info("Resource not found in KubeArchive list",
			"gvr", gvr.String(), "namespace", namespace, "name", name)
		return apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	logger.Info("Selected most recent archived version from list",
		"gvr", gvr.String(), "namespace", namespace, "name", name,
		"resourceVersion", bestRV)
	return json.Unmarshal(bestRaw, obj)
}

// resolve lazily discovers the KubeArchive API URL and builds an authenticated HTTP client
// on first call, caching both for subsequent use. If discovery has already been attempted
// and failed, it returns the cached error without retrying.
func (c *Client) resolve(ctx context.Context, cli client.Client) (string, *http.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.resolved {
		if c.apiURL == "" {
			return "", nil, fmt.Errorf("kubearchive is not available on this cluster")
		}
		return c.apiURL, c.httpClient, nil
	}

	logger := ctrl.LoggerFrom(ctx)
	apiURL, err := discoverAPIURL(ctx, cli)
	c.resolved = true
	if err != nil {
		logger.Info("KubeArchive not available on this cluster", "error", err)
		return "", nil, fmt.Errorf("kubearchive discovery failed: %w", err)
	}

	c.apiURL = apiURL
	if c.httpClient == nil {
		c.httpClient, err = newHTTPClient()
		if err != nil {
			return "", nil, fmt.Errorf("creating kubearchive HTTP client: %w", err)
		}
	}
	logger.Info("Discovered KubeArchive API", "url", apiURL)
	return c.apiURL, c.httpClient, nil
}

// discoverAPIURL finds the KubeArchive API server URL by reading the well-known
// ConfigMap "kubearchive-api-url" from the product-kubearchive namespace. The
// ConfigMap is deployed by infra-deployments and contains a single key "URL"
// with the route URL.
func discoverAPIURL(ctx context.Context, cli client.Client) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: kaNamespace, Name: apiURLConfigMapName}, cm); err != nil {
		return "", fmt.Errorf("ConfigMap %q not found in namespace %q: %w", apiURLConfigMapName, kaNamespace, err)
	}
	if u, ok := cm.Data[apiURLConfigMapKey]; ok &&
		(strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) {
		return strings.TrimRight(u, "/"), nil
	}
	return "", fmt.Errorf("ConfigMap %q in namespace %q has no valid URL", apiURLConfigMapName, kaNamespace)
}

// newHTTPClient builds an HTTP client that carries the in-cluster service account
// bearer token for KubeArchive authentication. The cluster CA is cleared so that
// Go falls back to the system CA bundle, which can verify the external route TLS
// certificate (signed by a public CA, not the cluster CA).
func newHTTPClient() (*http.Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("loading in-cluster config: %w", err)
	}
	cfg.TLSClientConfig.CAFile = ""
	cfg.TLSClientConfig.CAData = nil
	return rest.HTTPClientFor(cfg)
}

// buildAPIPath constructs the REST API path for a named resource, mirroring the
// Kubernetes API URL structure that the KubeArchive API server exposes.
func buildAPIPath(gvr schema.GroupVersionResource, namespace, name string) string {
	if gvr.Group == "" {
		return fmt.Sprintf("/api/%s/namespaces/%s/%s/%s",
			gvr.Version, namespace, gvr.Resource, name)
	}
	return fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s/%s",
		gvr.Group, gvr.Version, namespace, gvr.Resource, name)
}

// buildListAPIPath constructs the REST API path for listing all resources of a type
// in a namespace, without a specific resource name.
func buildListAPIPath(gvr schema.GroupVersionResource, namespace string) string {
	if gvr.Group == "" {
		return fmt.Sprintf("/api/%s/namespaces/%s/%s",
			gvr.Version, namespace, gvr.Resource)
	}
	return fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s",
		gvr.Group, gvr.Version, namespace, gvr.Resource)
}

// truncate shortens s to maxLen characters, appending "..." if truncation occurs.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
