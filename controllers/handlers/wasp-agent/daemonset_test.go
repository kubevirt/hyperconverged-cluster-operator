package wasp_agent

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("Wasp Agent DaemonSet", func() {
	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		ds  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("Wasp DaemonSet deployment", func() {
		It("should not create if overcommit percent is less or equal to 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			ds = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentDaemonSetHandler(ds, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDs := &appsv1.DaemonSetList{}
			Expect(ds.List(context.Background(), foundDs)).To(Succeed())
			Expect(foundDs.Items).To(BeEmpty())
		})
		It("should delete DaemonSet when percentage is set to 100 and below", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			scc := newWaspAgentDaemonSet(hco)
			ds = commontestutils.InitClient([]client.Object{hco, scc})

			handler := NewWaspAgentDaemonSetHandler(ds, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(scc.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundDs := &appsv1.DaemonSetList{}
			Expect(ds.List(context.Background(), foundDs)).To(Succeed())
			Expect(foundDs.Items).To(BeEmpty())
		})
		It("should create Deamonset when percentage is set to higher than 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			ds = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentDaemonSetHandler(ds, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("wasp-agent"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDs := &appsv1.DaemonSetList{}
			Expect(ds.List(context.Background(), foundDs)).To(Succeed())
			Expect(foundDs.Items).To(HaveLen(1))
			Expect(foundDs.Items[0].Name).To(Equal("wasp-agent"))
		})
	})
	Context("Wasp agent DaemonSet update", func() {
		It("should update DaemonSet fields if not matched to the requirements", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			originalDs := newWaspAgentDaemonSet(hco)
			modifiedDs := originalDs.DeepCopy()
			modifiedDs.Spec.Template.Spec.Containers[0].Image = "malicious:tag"
			modifiedDs.Spec.Template.Spec.HostPID = false
			modifiedDs.Spec.Template.Spec.Containers[0].SecurityContext.Privileged = ptr.To(false)
			modifiedDs.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("500m")
			modifiedDs.Spec.Template.Spec.Volumes = nil
			ds = commontestutils.InitClient([]client.Object{hco, modifiedDs})
			handler := NewWaspAgentDaemonSetHandler(ds, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			reconciledDs := &appsv1.DaemonSet{}
			Expect(ds.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, reconciledDs)).To(Succeed())

			Expect(reconciledDs.Spec.Template.Spec.Containers[0].Image).
				To(Equal(originalDs.Spec.Template.Spec.Containers[0].Image))
			Expect(reconciledDs.Spec.Template.Spec.HostPID).
				To(Equal(originalDs.Spec.Template.Spec.HostPID))
			Expect(reconciledDs.Spec.Template.Spec.Containers[0].SecurityContext.Privileged).
				To(Equal(originalDs.Spec.Template.Spec.Containers[0].SecurityContext.Privileged))
			Expect(reconciledDs.Spec.Template.Spec.Containers[0].Resources.Requests).
				To(Equal(originalDs.Spec.Template.Spec.Containers[0].Resources.Requests))
			Expect(reconciledDs.Spec.Template.Spec.Volumes).
				To(Equal(originalDs.Spec.Template.Spec.Volumes))
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			ds := newWaspAgentDaemonSet(hco)
			expectedLabels := maps.Clone(ds.Labels)
			delete(ds.Labels, "app.kubernetes.io/component")
			ds.Labels["user-added-label"] = "user-value"
			cli := commontestutils.InitClient([]client.Object{hco, ds})
			handler := NewWaspAgentDaemonSetHandler(cli, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundDs := &appsv1.DaemonSet{}
			Expect(cli.Get(context.Background(), client.ObjectKey{Name: "wasp-agent", Namespace: hco.Namespace}, foundDs)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundDs.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundDs.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})

})
