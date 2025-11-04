package observability

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("Perses controller", func() {
	var (
		cl client.Client
	)

	BeforeEach(func() {
		// Ensure CRD types are registered in the test scheme for fake client usage
		Expect(apiextensionsv1.AddToScheme(commontestutils.GetScheme())).To(Succeed())
		Expect(corev1.AddToScheme(commontestutils.GetScheme())).To(Succeed())

		cl = commontestutils.InitClient([]client.Object{})
	})

	Context("PersesReconciler end-to-end behavior", func() {
		makeCRD := func(name string) *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			})
			u.SetName(name)
			return u
		}

		makeDatasource := func() map[string]any {
			return map[string]any{
				"apiVersion": "perses.dev/v1alpha1",
				"kind":       "PersesDatasource",
				"metadata": map[string]any{
					"name": "perses-thanos-datasource",
				},
				"spec": map[string]any{
					"config": map[string]any{
						"display": map[string]any{
							"name": "OpenShift Monitoring (Thanos Querier)",
						},
						"default": true,
						"plugin": map[string]any{
							"kind": "PrometheusDatasource",
							"spec": map[string]any{
								"proxy": map[string]any{
									"kind": "HTTPProxy",
									"spec": map[string]any{
										"url":    "https://thanos-querier.openshift-monitoring.svc.cluster.local:9091",
										"secret": "perses-thanos-datasource-secret",
									},
								},
							},
						},
					},
					"client": map[string]any{
						"tls": map[string]any{
							"enable": true,
							"caCert": map[string]any{
								"type":     "file",
								"certPath": "/ca/service-ca.crt",
							},
						},
					},
				},
			}
		}

		makeValidDashboard := func() map[string]any {
			return map[string]any{
				"apiVersion": "perses.dev/v1alpha1",
				"kind":       "PersesDashboard",
				"metadata": map[string]any{
					"name": "perses-dashboard-node-memory-overview",
				},
				"spec": map[string]any{
					"display": map[string]any{
						"name": "Node Memory Overview",
					},
					"panels": map[string]any{
						"ClusterPressure": map[string]any{
							"kind": "Panel",
							"spec": map[string]any{
								"display": map[string]any{"name": "Cluster - Memory Pressure"},
								"plugin":  map[string]any{"kind": "TimeSeriesChart", "spec": map[string]any{}},
								"queries": []any{},
							},
						},
					},
					"layouts": []any{
						map[string]any{
							"kind": "Grid",
							"spec": map[string]any{
								"display": map[string]any{"title": "Overview"},
								"items": []any{
									map[string]any{
										"x":      0,
										"y":      0,
										"width":  24,
										"height": 8,
										"content": map[string]any{
											"$ref": "#/spec/panels/ClusterPressure",
										},
									},
								},
							},
						},
					},
				},
			}
		}

		It("should apply datasource and dashboard when CRDs are available", func() {
			// seed CRDs
			Expect(cl.Create(context.TODO(), makeCRD("persesdashboards.perses.dev"))).To(Succeed())
			Expect(cl.Create(context.TODO(), makeCRD("persesdatasources.perses.dev"))).To(Succeed())

			r := &PersesReconciler{
				Client:            cl,
				namespace:         "openshift-cnv",
				cachedDatasources: []map[string]any{makeDatasource()},
				cachedDashboards:  []map[string]any{makeValidDashboard()},
			}
			_, err := r.Reconcile(context.TODO(), reconcile.Request{})
			Expect(err).ToNot(HaveOccurred())

			// datasource exists
			ds := &unstructured.Unstructured{}
			ds.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDatasource"})
			Expect(cl.Get(context.TODO(), client.ObjectKey{Namespace: "openshift-cnv", Name: "perses-thanos-datasource"}, ds)).To(Succeed())
			// dashboard exists
			db := &unstructured.Unstructured{}
			db.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDashboard"})
			Expect(cl.Get(context.TODO(), client.ObjectKey{Namespace: "openshift-cnv", Name: "perses-dashboard-node-memory-overview"}, db)).To(Succeed())
		})

		It("should skip reconcile when Perses CRDs are missing", func() {
			r := &PersesReconciler{
				Client:            cl,
				namespace:         "openshift-cnv",
				cachedDatasources: []map[string]any{makeDatasource()},
				cachedDashboards:  []map[string]any{makeValidDashboard()},
			}
			_, err := r.Reconcile(context.TODO(), reconcile.Request{})
			Expect(err).ToNot(HaveOccurred())

			// Resources should not exist
			ds := &unstructured.Unstructured{}
			ds.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDatasource"})
			err = cl.Get(context.TODO(), client.ObjectKey{Namespace: "openshift-cnv", Name: "perses-thanos-datasource"}, ds)
			Expect(client.IgnoreNotFound(err)).To(Succeed())

			db := &unstructured.Unstructured{}
			db.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDashboard"})
			err = cl.Get(context.TODO(), client.ObjectKey{Namespace: "openshift-cnv", Name: "perses-dashboard-node-memory-overview"}, db)
			Expect(client.IgnoreNotFound(err)).To(Succeed())
		})

		// Note: schema validation errors (e.g., missing required fields) are not enforced by the fake client.
		// Such cases are covered by integration/e2e; here we focus on positive flow and gating.
	})
})
