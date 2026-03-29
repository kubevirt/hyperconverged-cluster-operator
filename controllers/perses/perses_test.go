package perses

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing/fstest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	persesv1alpha1 "github.com/rhobs/perses-operator/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// countingClient wraps a client.Client and counts selected calls
type countingClient struct {
	client.Client
	createCount int
	updateCount int
	listCount   int
}

func (c *countingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	c.createCount++
	return c.Client.Create(ctx, obj, opts...)
}
func (c *countingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateCount++
	return c.Client.Update(ctx, obj, opts...)
}
func (c *countingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.listCount++
	return c.Client.List(ctx, list, opts...)
}

// failingUpdateClient wraps countingClient and forces Update to return an error
type failingUpdateClient struct{ *countingClient }

func (f *failingUpdateClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	f.updateCount++
	return errors.New("boom")
}

var _ = Describe("Perses controller", func() {
	var (
		dashboards map[string]persesv1alpha1.PersesDashboard
		datasource *persesv1alpha1.PersesDatasource
		s          *runtime.Scheme
		startupReq reconcile.Request
	)

	BeforeEach(func() {
		s = scheme.Scheme
		Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())
		Expect(persesv1alpha1.AddToScheme(s)).To(Succeed())

		var err error
		dashboards, err = initDashboards(commontestutils.Namespace, GinkgoLogr)
		Expect(err).ToNot(HaveOccurred())
		Expect(dashboards).ToNot(BeEmpty())

		datasource, err = initDatasource(commontestutils.Namespace)
		Expect(err).ToNot(HaveOccurred())

		startupReq = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: startupReqType + randomSufix,
			},
		}
	})

	Context("PersesReconciler end-to-end behavior", func() {
		makeCRD := func(name string) *apiextensionsv1.CustomResourceDefinition {
			return &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}
		}

		It("should apply datasource and dashboard when CRDs are available with labels and owner", func(ctx context.Context) {
			ctx = logr.NewContext(ctx, GinkgoLogr)
			tracker := testing.NewObjectTracker(s, serializer.NewCodecFactory(s).UniversalDecoder())
			Expect(tracker.Add(makeCRD("persesdashboards.perses.dev"))).To(Succeed())
			Expect(tracker.Add(makeCRD("persesdatasources.perses.dev"))).To(Succeed())

			base := fake.NewClientBuilder().
				WithScheme(s).
				WithObjectTracker(tracker).
				Build()
			cl := &countingClient{Client: base}

			r := &PersesReconciler{
				Client:           cl,
				namespace:        commontestutils.Namespace,
				cachedDatasource: datasource,
				cachedDashboards: dashboards,
				owner:            metav1.OwnerReference{UID: types.UID("test-uid")},
			}
			_, err := r.Reconcile(ctx, startupReq)
			Expect(err).ToNot(HaveOccurred())

			ds := &persesv1alpha1.PersesDatasource{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: commontestutils.Namespace, Name: datasource.Name}, ds)).To(Succeed())
			Expect(ds.GetLabels()).To(SatisfyAll(
				HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName),
				HaveKeyWithValue(hcoutil.AppLabelPartOf, hcoutil.HyperConvergedCluster),
			))
			Expect(ds.OwnerReferences).ToNot(BeEmpty())

			for name := range dashboards {
				db := &persesv1alpha1.PersesDashboard{}
				Expect(cl.Get(ctx, client.ObjectKey{Namespace: commontestutils.Namespace, Name: name}, db)).To(Succeed())
				Expect(db.GetLabels()).To(SatisfyAll(
					HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName),
					HaveKeyWithValue(hcoutil.AppLabelPartOf, hcoutil.HyperConvergedCluster),
				))
				Expect(db.OwnerReferences).ToNot(BeEmpty())
			}
			Expect(cl.createCount).To(BeNumerically(">=", 1))
		})

		It("should skip reconcile when Perses CRDs are missing without writes", func(ctx context.Context) {
			ctx = logr.NewContext(ctx, GinkgoLogr)
			tracker := testing.NewObjectTracker(s, serializer.NewCodecFactory(s).UniversalDecoder())
			base := fake.NewClientBuilder().
				WithScheme(s).
				WithObjectTracker(tracker).
				Build()
			cl := &countingClient{Client: base}

			r := &PersesReconciler{
				Client:           cl,
				namespace:        commontestutils.Namespace,
				cachedDatasource: datasource,
				cachedDashboards: dashboards,
			}
			_, err := r.Reconcile(ctx, startupReq)
			Expect(err).ToNot(HaveOccurred())

			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "any" + dashboardReqSufix + randomSufix}})
			Expect(err).ToNot(HaveOccurred())
			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "any" + datasourceReqSufix + randomSufix}})
			Expect(err).ToNot(HaveOccurred())

			Expect(cl.createCount).To(Equal(0))
			Expect(cl.updateCount).To(Equal(0))
		})
	})

	Context("SetupPersesWithManager guard", func() {
		It("should skip controller registration when Perses CRDs are not available", func(ctx context.Context) {
			ctx = logr.NewContext(ctx, GinkgoLogr)
			old := checkPersesAvailable
			checkPersesAvailable = func(_ context.Context, _ client.Client) bool { return false }
			defer func() { checkPersesAvailable = old }()

			// Build a no-CRD fake client and a lightweight manager mock
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			mgr, err := commontestutils.NewManagerMock(nil, manager.Options{Scheme: scheme.Scheme}, cl, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())

			err = SetupPersesWithManager(ctx, mgr, metav1.OwnerReference{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Update paths", func() {
		It("should fix missing ownerRef without spec change (dashboard)", func(ctx context.Context) {
			ctx = logr.NewContext(ctx, GinkgoLogr)
			tracker := testing.NewObjectTracker(s, serializer.NewCodecFactory(s).UniversalDecoder())
			Expect(tracker.Add(&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "persesdashboards.perses.dev"}})).To(Succeed())
			Expect(tracker.Add(&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "persesdatasources.perses.dev"}})).To(Succeed())
			base := fake.NewClientBuilder().WithScheme(s).WithObjectTracker(tracker).Build()
			cl := &countingClient{Client: base}

			r := &PersesReconciler{Client: cl, namespace: commontestutils.Namespace, cachedDashboards: dashboards, cachedDatasource: datasource, owner: metav1.OwnerReference{UID: types.UID("uid-2")}}
			name := slices.Collect(maps.Keys(dashboards))[0]
			obj := dashboards[name]
			obj.Namespace = commontestutils.Namespace
			obj.OwnerReferences = nil
			Expect(cl.Create(ctx, &obj)).To(Succeed())

			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: commontestutils.Namespace, Name: name + dashboardReqSufix + randomSufix}}
			_, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(cl.updateCount).To(Equal(1))
		})

		It("should fix missing labels without spec change (datasource)", func(ctx context.Context) {
			ctx = logr.NewContext(ctx, GinkgoLogr)
			tracker := testing.NewObjectTracker(s, serializer.NewCodecFactory(s).UniversalDecoder())
			Expect(tracker.Add(&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "persesdashboards.perses.dev"}})).To(Succeed())
			Expect(tracker.Add(&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "persesdatasources.perses.dev"}})).To(Succeed())
			base := fake.NewClientBuilder().WithScheme(s).WithObjectTracker(tracker).Build()
			cl := &countingClient{Client: base}

			r := &PersesReconciler{Client: cl, namespace: commontestutils.Namespace, cachedDashboards: dashboards, cachedDatasource: datasource, owner: metav1.OwnerReference{UID: types.UID("uid-3")}}
			obj := datasource.DeepCopy()
			obj.Namespace = commontestutils.Namespace
			obj.Labels = map[string]string{"foo": "bar"}
			Expect(cl.Create(ctx, obj)).To(Succeed())

			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: commontestutils.Namespace, Name: obj.Name + datasourceReqSufix + randomSufix}}
			_, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			// labels should be merged; extra preserved; update performed
			Expect(cl.updateCount).To(Equal(1))
		})
	})

	Context("Parsers and helpers", func() {
		It("parseDashboards: valid and invalid yaml", func() {
			good := fstest.MapFS{
				"good.yaml": {Data: []byte("apiVersion: perses.dev/v1alpha1\nkind: PersesDashboard\nmetadata:\n  name: test\nspec: {}\n")},
			}
			m, err := parseDashboards(good, commontestutils.Namespace, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(HaveKey("test"))
			Expect(m["test"].Namespace).To(Equal(commontestutils.Namespace))
			// now the invalid
			_, err = parseDashboards(fstest.MapFS{"bad.yaml": {Data: []byte("not: [yaml")}}, commontestutils.Namespace, GinkgoLogr)
			Expect(err).To(HaveOccurred())
		})

		It("initDatasource sets namespace and labels", func() {
			ds, err := initDatasource(commontestutils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.Namespace).To(Equal(commontestutils.Namespace))
			Expect(ds.Labels).ToNot(BeEmpty())
		})

		It("sync helpers: no changes returns false", func() {
			r := &PersesReconciler{owner: metav1.OwnerReference{UID: types.UID("u1")}}
			foundD := &persesv1alpha1.PersesDashboard{
				ObjectMeta: metav1.ObjectMeta{
					Labels:          hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
					OwnerReferences: []metav1.OwnerReference{{UID: types.UID("u1")}},
				},
			}
			desiredD := &persesv1alpha1.PersesDashboard{}
			Expect(r.syncDashboard(foundD, desiredD)).To(BeFalse())

			foundS := &persesv1alpha1.PersesDatasource{
				ObjectMeta: metav1.ObjectMeta{
					Labels:          hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
					OwnerReferences: []metav1.OwnerReference{{UID: types.UID("u1")}},
				},
			}
			desiredS := &persesv1alpha1.PersesDatasource{}
			Expect(r.syncDatasource(foundS, desiredS)).To(BeFalse())
		})
	})

	Context("ReconcileAll aggregation", func() {
		It("returns error when one dashboard update fails", func(ctx context.Context) {
			ctx = logr.NewContext(ctx, GinkgoLogr)
			tracker := testing.NewObjectTracker(s, serializer.NewCodecFactory(s).UniversalDecoder())
			Expect(tracker.Add(&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "persesdashboards.perses.dev"}})).To(Succeed())
			base := fake.NewClientBuilder().WithScheme(s).WithObjectTracker(tracker).Build()
			// client that fails update to force error aggregation
			fc := &failingUpdateClient{&countingClient{Client: base}}

			r := &PersesReconciler{Client: fc, namespace: commontestutils.Namespace, cachedDashboards: dashboards, cachedDatasource: datasource, owner: metav1.OwnerReference{UID: types.UID("uid-agg")}}
			// create all dashboards first so reconcileAll will try to update labels/owners
			for name := range dashboards {
				obj := dashboards[name]
				obj.Namespace = commontestutils.Namespace
				obj.Labels = map[string]string{}
				Expect(base.Create(ctx, &obj)).To(Succeed())
			}
			err := r.reconcileAll(ctx, GinkgoLogr)
			Expect(err).To(HaveOccurred())
		})
	})
})
