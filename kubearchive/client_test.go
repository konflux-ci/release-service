package kubearchive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	applicationapiv1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// stubHTTPClient overrides the package-level function variables so that Get
// uses the supplied httptest.Server instead of building a real in-cluster
// client. It returns a cleanup function that restores the originals.
func stubHTTPClient(server *httptest.Server) func() {
	origDiscover := discoverAPIURLFunc
	origNewHTTP := newHTTPClientFunc

	discoverAPIURLFunc = func(_ context.Context, _ client.Client) (string, error) {
		return server.URL, nil
	}
	newHTTPClientFunc = func(_ context.Context, _ client.Client) (*http.Client, error) {
		return server.Client(), nil
	}

	return func() {
		discoverAPIURLFunc = origDiscover
		newHTTPClientFunc = origNewHTTP
	}
}

var _ = Describe("KubeArchive Client", func() {

	Describe("buildAPIPath", func() {
		It("builds a path for a core resource", func() {
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
			Expect(buildAPIPath(gvr, "my-ns", "my-cm")).To(Equal("/api/v1/namespaces/my-ns/configmaps/my-cm"))
		})

		It("builds a path for a non-core resource", func() {
			gvr := schema.GroupVersionResource{Group: "appstudio.redhat.com", Version: "v1alpha1", Resource: "snapshots"}
			Expect(buildAPIPath(gvr, "tenant", "snap-1")).To(
				Equal("/apis/appstudio.redhat.com/v1alpha1/namespaces/tenant/snapshots/snap-1"))
		})
	})

	Describe("buildListAPIPath", func() {
		It("builds a list path for a core resource", func() {
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
			Expect(buildListAPIPath(gvr, "my-ns")).To(Equal("/api/v1/namespaces/my-ns/configmaps"))
		})

		It("builds a list path for a non-core resource", func() {
			gvr := schema.GroupVersionResource{Group: "appstudio.redhat.com", Version: "v1alpha1", Resource: "snapshots"}
			Expect(buildListAPIPath(gvr, "tenant")).To(
				Equal("/apis/appstudio.redhat.com/v1alpha1/namespaces/tenant/snapshots"))
		})
	})

	Describe("truncate", func() {
		It("returns the string unchanged when shorter than maxLen", func() {
			Expect(truncate("short", 10)).To(Equal("short"))
		})

		It("truncates and appends ellipsis when longer than maxLen", func() {
			Expect(truncate("a]long string here", 6)).To(Equal("a]long..."))
		})

		It("returns the string unchanged when exactly maxLen", func() {
			Expect(truncate("exact", 5)).To(Equal("exact"))
		})
	})

	Describe("discoverAPIURL", func() {
		var (
			ctx    context.Context
			scheme *runtime.Scheme
		)

		BeforeEach(func() {
			ctx = context.Background()
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
		})

		It("returns the internal service URL when the ConfigMap exists", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      apiURLConfigMapName,
					Namespace: kaNamespace,
				},
				Data: map[string]string{
					"URL": "https://kubearchive.example.com/",
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

			url, err := discoverAPIURL(ctx, cli)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal(fmt.Sprintf("https://%s.%s.svc.cluster.local:%d",
				kaServiceName, kaNamespace, kaServicePort)))
		})

		It("returns an error when no ConfigMap exists", func() {
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()

			_, err := discoverAPIURL(ctx, cli)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found in namespace"))
		})
	})

	Describe("doGet", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		It("returns the body and status code on success", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "hello")
			}))
			defer server.Close()

			body, status, err := doGet(ctx, server.Client(), server.URL+"/test")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(http.StatusOK))
			Expect(string(body)).To(Equal("hello"))
		})

		It("returns an error when the server is unreachable", func() {
			_, _, err := doGet(ctx, http.DefaultClient, "http://127.0.0.1:1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kubearchive request"))
		})
	})

	Describe("getFromList", func() {
		var (
			ctx context.Context
			gvr schema.GroupVersionResource
		)

		BeforeEach(func() {
			ctx = context.Background()
			gvr = schema.GroupVersionResource{
				Group: "appstudio.redhat.com", Version: "v1alpha1", Resource: "snapshots",
			}
		})

		It("selects the item with the highest resourceVersion", func() {
			list := map[string]interface{}{
				"items": []map[string]interface{}{
					{"metadata": map[string]interface{}{"name": "snap", "namespace": "ns", "resourceVersion": "50"}},
					{"metadata": map[string]interface{}{"name": "snap", "namespace": "ns", "resourceVersion": "300"}},
					{"metadata": map[string]interface{}{"name": "snap", "namespace": "ns", "resourceVersion": "150"}},
				},
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(list)
			}))
			defer server.Close()

			obj := &applicationapiv1alpha1.Snapshot{}
			err := getFromList(ctx, server.Client(), server.URL, gvr, "ns", "snap", obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.ResourceVersion).To(Equal("300"))
		})

		It("filters items by name", func() {
			list := map[string]interface{}{
				"items": []map[string]interface{}{
					{"metadata": map[string]interface{}{"name": "other", "namespace": "ns", "resourceVersion": "999"}},
					{"metadata": map[string]interface{}{"name": "target", "namespace": "ns", "resourceVersion": "100"}},
				},
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(list)
			}))
			defer server.Close()

			obj := &applicationapiv1alpha1.Snapshot{}
			err := getFromList(ctx, server.Client(), server.URL, gvr, "ns", "target", obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Name).To(Equal("target"))
			Expect(obj.ResourceVersion).To(Equal("100"))
		})

		It("returns NotFound when the list is empty", func() {
			list := map[string]interface{}{"items": []interface{}{}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(list)
			}))
			defer server.Close()

			obj := &applicationapiv1alpha1.Snapshot{}
			err := getFromList(ctx, server.Client(), server.URL, gvr, "ns", "snap", obj)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("returns NotFound when no items match the name", func() {
			list := map[string]interface{}{
				"items": []map[string]interface{}{
					{"metadata": map[string]interface{}{"name": "other", "namespace": "ns", "resourceVersion": "1"}},
				},
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(list)
			}))
			defer server.Close()

			obj := &applicationapiv1alpha1.Snapshot{}
			err := getFromList(ctx, server.Client(), server.URL, gvr, "ns", "snap", obj)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("returns an error when the list endpoint returns a non-200 status", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("unavailable"))
			}))
			defer server.Close()

			obj := &applicationapiv1alpha1.Snapshot{}
			err := getFromList(ctx, server.Client(), server.URL, gvr, "ns", "snap", obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("HTTP 503"))
		})

		It("passes the fieldSelector query parameter", func() {
			var receivedQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedQuery = r.URL.Query().Get("fieldSelector")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
			}))
			defer server.Close()

			obj := &applicationapiv1alpha1.Snapshot{}
			_ = getFromList(ctx, server.Client(), server.URL, gvr, "ns", "my-snap", obj)
			Expect(receivedQuery).To(Equal("metadata.name=my-snap"))
		})
	})
})

var _ = Describe("loadServiceCA", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
	})

	It("returns the CA data from the ConfigMap", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceCAConfigMapName,
				Namespace: kaNamespace,
			},
			Data: map[string]string{
				serviceCAConfigMapKey: "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----\n",
			},
		}
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

		data, err := loadServiceCA(ctx, cli)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("BEGIN CERTIFICATE"))
	})

	It("returns an error when the ConfigMap does not exist", func() {
		cli := fake.NewClientBuilder().WithScheme(scheme).Build()

		_, err := loadServiceCA(ctx, cli)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found in namespace"))
	})

	It("returns an error when the CA key is missing", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceCAConfigMapName,
				Namespace: kaNamespace,
			},
			Data: map[string]string{},
		}
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

		_, err := loadServiceCA(ctx, cli)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no"))
	})
})

var _ = Describe("Get", func() {
	var (
		ctx context.Context
		gvr schema.GroupVersionResource
	)

	BeforeEach(func() {
		ctx = context.Background()
		gvr = schema.GroupVersionResource{
			Group: "appstudio.redhat.com", Version: "v1alpha1", Resource: "snapshots",
		}
	})

	It("unmarshals a 200 response into the target object", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			snap := applicationapiv1alpha1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{Name: "my-snap", Namespace: "ns"},
				Spec:       applicationapiv1alpha1.SnapshotSpec{Application: "my-app"},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(snap)
		}))
		defer server.Close()
		cleanup := stubHTTPClient(server)
		defer cleanup()

		obj := &applicationapiv1alpha1.Snapshot{}
		err := Get(ctx, nil, gvr, "ns", "my-snap", obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.Name).To(Equal("my-snap"))
		Expect(obj.Spec.Application).To(Equal("my-app"))
	})

	It("returns a NotFound error on 404", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}))
		defer server.Close()
		cleanup := stubHTTPClient(server)
		defer cleanup()

		obj := &applicationapiv1alpha1.Snapshot{}
		err := Get(ctx, nil, gvr, "ns", "missing", obj)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("falls back to the list endpoint on HTTP 500 with 'more than one resource found'", func() {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("more than one resource found"))
				return
			}
			list := map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"apiVersion": "appstudio.redhat.com/v1alpha1",
						"kind":       "Snapshot",
						"metadata":   map[string]interface{}{"name": "target", "namespace": "ns", "resourceVersion": "100"},
						"spec":       map[string]interface{}{"application": "old-app"},
					},
					{
						"apiVersion": "appstudio.redhat.com/v1alpha1",
						"kind":       "Snapshot",
						"metadata":   map[string]interface{}{"name": "target", "namespace": "ns", "resourceVersion": "200"},
						"spec":       map[string]interface{}{"application": "latest-app"},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(list)
		}))
		defer server.Close()
		cleanup := stubHTTPClient(server)
		defer cleanup()

		obj := &applicationapiv1alpha1.Snapshot{}
		err := Get(ctx, nil, gvr, "ns", "target", obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.Name).To(Equal("target"))
		Expect(obj.ResourceVersion).To(Equal("200"))
		Expect(obj.Spec.Application).To(Equal("latest-app"))
		Expect(callCount).To(Equal(2))
	})

	It("returns an error on unexpected HTTP status codes", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}))
		defer server.Close()
		cleanup := stubHTTPClient(server)
		defer cleanup()

		obj := &applicationapiv1alpha1.Snapshot{}
		err := Get(ctx, nil, gvr, "ns", "snap", obj)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("HTTP 403"))
	})

	It("returns an error on HTTP 500 without the duplicate-resource message", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}))
		defer server.Close()
		cleanup := stubHTTPClient(server)
		defer cleanup()

		obj := &applicationapiv1alpha1.Snapshot{}
		err := Get(ctx, nil, gvr, "ns", "snap", obj)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("HTTP 500"))
	})

	It("returns an error when API URL discovery fails", func() {
		origDiscover := discoverAPIURLFunc
		defer func() { discoverAPIURLFunc = origDiscover }()

		discoverAPIURLFunc = func(_ context.Context, _ client.Client) (string, error) {
			return "", fmt.Errorf("discovery boom")
		}

		obj := &applicationapiv1alpha1.Snapshot{}
		err := Get(ctx, nil, gvr, "ns", "snap", obj)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("discovery boom"))
	})

	It("returns an error when HTTP client creation fails", func() {
		origDiscover := discoverAPIURLFunc
		origNewHTTP := newHTTPClientFunc
		defer func() {
			discoverAPIURLFunc = origDiscover
			newHTTPClientFunc = origNewHTTP
		}()

		discoverAPIURLFunc = func(_ context.Context, _ client.Client) (string, error) {
			return "https://fake", nil
		}
		newHTTPClientFunc = func(_ context.Context, _ client.Client) (*http.Client, error) {
			return nil, fmt.Errorf("client boom")
		}

		obj := &applicationapiv1alpha1.Snapshot{}
		err := Get(ctx, nil, gvr, "ns", "snap", obj)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("client boom"))
	})
})
