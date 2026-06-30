package kubearchive

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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

	kaNamespace   = "product-kubearchive"
	kaServiceName = "kubearchive-api-server"
	kaServicePort = 8081

	serviceCAConfigMapName = "openshift-service-ca.crt"
	serviceCAConfigMapKey  = "service-ca.crt"

	httpClientTimeout = 10 * time.Second
)

var (
	discoverAPIURLFunc = discoverAPIURL
	newHTTPClientFunc  = newHTTPClient
)

// Get retrieves an archived resource from KubeArchive by its GroupVersionResource, namespace,
// and name. The Kubernetes client is used to check whether KubeArchive is deployed and to load
// the service-serving CA for TLS. The result is unmarshaled into obj.
//
// When multiple archived versions of the same resource exist, the KubeArchive single-resource
// endpoint returns HTTP 500. In that case, Get falls back to the list endpoint and selects the
// item with the highest resourceVersion (the most recently persisted version).
func Get(ctx context.Context, cli client.Client, gvr schema.GroupVersionResource,
	namespace, name string, obj client.Object) error {

	logger := ctrl.LoggerFrom(ctx)

	apiURL, err := discoverAPIURLFunc(ctx, cli)
	if err != nil {
		logger.Error(err, "Failed to discover KubeArchive API")
		return err
	}

	httpClient, err := newHTTPClientFunc(ctx, cli)
	if err != nil {
		logger.Error(err, "Failed to create KubeArchive HTTP client")
		return err
	}

	body, statusCode, err := doGet(ctx, httpClient, apiURL+buildAPIPath(gvr, namespace, name))
	if err != nil {
		logger.Error(err, "KubeArchive request failed",
			"gvr", gvr.String(), "namespace", namespace, "name", name)
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
		return getFromList(ctx, httpClient, apiURL, gvr, namespace, name, obj)
	}

	retErr := fmt.Errorf("kubearchive returned HTTP %d for %s/%s: %s",
		statusCode, namespace, name, truncate(string(body), 200))
	logger.Error(retErr, "Unexpected KubeArchive response",
		"gvr", gvr.String(), "statusCode", statusCode)
	return retErr
}

// doGet performs an HTTP GET and returns the body, status code, and any transport error.
func doGet(ctx context.Context, httpClient *http.Client, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("building kubearchive request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("kubearchive request to %s failed: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("reading kubearchive response: %w", err)
	}
	return body, resp.StatusCode, nil
}

// getFromList queries the list endpoint with a fieldSelector to retrieve only archived
// versions matching the given name, then picks the one with the highest resourceVersion.
func getFromList(ctx context.Context, httpClient *http.Client,
	apiURL string, gvr schema.GroupVersionResource,
	namespace, name string, obj client.Object) error {

	listURL := apiURL + buildListAPIPath(gvr, namespace) +
		"?fieldSelector=" + url.QueryEscape("metadata.name="+name)
	body, statusCode, err := doGet(ctx, httpClient, listURL)
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

// discoverAPIURL checks whether KubeArchive is deployed on this cluster by
// looking for the well-known ConfigMap "kubearchive-api-url" in the
// product-kubearchive namespace. If present, it returns the in-cluster service
// URL so that the controller communicates directly with the KubeArchive API
// server over the cluster network, avoiding external route egress restrictions.
func discoverAPIURL(ctx context.Context, cli client.Client) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: kaNamespace, Name: apiURLConfigMapName}, cm); err != nil {
		return "", fmt.Errorf("ConfigMap %q not found in namespace %q: %w", apiURLConfigMapName, kaNamespace, err)
	}
	return fmt.Sprintf("https://%s.%s.svc.cluster.local:%d", kaServiceName, kaNamespace, kaServicePort), nil
}

// loadServiceCA reads the OpenShift service-serving CA bundle from the
// well-known "openshift-service-ca.crt" ConfigMap in the product-kubearchive
// namespace. The ConfigMap is auto-populated by the service-ca operator in
// every namespace.
func loadServiceCA(ctx context.Context, cli client.Client) ([]byte, error) {
	cm := &corev1.ConfigMap{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: kaNamespace, Name: serviceCAConfigMapName}, cm); err != nil {
		return nil, fmt.Errorf("ConfigMap %q not found in namespace %q: %w", serviceCAConfigMapName, kaNamespace, err)
	}
	pem, ok := cm.Data[serviceCAConfigMapKey]
	if !ok || len(pem) == 0 {
		return nil, fmt.Errorf("ConfigMap %q in namespace %q has no %q key", serviceCAConfigMapName, kaNamespace, serviceCAConfigMapKey)
	}
	return []byte(pem), nil
}

// newHTTPClient builds an HTTP client that carries the in-cluster service
// account bearer token for KubeArchive authentication and trusts the
// service-serving CA for TLS verification of the internal service endpoint.
func newHTTPClient(ctx context.Context, cli client.Client) (*http.Client, error) {
	caData, err := loadServiceCA(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("loading service-serving CA: %w", err)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("loading in-cluster config: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("failed to parse service-serving CA bundle")
	}

	cfg.TLSClientConfig = rest.TLSClientConfig{CAData: caData}
	cfg.Timeout = httpClientTimeout

	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("building transport: %w", err)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   httpClientTimeout,
	}, nil
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
