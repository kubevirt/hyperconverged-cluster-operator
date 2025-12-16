package handlers

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	migrationv1alpha1 "kubevirt.io/kubevirt-migration-operator/api/v1alpha1"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Migration tests", func() {
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

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("test NewMigController", func() {
		It("should have all default fields", func() {
			migController, err := NewMigController(hco)
			Expect(err).ToNot(HaveOccurred())

			Expect(migController.Name).To(Equal("migcontroller-" + hco.Name))
			Expect(migController.Namespace).To(Equal(hco.Namespace))

			Expect(migController.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

			Expect(migController.Spec.Infra.Tolerations).To(BeEmpty())
			Expect(migController.Spec.Infra.Affinity).To(BeNil())
			Expect(migController.Spec.Infra.NodeSelector).To(BeEmpty())
		})

		It("should get node placement configurations from the HyperConverged CR", func() {
			hco.Spec.Infra.NodePlacement = &testNodePlacement

			migController, err := NewMigController(hco)
			Expect(err).ToNot(HaveOccurred())

			Expect(migController.Spec.Infra).To(Equal(testNodePlacement))
		})
	})

	Context("check handler Ensure", func() {
		It("should create MigController if it doesn't exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewMigControllerHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("migcontroller-kubevirt-hyperconverged"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundMigController := &migrationv1alpha1.MigController{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundMigController)).To(Succeed())

			Expect(foundMigController.Name).To(Equal("migcontroller-" + hco.Name))
			Expect(foundMigController.Namespace).To(Equal(hco.Namespace))
			Expect(foundMigController.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

		It("should update MigController fields, if not matched to the requirements", func() {
			migController := NewMigControllerWithNameOnly(hco)
			migController.Spec.ImagePullPolicy = corev1.PullAlways
			migController.Spec.Infra = testNodePlacement

			cl = commontestutils.InitClient([]client.Object{hco, migController})
			handler := NewMigControllerHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())
			Expect(res.Updated).To(BeTrue())

			foundMigController := &migrationv1alpha1.MigController{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundMigController)).To(Succeed())

			Expect(foundMigController.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(foundMigController.Spec.Infra.Affinity).To(BeNil())
			Expect(foundMigController.Spec.Infra.NodeSelector).To(BeEmpty())
			Expect(foundMigController.Spec.Infra.Tolerations).To(BeEmpty())
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			outdatedResource := NewMigControllerWithNameOnly(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)
			for k, v := range expectedLabels {
				outdatedResource.Labels[k] = "wrong_" + v
			}
			outdatedResource.Labels[userLabelKey] = userLabelValue

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewMigControllerHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &migrationv1alpha1.MigController{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
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
			outdatedResource := NewMigControllerWithNameOnly(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)
			outdatedResource.Labels[userLabelKey] = userLabelValue
			delete(outdatedResource.Labels, hcoutil.AppLabelVersion)

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewMigControllerHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &migrationv1alpha1.MigController{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})
	})

	Context("check cache", func() {
		It("should create new cache if it empty", func() {
			hook := &migrationHooks{}
			Expect(hook.cache).To(BeNil())

			firstCallResult, err := hook.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(firstCallResult).ToNot(BeNil())
			Expect(hook.cache).To(BeIdenticalTo(firstCallResult))

			secondCallResult, err := hook.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(secondCallResult).ToNot(BeNil())
			Expect(hook.cache).To(BeIdenticalTo(secondCallResult))
			Expect(firstCallResult).To(BeIdenticalTo(secondCallResult))

			hook.Reset()
			Expect(hook.cache).To(BeNil())

			thirdCallResult, err := hook.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(thirdCallResult).ToNot(BeNil())
			Expect(hook.cache).To(BeIdenticalTo(thirdCallResult))
			Expect(thirdCallResult).ToNot(BeIdenticalTo(firstCallResult))
			Expect(thirdCallResult).ToNot(BeIdenticalTo(secondCallResult))
		})
	})
})
