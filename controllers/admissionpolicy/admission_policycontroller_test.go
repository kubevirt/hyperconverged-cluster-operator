package admissionpolicy

import (
	"context"
	"os"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var ref = &metav1.OwnerReference{
	APIVersion:         apiextensionsv1.SchemeGroupVersion.String(),
	Kind:               "CustomResourceDefinition",
	Name:               crdName,
	UID:                types.UID("12345678"),
	Controller:         ptr.To(false),
	BlockOwnerDeletion: ptr.To(true),
}

func TestAdmissionPolicyController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AdmissionPolicy Controller Suite")
}

var _ = Describe("AdmissionPolicyController", func() {

	BeforeEach(func() {
		origNS, hasNSVar := os.LookupEnv(hcoutil.OperatorNamespaceEnv)
		Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)).To(Succeed())

		fakeownresources.OLMV0OwnResourcesMock()
		resetOnces()

		DeferCleanup(func() {
			if hasNSVar {
				Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, origNS)).To(Succeed())
			} else {
				Expect(os.Unsetenv(hcoutil.OperatorNamespaceEnv)).To(Succeed())
			}

			fakeownresources.ResetOwnResources()
			resetOnces()
		})
	})

	Context("test creation", func() {
		It("Should create a new AdmissionPolicy and binding on startup", func(ctx context.Context) {
			var resources []client.Object
			cli := commontestutils.InitClient(resources)

			r := &ReconcileAdmissionPolicy{
				Client: cli,
				owner:  ref,
			}
			res, err := r.Reconcile(ctx, startupReq)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			policy := getRequiredPolicy(ref)
			foundPolicy := &admissionv1.ValidatingAdmissionPolicy{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(policy), foundPolicy)).To(Succeed())
			Expect(foundPolicy.Spec).To(Equal(policy.Spec))

			binding := getRequiredBinding(ref)
			foundBinding := &admissionv1.ValidatingAdmissionPolicyBinding{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(binding), foundBinding)).To(Succeed())
			Expect(foundBinding.Spec).To(Equal(binding.Spec))
		})

		It("Should only create a new AdmissionPolicy if it's missing", func(ctx context.Context) {
			var resources []client.Object
			cli := commontestutils.InitClient(resources)

			r := &ReconcileAdmissionPolicy{
				Client: cli,
				owner:  ref,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: policyName,
				},
			}

			res, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			policy := getRequiredPolicy(ref)
			foundPolicy := &admissionv1.ValidatingAdmissionPolicy{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(policy), foundPolicy)).To(Succeed())
			Expect(foundPolicy.Spec).To(Equal(policy.Spec))

			binding := getRequiredBinding(ref)
			foundBinding := &admissionv1.ValidatingAdmissionPolicyBinding{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(binding), foundBinding)).To(MatchError(k8serrors.IsNotFound, "expected to not be found"))
		})

		It("Should only create a new Binding if it's missing", func(ctx context.Context) {
			var resources []client.Object
			cli := commontestutils.InitClient(resources)

			r := &ReconcileAdmissionPolicy{
				Client: cli,
				owner:  ref,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: policyBindingName,
				},
			}

			res, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			policy := getRequiredPolicy(ref)
			foundPolicy := &admissionv1.ValidatingAdmissionPolicy{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(policy), foundPolicy)).To(MatchError(k8serrors.IsNotFound, "expected to not be found"))

			binding := getRequiredBinding(ref)
			foundBinding := &admissionv1.ValidatingAdmissionPolicyBinding{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(binding), foundBinding)).To(Succeed())
			Expect(foundBinding.Spec).To(Equal(binding.Spec))
		})
	})

	Context("test update", func() {
		var (
			cli client.Client
		)

		const (
			wrongExpr       = "wrong expression"
			wrongPolicyName = "wrongPolicyName"
			wrongUID        = types.UID("87654321")
		)

		BeforeEach(func() {
			modifiedPolicy := getRequiredPolicy(ref)
			modifiedPolicy.Spec.Validations[0].Expression = wrongExpr
			modifiedPolicy.OwnerReferences[0].UID = wrongUID

			modifiedPolicyBinding := getRequiredBinding(ref)
			modifiedPolicyBinding.Spec.PolicyName = wrongPolicyName
			modifiedPolicyBinding.OwnerReferences[0].UID = wrongUID

			resources := []client.Object{modifiedPolicy, modifiedPolicyBinding}

			cli = commontestutils.InitClient(resources)
		})

		It("Should update the AdmissionPolicy and the binding on startup", func(ctx context.Context) {
			r := &ReconcileAdmissionPolicy{
				Client: cli,
				owner:  ref,
			}

			res, err := r.Reconcile(ctx, startupReq)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			policy := getRequiredPolicy(ref)
			foundPolicy := &admissionv1.ValidatingAdmissionPolicy{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(policy), foundPolicy)).To(Succeed())
			Expect(foundPolicy.Spec).To(Equal(policy.Spec))
			Expect(foundPolicy.OwnerReferences).To(HaveLen(1))
			Expect(foundPolicy.OwnerReferences[0]).To(Equal(*ref))

			binding := getRequiredBinding(ref)
			foundBinding := &admissionv1.ValidatingAdmissionPolicyBinding{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(binding), foundBinding)).To(Succeed())
			Expect(foundBinding.Spec).To(Equal(binding.Spec))
			Expect(foundBinding.OwnerReferences).To(HaveLen(1))
			Expect(foundBinding.OwnerReferences[0]).To(Equal(*ref))
		})

		It("Should only update the AdmissionPolicy if was modified", func(ctx context.Context) {
			r := &ReconcileAdmissionPolicy{
				Client: cli,
				owner:  ref,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: policyName,
				},
			}

			res, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			policy := getRequiredPolicy(ref)
			foundPolicy := &admissionv1.ValidatingAdmissionPolicy{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(policy), foundPolicy)).To(Succeed())
			Expect(foundPolicy.Spec).To(Equal(policy.Spec))

			binding := getRequiredBinding(ref)
			foundBinding := &admissionv1.ValidatingAdmissionPolicyBinding{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(binding), foundBinding)).To(Succeed())
			Expect(foundBinding.Spec.PolicyName).To(Equal(wrongPolicyName))
			Expect(foundBinding.OwnerReferences).To(HaveLen(1))
			Expect(foundBinding.OwnerReferences[0].UID).To(Equal(wrongUID))
		})

		It("Should only update the Binding if it was changed", func(ctx context.Context) {
			r := &ReconcileAdmissionPolicy{
				Client: cli,
				owner:  ref,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: policyBindingName,
				},
			}

			res, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			policy := getRequiredPolicy(ref)
			foundPolicy := &admissionv1.ValidatingAdmissionPolicy{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(policy), foundPolicy)).To(Succeed())
			Expect(foundPolicy.Spec.Validations[0].Expression).To(Equal(wrongExpr))
			Expect(foundPolicy.OwnerReferences).To(HaveLen(1))
			Expect(foundPolicy.OwnerReferences[0].UID).To(Equal(wrongUID))

			binding := getRequiredBinding(ref)
			foundBinding := &admissionv1.ValidatingAdmissionPolicyBinding{}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(binding), foundBinding)).To(Succeed())
			Expect(foundBinding.Spec).To(Equal(binding.Spec))
		})
	})
})

func resetOnces() {
	policyOnce = &sync.Once{}
	bindingOnce = &sync.Once{}
}
