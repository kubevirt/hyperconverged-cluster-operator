package operands

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	mtqv1alpha1 "kubevirt.io/managed-tenant-quota/staging/src/kubevirt.io/managed-tenant-quota-api/pkg/apis/core/v1alpha1"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("MTQ tests", func() {
	var (
		hco *v1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client

		testNodePlacement = api.NodePlacement{
			NodeSelector: map[string]string{
				"test": "testing",
			},
			Tolerations: []corev1.Toleration{{Key: "test", Operator: corev1.TolerationOpEqual, Value: "test", Effect: corev1.TaintEffectNoSchedule}},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchFields: []corev1.NodeSelectorRequirement{
									{
										Key:      "test",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"test"},
									},
								},
							},
						},
					},
				},
			},
		}
	)
	//var

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("test NewMTQ", func() {
		It("should have all default fields", func() {
			mtq := NewMTQ(hco)

			Expect(mtq.Name).Should(Equal("mtq-" + hco.Name))
			Expect(mtq.Namespace).Should(BeEmpty())

			Expect(mtq.Spec.Infra.Tolerations).Should(BeEmpty())
			Expect(mtq.Spec.Infra.Affinity).Should(BeNil())
			Expect(mtq.Spec.Infra.NodeSelector).Should(BeEmpty())

			Expect(mtq.Spec.Workloads.Tolerations).Should(BeEmpty())
			Expect(mtq.Spec.Workloads.Affinity).Should(BeNil())
			Expect(mtq.Spec.Workloads.NodeSelector).Should(BeEmpty())

			Expect(mtq.Spec.PriorityClass).To(HaveValue(Equal(mtqv1alpha1.MTQPriorityClass(kvPriorityClass))))

			Expect(mtq.Spec.CertConfig.CA).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.CA.Duration).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.CA.Duration.Duration.String()).Should(Equal("48h0m0s"))
			Expect(mtq.Spec.CertConfig.CA.RenewBefore.Duration.String()).Should(Equal("24h0m0s"))

			Expect(mtq.Spec.CertConfig.Server).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.Server.Duration).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.Server.Duration.Duration.String()).Should(Equal("24h0m0s"))
			Expect(mtq.Spec.CertConfig.Server.RenewBefore.Duration.String()).Should(Equal("12h0m0s"))
		})

		It("should get node placement node placement configurations from the HyperConverged CR", func() {
			hco.Spec.Infra.NodePlacement = &testNodePlacement
			hco.Spec.Workloads.NodePlacement = &testNodePlacement

			mtq := NewMTQ(hco)

			Expect(mtq.Spec.Infra).Should(Equal(testNodePlacement))
			Expect(mtq.Spec.Workloads).Should(Equal(testNodePlacement))
		})

		It("should get node placement certification configurations from the HyperConverged CR", func() {

			hco.Spec.CertConfig = v1beta1.HyperConvergedCertConfig{
				CA: v1beta1.CertRotateConfigCA{
					Duration:    &metav1.Duration{Duration: time.Hour * 72},
					RenewBefore: &metav1.Duration{Duration: time.Hour * 56},
				},
				Server: v1beta1.CertRotateConfigServer{
					Duration:    &metav1.Duration{Duration: time.Hour * 36},
					RenewBefore: &metav1.Duration{Duration: time.Hour * 18},
				},
			}

			mtq := NewMTQ(hco)

			Expect(mtq.Spec.CertConfig.CA).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.CA.Duration).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.CA.Duration.Duration.String()).Should(Equal("72h0m0s"))
			Expect(mtq.Spec.CertConfig.CA.RenewBefore.Duration.String()).Should(Equal("56h0m0s"))

			Expect(mtq.Spec.CertConfig.Server).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.Server.Duration).ShouldNot(BeNil())
			Expect(mtq.Spec.CertConfig.Server.Duration.Duration.String()).Should(Equal("36h0m0s"))
			Expect(mtq.Spec.CertConfig.Server.RenewBefore.Duration.String()).Should(Equal("18h0m0s"))
		})
	})

	Context("check FG", func() {
		It("should not create MTQ if the FG is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := newMtqHandler(cl, commontestutils.GetScheme())

			res := handler.ensure(req)

			Expect(res.Err).ShouldNot(HaveOccurred())
			Expect(res.Created).Should(BeFalse())
			Expect(res.Updated).Should(BeFalse())
			Expect(res.Deleted).Should(BeFalse())

			foundMTQs := &mtqv1alpha1.MTQList{}
			Expect(cl.List(context.Background(), foundMTQs)).Should(Succeed())
			Expect(foundMTQs.Items).Should(BeEmpty())
		})

		It("should delete MTQ if the FG is not set", func() {
			mtq := NewMTQ(hco)
			cl = commontestutils.InitClient([]client.Object{hco, mtq})

			handler := newMtqHandler(cl, commontestutils.GetScheme())

			res := handler.ensure(req)

			Expect(res.Err).ShouldNot(HaveOccurred())
			Expect(res.Name).Should(Equal(mtq.Name))
			Expect(res.Created).Should(BeFalse())
			Expect(res.Updated).Should(BeFalse())
			Expect(res.Deleted).Should(BeTrue())

			foundMTQs := &mtqv1alpha1.MTQList{}
			Expect(cl.List(context.Background(), foundMTQs)).Should(Succeed())
			Expect(foundMTQs.Items).Should(BeEmpty())
		})

		It("should create MTQ if the FG is set", func() {
			hco.Spec.FeatureGates.EnableManagedTenantQuota = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := newMtqHandler(cl, commontestutils.GetScheme())

			res := handler.ensure(req)

			Expect(res.Err).ShouldNot(HaveOccurred())
			Expect(res.Name).Should(Equal("mtq-kubevirt-hyperconverged"))
			Expect(res.Created).Should(BeTrue())
			Expect(res.Updated).Should(BeFalse())
			Expect(res.Deleted).Should(BeFalse())

			foundMTQ := &mtqv1alpha1.MTQ{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundMTQ)).Should(Succeed())

			Expect(foundMTQ.Name).Should(Equal("mtq-" + hco.Name))
			Expect(foundMTQ.Namespace).Should(BeEmpty())

			// example of field set by the handler
			Expect(foundMTQ.Spec.PriorityClass).To(HaveValue(Equal(mtqv1alpha1.MTQPriorityClass(kvPriorityClass))))
		})
	})

	Context("check update", func() {

		It("should update MTQ fields, if not matched to the requirements", func() {
			wrongPC := mtqv1alpha1.MTQPriorityClass("wrongPC")

			hco.Spec.FeatureGates.EnableManagedTenantQuota = ptr.To(true)
			mtq := NewMTQWithNameOnly(hco)
			mtq.Spec.Infra = testNodePlacement
			mtq.Spec.PriorityClass = &wrongPC
			mtq.Spec.CertConfig = &mtqv1alpha1.MTQCertConfig{
				CA: &mtqv1alpha1.CertConfig{
					Duration:    &metav1.Duration{Duration: time.Hour * 72},
					RenewBefore: &metav1.Duration{Duration: time.Hour * 56},
				},
			}

			cl = commontestutils.InitClient([]client.Object{hco, mtq})
			handler := newMtqHandler(cl, commontestutils.GetScheme())

			res := handler.ensure(req)

			Expect(res.Err).ShouldNot(HaveOccurred())
			Expect(res.Created).Should(BeFalse())
			Expect(res.Deleted).Should(BeFalse())
			Expect(res.Updated).Should(BeTrue())

			foundMTQ := &mtqv1alpha1.MTQ{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundMTQ)).Should(Succeed())
			Expect(foundMTQ.Spec.Infra.Affinity).Should(BeNil())
			Expect(foundMTQ.Spec.Infra.NodeSelector).Should(BeEmpty())
			Expect(foundMTQ.Spec.Infra.Tolerations).Should(BeEmpty())

			Expect(foundMTQ.Spec.PriorityClass).To(HaveValue(Equal(mtqv1alpha1.MTQPriorityClass(kvPriorityClass))))
			Expect(foundMTQ.Spec.CertConfig.CA).ShouldNot(BeNil())
			Expect(foundMTQ.Spec.CertConfig.CA.Duration).ShouldNot(BeNil())
			Expect(foundMTQ.Spec.CertConfig.CA.Duration.Duration.String()).Should(Equal("48h0m0s"))
			Expect(foundMTQ.Spec.CertConfig.CA.RenewBefore.Duration.String()).Should(Equal("24h0m0s"))

			Expect(foundMTQ.Spec.CertConfig.Server).ShouldNot(BeNil())
			Expect(foundMTQ.Spec.CertConfig.Server.Duration).ShouldNot(BeNil())
			Expect(foundMTQ.Spec.CertConfig.Server.Duration.Duration.String()).Should(Equal("24h0m0s"))
			Expect(foundMTQ.Spec.CertConfig.Server.RenewBefore.Duration.String()).Should(Equal("12h0m0s"))
		})
	})

	Context("check cache", func() {
		It("should create new cache if it empty", func() {
			hco.Spec.FeatureGates.EnableManagedTenantQuota = ptr.To(true)
			handler := newMtqHandler(cl, commontestutils.GetScheme())
			op, ok := handler.(*mtqOperand)
			Expect(ok).Should(BeTrue())

			hooks, ok := op.operand.hooks.(*mtqHooks)
			Expect(ok).Should(BeTrue())

			Expect(hooks.cache).Should(BeNil())

			res := handler.ensure(req)
			Expect(res.Err).ShouldNot(HaveOccurred())

			cache := hooks.cache
			Expect(cache).ShouldNot(BeNil())

			Expect(hooks.getFullCr(hco)).Should(BeIdenticalTo(cache))

			By("recreate cache after reset")
			handler.reset()
			Expect(hooks.cache).Should(BeNil())
			res = handler.ensure(req)
			Expect(res.Err).ShouldNot(HaveOccurred())

			Expect(hooks.cache).ShouldNot(BeIdenticalTo(cache))
			mtq, _ := hooks.getFullCr(hco)
			Expect(mtq).ShouldNot(BeIdenticalTo(cache))
			Expect(mtq).Should(BeIdenticalTo(hooks.cache))
		})
	})
})
