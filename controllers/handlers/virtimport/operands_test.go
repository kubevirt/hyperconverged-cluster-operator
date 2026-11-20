package virtimport

import (
	"context"
	"os"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	testFinalizer       = "test.kubevirt.io/finalizer"
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

	It("should register the admission policies before the CRDs and the CRDs before the rest", func() {
		objs := bundleObjects()
		isPolicy := func(o client.Object) bool {
			_, vap := o.(*admissionregistrationv1.ValidatingAdmissionPolicy)
			_, vapb := o.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding)
			return vap || vapb
		}
		isCRD := func(o client.Object) bool {
			_, ok := o.(*apiextensionsv1.CustomResourceDefinition)
			return ok
		}

		firstCRD, lastCRD := -1, -1
		for i, o := range objs {
			if isCRD(o) {
				if firstCRD == -1 {
					firstCRD = i
				}
				lastCRD = i
			}
		}
		Expect(firstCRD).ToNot(Equal(-1))
		for i, o := range objs {
			switch {
			case isPolicy(o):
				Expect(i).To(BeNumerically("<", firstCRD), "%T %s", o, o.GetName())
			case !isCRD(o):
				Expect(i).To(BeNumerically(">", lastCRD), "%T %s", o, o.GetName())
			}
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

	Context("feature gate disabled", func() {
		It("should delete the CRDs first and the rest once they are gone", func() {
			hco.Spec.FeatureGates.Enable(featureGateName)
			cl := commontestutils.InitClient([]client.Object{hco})
			Expect(reconcile(newOperands(cl), req)).To(Succeed())

			// A finalizer keeps the CRDs terminating, like CRs still being cleaned up.
			for _, bundleCRD := range crds() {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(bundleCRD), crd)).To(Succeed())
				crd.Finalizers = append(crd.Finalizers, testFinalizer)
				Expect(cl.Update(context.Background(), crd)).To(Succeed())
			}

			hco.Spec.FeatureGates.Disable(featureGateName)
			ops := newOperands(cl)
			Expect(reconcile(ops, req)).To(Succeed())

			for _, obj := range bundleObjects() {
				if crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition); ok {
					found := &apiextensionsv1.CustomResourceDefinition{}
					Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(crd), found)).To(Succeed())
					Expect(found.DeletionTimestamp).ToNot(BeNil())
					continue
				}
				Expect(exists(cl, obj)).To(BeTrue(), "%T %s was removed before the CRDs", obj, obj.GetName())
			}

			for _, bundleCRD := range crds() {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(bundleCRD), crd)).To(Succeed())
				crd.Finalizers = nil
				Expect(cl.Update(context.Background(), crd)).To(Succeed())
			}

			Expect(reconcile(ops, req)).To(Succeed())

			for _, obj := range bundleObjects() {
				Expect(exists(cl, obj)).To(BeFalse(), "%T %s was not removed", obj, obj.GetName())
			}
		})
	})

	It("should wait without an error while a CRD is terminating and the feature gate is enabled", func() {
		hco.Spec.FeatureGates.Enable(featureGateName)
		cl := commontestutils.InitClient([]client.Object{hco})
		ops := newOperands(cl)
		Expect(reconcile(ops, req)).To(Succeed())

		crd := &apiextensionsv1.CustomResourceDefinition{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(crds()[0]), crd)).To(Succeed())
		crd.Finalizers = []string{testFinalizer}
		Expect(cl.Update(context.Background(), crd)).To(Succeed())
		Expect(cl.Delete(context.Background(), crd)).To(Succeed())

		Expect(reconcile(ops, req)).To(Succeed())
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(crd), crd)).To(Succeed())
		Expect(crd.DeletionTimestamp).ToNot(BeNil())
	})

	It("should not delete a terminating CRD again when the feature gate is disabled", func() {
		hco.Spec.FeatureGates.Enable(featureGateName)
		cl := commontestutils.InitClient([]client.Object{hco})
		ops := newOperands(cl)
		Expect(reconcile(ops, req)).To(Succeed())

		crd := &apiextensionsv1.CustomResourceDefinition{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(crds()[0]), crd)).To(Succeed())
		crd.Finalizers = []string{testFinalizer}
		Expect(cl.Update(context.Background(), crd)).To(Succeed())

		hco.Spec.FeatureGates.Disable(featureGateName)
		crdOp := ops[slices.IndexFunc(ops, func(op operands.Operand) bool {
			_, ok := op.(*crdHandler)
			return ok
		})]
		Expect(crdOp.Ensure(req).Deleted).To(BeTrue())
		Expect(crdOp.Ensure(req).Deleted).To(BeFalse())
	})

	It("should remove the other objects but keep a CRD HCO does not manage", func() {
		hco.Spec.FeatureGates.Enable(featureGateName)
		cl := commontestutils.InitClient([]client.Object{hco})
		Expect(reconcile(newOperands(cl), req)).To(Succeed())

		for _, c := range crds() {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(c), crd)).To(Succeed())
			crd.Labels = nil
			Expect(cl.Update(context.Background(), crd)).To(Succeed())
		}

		hco.Spec.FeatureGates.Disable(featureGateName)
		Expect(reconcile(newOperands(cl), req)).To(Succeed())

		for _, obj := range bundleObjects() {
			_, isCRD := obj.(*apiextensionsv1.CustomResourceDefinition)
			Expect(exists(cl, obj)).To(Equal(isCRD), "%T %s", obj, obj.GetName())
		}
	})

	Context("CRDs", func() {
		It("should return all CRDs with HCO labels and type information", func() {
			crds := crds()
			Expect(crds).ToNot(BeEmpty())
			for _, crd := range crds {
				Expect(crd.Kind).To(Equal("CustomResourceDefinition"))
				Expect(crd.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			}
		})
	})
})
