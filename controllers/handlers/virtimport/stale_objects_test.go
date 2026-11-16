package virtimport

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("RemoveStaleObjects", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	virtImportLabels := func() map[string]string {
		return operands.GetLabels(appComponent)
	}

	newClusterRole := func(name string, labels map[string]string) *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}

	newRole := func(name, namespace string) *rbacv1.Role {
		return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: virtImportLabels()}}
	}

	It("should remove virt-import objects that are no longer in the bundle", func() {
		staleRole := newClusterRole("virt-import-old-role", virtImportLabels())
		staleSA := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name: "virt-import-old-sa", Namespace: hcoutil.GetOperatorNamespaceFromEnv(), Labels: virtImportLabels(),
		}}
		cl := commontestutils.InitClient([]client.Object{hco, staleRole, staleSA})

		Expect(RemoveStaleObjects(req, cl)).To(Succeed())

		Expect(exists(cl, staleRole)).To(BeFalse())
		Expect(exists(cl, staleSA)).To(BeFalse())
	})

	It("should remove the related object of a removed object", func() {
		staleRole := newClusterRole("virt-import-old-role", virtImportLabels())
		cl := commontestutils.InitClient([]client.Object{hco, staleRole})
		ref, err := reference.GetReference(cl.Scheme(), staleRole)
		Expect(err).ToNot(HaveOccurred())
		hco.Status.RelatedObjects = []corev1.ObjectReference{*ref}

		Expect(RemoveStaleObjects(req, cl)).To(Succeed())

		Expect(hco.Status.RelatedObjects).To(BeEmpty())
		Expect(req.StatusDirty).To(BeTrue())
	})

	It("should keep the objects of the bundle", func() {
		bundleRole := getBundle().ClusterRoles[0]
		currentRole := newClusterRole(bundleRole.Name, virtImportLabels())
		cl := commontestutils.InitClient([]client.Object{hco, currentRole})

		Expect(RemoveStaleObjects(req, cl)).To(Succeed())

		Expect(exists(cl, currentRole)).To(BeTrue())
		Expect(req.StatusDirty).To(BeFalse())
	})

	It("should keep objects of other components and objects without the HCO labels", func() {
		otherComponent := newClusterRole("other-component-role", operands.GetLabels(hcoutil.AppComponentNetResInjector))
		unlabeled := newClusterRole("virt-import-unmanaged-role", nil)
		cl := commontestutils.InitClient([]client.Object{hco, otherComponent, unlabeled})

		Expect(RemoveStaleObjects(req, cl)).To(Succeed())

		Expect(exists(cl, otherComponent)).To(BeTrue())
		Expect(exists(cl, unlabeled)).To(BeTrue())
	})

	It("should keep namespaced objects outside the operator namespace", func() {
		otherNamespace := newRole("virt-import-old-role", "other-namespace")
		cl := commontestutils.InitClient([]client.Object{hco, otherNamespace})

		Expect(RemoveStaleObjects(req, cl)).To(Succeed())

		Expect(exists(cl, otherNamespace)).To(BeTrue())
	})

	It("should keep CRDs that are no longer in the bundle", func() {
		staleCRD := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
			Name: "oldimports.import.kubevirt.io", Labels: virtImportLabels(),
		}}
		cl := commontestutils.InitClient([]client.Object{hco, staleCRD})

		Expect(RemoveStaleObjects(req, cl)).To(Succeed())

		Expect(exists(cl, staleCRD)).To(BeTrue())
	})
})
