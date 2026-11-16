package virtimport

import (
	"context"
	"os"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	testControllerImage = "quay.io/kubevirt/virt-import-controller@sha256:1111"
	testFetcherImage    = "quay.io/kubevirt/virt-import-fetcher@sha256:2222"
)

func setEnv(name, value string) {
	orig, wasSet := os.LookupEnv(name)
	Expect(os.Setenv(name, value)).To(Succeed())
	DeferCleanup(func() {
		if wasSet {
			Expect(os.Setenv(name, orig)).To(Succeed())
		} else {
			Expect(os.Unsetenv(name)).To(Succeed())
		}
	})
}

func setupImageEnv() {
	setEnv(hcoutil.VirtImportControllerImageEnvV, testControllerImage)
	setEnv(hcoutil.VirtImportFetcherImageEnvV, testFetcherImage)
}

// reconcile runs all handlers in order and stops at the first error, as OperandHandler.Ensure does.
func reconcile(ops []operands.Operand, req *common.HcoRequest) error {
	for _, op := range ops {
		if res := op.Ensure(req); res.Err != nil {
			return res.Err
		}
	}
	return nil
}

// bundleObjects returns empty objects for everything the handlers deploy, keyed for a Get.
func bundleObjects() []client.Object {
	ops := newOperands(nil)
	objs := make([]client.Object, 0, len(ops))
	for _, op := range ops {
		getter, ok := op.(operands.CRGetter)
		Expect(ok).To(BeTrue())
		obj, err := getter.GetFullCr(commontestutils.NewHco())
		Expect(err).ToNot(HaveOccurred())
		objs = append(objs, obj)
	}
	return objs
}

func isPolicy(o client.Object) bool {
	_, vap := o.(*admissionregistrationv1.ValidatingAdmissionPolicy)
	_, vapb := o.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding)
	return vap || vapb
}

func exists(cl client.Client, obj client.Object) bool {
	err := cl.Get(context.Background(), client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
	if apierrors.IsNotFound(err) {
		return false
	}
	Expect(err).ToNot(HaveOccurred())
	return true
}

var _ = Describe("virt-import handlers", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
		setupImageEnv()
	})

	It("should register the admission policies first", func() {
		objs := bundleObjects()
		firstOther := slices.IndexFunc(objs, func(o client.Object) bool { return !isPolicy(o) })
		Expect(firstOther).To(BeNumerically(">", 0))
		for _, o := range objs[firstOther:] {
			Expect(isPolicy(o)).To(BeFalse(), "%T %s comes after the other objects", o, o.GetName())
		}
	})

	It("should deploy all bundle objects with HCO labels when the feature gate is enabled", func() {
		hco.Spec.FeatureGates.Enable(featureGateName)
		cl := commontestutils.InitClient([]client.Object{hco})

		Expect(reconcile(newOperands(cl), req)).To(Succeed())

		for _, obj := range bundleObjects() {
			found := obj.DeepCopyObject().(client.Object)
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(obj), found)).To(Succeed())
			Expect(found.GetLabels()).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(found.GetLabels()).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(appComponent)))
			if found.GetNamespace() != "" {
				Expect(found.GetNamespace()).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
			}
		}
	})

	It("should set and remove the VirtImportReady condition", func() {
		hco.Spec.FeatureGates.Enable(featureGateName)
		cl := commontestutils.InitClient([]client.Object{hco})
		ops := newOperands(cl)
		Expect(reconcile(ops, req)).To(Succeed())

		cond := apimeta.FindStatusCondition(req.Instance.Status.Conditions, hcov1.ConditionVirtImportReady)
		Expect(cond).ToNot(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))

		hco.Spec.FeatureGates.Disable(featureGateName)
		Expect(reconcile(ops, req)).To(Succeed())
		Expect(apimeta.FindStatusCondition(req.Instance.Status.Conditions, hcov1.ConditionVirtImportReady)).To(BeNil())
	})

	It("should be stable on a second reconcile", func() {
		hco.Spec.FeatureGates.Enable(featureGateName)
		cl := commontestutils.InitClient([]client.Object{hco})
		ops := newOperands(cl)
		Expect(reconcile(ops, req)).To(Succeed())

		for _, op := range ops {
			res := op.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse(), "%s %s was updated", res.Type, res.Name)
		}
	})

	It("should not deploy anything when the feature gate is not set", func() {
		cl := commontestutils.InitClient([]client.Object{hco})

		Expect(reconcile(newOperands(cl), req)).To(Succeed())

		for _, obj := range bundleObjects() {
			Expect(exists(cl, obj)).To(BeFalse(), "%T %s exists", obj, obj.GetName())
		}
	})

	It("should remove everything when the feature gate is disabled", func() {
		hco.Spec.FeatureGates.Enable(featureGateName)
		cl := commontestutils.InitClient([]client.Object{hco})
		ops := newOperands(cl)
		Expect(reconcile(ops, req)).To(Succeed())

		hco.Spec.FeatureGates.Disable(featureGateName)
		Expect(reconcile(ops, req)).To(Succeed())

		By("keeping the admission policies until the controller is gone")
		for _, obj := range bundleObjects() {
			Expect(exists(cl, obj)).To(Equal(isPolicy(obj)), "%T %s", obj, obj.GetName())
		}

		Expect(reconcile(ops, req)).To(Succeed())
		for _, obj := range bundleObjects() {
			Expect(exists(cl, obj)).To(BeFalse(), "%T %s was not removed", obj, obj.GetName())
		}
	})
})
