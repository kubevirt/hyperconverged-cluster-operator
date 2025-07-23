package passt_test

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/passt"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Passt DaemonSet tests", func() {
	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("test NewPasstBindingCNIDaemonSet", func() {
		It("should have all default fields", func() {
			ds := passt.NewPasstBindingCNIDaemonSet(hco)

			Expect(ds.Name).To(Equal("passt-binding-cni"))
			Expect(ds.Namespace).To(Equal(hco.Namespace))

			Expect(ds.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(ds.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))
			Expect(ds.Labels).To(HaveKeyWithValue("tier", "node"))
			Expect(ds.Labels).To(HaveKeyWithValue("app", "kubevirt-hyperconverged"))

			Expect(ds.Spec.Selector.MatchLabels).To(HaveKeyWithValue("name", "passt-binding-cni"))

			Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("name", "passt-binding-cni"))
			Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("tier", "node"))
			Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("app", "passt-binding-cni"))

			Expect(ds.Spec.Template.Annotations).To(HaveKeyWithValue("description", "passt-binding-cni installs 'passt binding' CNI on cluster nodes"))

			Expect(ds.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))

			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := ds.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("installer"))
			Expect(container.SecurityContext.Privileged).ToNot(BeNil())
			Expect(*container.SecurityContext.Privileged).To(BeTrue())

			Expect(ds.Spec.Template.Spec.Volumes).To(HaveLen(1))
			volume := ds.Spec.Template.Spec.Volumes[0]
			Expect(volume.Name).To(Equal("cnibin"))
			Expect(volume.HostPath).ToNot(BeNil())
			Expect(volume.HostPath.Path).To(Equal("/opt/cni/bin"))
		})
	})

	Context("DaemonSet deployment", func() {
		It("should not create DaemonSet if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDSs := &appsv1.DaemonSetList{}
			Expect(cl.List(context.Background(), foundDSs)).To(Succeed())
			Expect(foundDSs.Items).To(BeEmpty())
		})

		It("should delete DaemonSet if the deployPasstNetworkBinding annotation is false", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "false"

			ds := passt.NewPasstBindingCNIDaemonSet(hco)
			cl = commontestutils.InitClient([]client.Object{hco, ds})

			handler := passt.NewPasstDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(ds.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundDSs := &appsv1.DaemonSetList{}
			Expect(cl.List(context.Background(), foundDSs)).To(Succeed())
			Expect(foundDSs.Items).To(BeEmpty())
		})

		It("should create DaemonSet if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("passt-binding-cni"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDS := &appsv1.DaemonSet{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundDS)).To(Succeed())

			Expect(foundDS.Name).To(Equal("passt-binding-cni"))
			Expect(foundDS.Namespace).To(Equal(hco.Namespace))

			// example of field set by the handler
			Expect(foundDS.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))
		})
	})

	Context("DaemonSet update", func() {
		It("should update DaemonSet fields if not matched to the requirements", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			ds := passt.NewPasstBindingCNIDaemonSet(hco)
			ds.Spec.Template.Spec.PriorityClassName = "wrong-priority-class"
			ds.Spec.Template.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			}

			cl = commontestutils.InitClient([]client.Object{hco, ds})

			handler := passt.NewPasstDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundDS := &appsv1.DaemonSet{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundDS)).To(Succeed())

			Expect(foundDS.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))

			expectedResources := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("15Mi"),
				},
			}
			Expect(foundDS.Spec.Template.Spec.Containers[0].Resources).To(Equal(expectedResources))
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			outdatedResource := passt.NewPasstBindingCNIDaemonSet(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)
			for k, v := range expectedLabels {
				outdatedResource.Labels[k] = "wrong_" + v
			}
			outdatedResource.Labels[userLabelKey] = userLabelValue

			cl = commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := passt.NewPasstDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.DaemonSet{}
			Expect(
				cl.Get(context.TODO(),
					client.ObjectKey{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})

		It("should reconcile managed labels to default on label deletion without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			outdatedResource := passt.NewPasstBindingCNIDaemonSet(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)
			outdatedResource.Labels[userLabelKey] = userLabelValue
			delete(outdatedResource.Labels, hcoutil.AppLabelVersion)

			cl = commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := passt.NewPasstDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.DaemonSet{}
			Expect(
				cl.Get(context.TODO(),
					client.ObjectKey{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})
	})

	Context("check cache", func() {
		It("should cache DaemonSet creation", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			handler := operands.NewDaemonSetHandler(nil, commontestutils.GetScheme(), passt.NewPasstBindingCNIDaemonSet)

			firstCall, err := handler.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(firstCall).ToNot(BeNil())

			secondCall, err := handler.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(secondCall).ToNot(BeNil())

			Expect(firstCall).To(BeIdenticalTo(secondCall))

			ds, ok := firstCall.(*appsv1.DaemonSet)
			Expect(ok).To(BeTrue())
			Expect(ds.Name).To(Equal("passt-binding-cni"))

			handler.Reset()

			thirdCall, err := handler.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(thirdCall).ToNot(BeNil())
			Expect(thirdCall).ToNot(BeIdenticalTo(firstCall))
			Expect(thirdCall).ToNot(BeIdenticalTo(secondCall))
		})
	})

	Context("DaemonSet OpenShift specific configuration", func() {
		BeforeEach(func() {
			getClusterInfo := hcoutil.GetClusterInfo

			hcoutil.GetClusterInfo = func() hcoutil.ClusterInfo {
				return &commontestutils.ClusterInfoMock{}
			}

			DeferCleanup(func() {
				hcoutil.GetClusterInfo = getClusterInfo
			})
		})

		It("should configure for OpenShift cluster", func() {
			ds := passt.NewPasstBindingCNIDaemonSet(hco)
			Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal("passt-binding-cni"))

			Expect(ds.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Volumes[0].HostPath.Path).To(Equal("/var/lib/cni/bin"))
		})
	})
})
