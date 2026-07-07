package netresinjector

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Network Resources Injector PodDisruptionBudget", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Spec.Deployment.DeployNetworkResourcesInjector = new(true)
		req = commontestutils.NewReq(hco)
	})

	Context("newPDB", func() {
		It("should have all default values", func() {
			pdb := newPDB()
			Expect(pdb.Name).To(Equal(deploymentName + "-pdb"))
			Expect(pdb.Namespace).To(Equal(hco.Namespace))
			Expect(pdb.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(pdb.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))

			Expect(pdb.Spec.MinAvailable).ToNot(BeNil())
			Expect(pdb.Spec.MinAvailable.IntValue()).To(Equal(1))
			Expect(pdb.Spec.Selector).ToNot(BeNil())
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))
		})
	})

	Context("PDB handler", func() {
		It("should create PDB if it does not exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewPDBHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundPDBs := &policyv1.PodDisruptionBudgetList{}
			Expect(cl.List(context.Background(), foundPDBs)).To(Succeed())
			Expect(foundPDBs.Items).To(HaveLen(1))
			Expect(foundPDBs.Items[0].Name).To(Equal(deploymentName + "-pdb"))
		})
	})

	Context("PDB update", func() {
		It("should update PDB spec if not matched to requirements", func() {
			pdb := newPDB()
			pdb.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: 99}
			cl = commontestutils.InitClient([]client.Object{hco, pdb})

			handler := NewPDBHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundPDB := &policyv1.PodDisruptionBudget{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: deploymentName + "-pdb", Namespace: hco.Namespace}, foundPDB)).To(Succeed())
			Expect(foundPDB.Spec.MinAvailable.IntValue()).To(Equal(1))
		})

		It("should not update PDB if spec already matches", func() {
			pdb := newPDB()
			cl = commontestutils.InitClient([]client.Object{hco, pdb})

			handler := NewPDBHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeFalse())
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			pdb := newPDB()
			expectedLabels := maps.Clone(pdb.Labels)
			delete(pdb.Labels, hcoutil.AppLabelComponent)
			pdb.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, pdb})

			handler := NewPDBHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundPDB := &policyv1.PodDisruptionBudget{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: deploymentName + "-pdb", Namespace: hco.Namespace}, foundPDB)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundPDB.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundPDB.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
