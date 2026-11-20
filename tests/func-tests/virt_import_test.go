package tests_test

import (
	"context"
	"fmt"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	virtImportFGName          = "deployVirtImport"
	virtImportImportName      = "virt-import-functest"
	virtImportImportKind      = "VirtualMachineImport"
	virtImportTouchAnnotation = "virt-import-functest/touched"
)

var _ = Describe("virt-import Deployment", Label("virt-import"), Serial, Ordered, func() {
	tests.FlagParse()

	var (
		cli             client.Client
		bundle          *virtimport.Bundle
		controllerImage string
		fetcherImage    string
	)

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		By("checking the virt-import images are configured")
		var err error
		controllerImage, fetcherImage, err = getVirtImportImages(ctx, cli)
		Expect(err).ToNot(HaveOccurred())
		Expect(controllerImage != "" && fetcherImage != "").To(BeTrueBecause(`the VIRT_IMPORT_CONTROLLER_IMAGE and VIRT_IMPORT_FETCHER_IMAGE environment variables should be configured in operator deployment. Use the "!virt-import" label filter in order to skip this test`))

		bundle, err = virtimport.GetBundle()
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			tests.RestoreDefaultFeatureGates(ctx, cli)
		})
	})

	getDeployment := func(ctx context.Context) (*appsv1.Deployment, error) {
		dep := &appsv1.Deployment{}
		err := cli.Get(ctx, client.ObjectKey{Name: bundle.Deployment.Name, Namespace: tests.InstallNamespace}, dep)
		return dep, err
	}

	getResourceVersions := func(ctx context.Context, objs []client.Object) map[string]string {
		versions := make(map[string]string, len(objs))
		for _, obj := range objs {
			found := obj.DeepCopyObject().(client.Object)
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(obj), found)).To(Succeed())
			versions[fmt.Sprintf("%T/%s", obj, obj.GetName())] = found.GetResourceVersion()
		}
		return versions
	}

	When("deployVirtImport feature gate is enabled", func() {
		It("should deploy all virt-import resources", func(ctx context.Context) {
			By("enabling the deployVirtImport feature gate")
			Expect(tests.EnableFG(ctx, cli, virtImportFGName)).To(Succeed())

			By("checking the CRDs are established")
			for _, crd := range bundle.CRDs {
				Eventually(func(g Gomega, ctx context.Context) {
					found := &apiextensionsv1.CustomResourceDefinition{}
					g.Expect(cli.Get(ctx, client.ObjectKey{Name: crd.Name}, found)).To(Succeed())
					g.Expect(found.Status.Conditions).To(ContainElement(And(
						HaveField("Type", apiextensionsv1.Established),
						HaveField("Status", apiextensionsv1.ConditionTrue),
					)))
				}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())
			}

			By("checking the other objects are created")
			for _, obj := range virtImportObjects(bundle) {
				Eventually(func(ctx context.Context) error {
					return cli.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
				}).WithTimeout(2*time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed(), "%T %s", obj, obj.GetName())
			}

			By("checking the Deployment uses the images of the HCO Deployment and is available")
			Eventually(func(g Gomega, ctx context.Context) {
				dep, err := getDeployment(ctx)
				g.Expect(err).ToNot(HaveOccurred())
				manager := dep.Spec.Template.Spec.Containers[slices.IndexFunc(dep.Spec.Template.Spec.Containers, func(c v1.Container) bool {
					return c.Name == virtimport.ManagerContainerName
				})]
				g.Expect(manager.Image).To(Equal(controllerImage))
				g.Expect(manager.Env).To(ContainElement(v1.EnvVar{Name: virtimport.FetcherImageEnvName, Value: fetcherImage}))
				g.Expect(dep.Status.AvailableReplicas).To(BeNumerically(">=", 1))
			}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(Succeed())

			By("checking the VirtImportReady condition")
			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).ToNot(HaveOccurred())
				cond := apimeta.FindStatusCondition(hc.Status.Conditions, hcov1.ConditionVirtImportReady)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(Succeed())
		})

		It("should not update the virt-import objects on every reconcile", func(ctx context.Context) {
			var objs []client.Object
			for _, crd := range bundle.CRDs {
				objs = append(objs, crd)
			}
			objs = append(objs, virtImportObjects(bundle)...)
			objs = append(objs, &appsv1.Deployment{ObjectMeta: nameAndNS(bundle.Deployment.Name, tests.InstallNamespace)})
			versions := getResourceVersions(ctx, objs)

			By("triggering a reconcile of the HyperConverged")
			touchHCO(ctx, cli)

			Consistently(func(ctx context.Context) map[string]string {
				return getResourceVersions(ctx, objs)
			}).WithTimeout(time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(Equal(versions))
		})
	})

	When("deployVirtImport feature gate is disabled", func() {
		It("should remove the VirtualMachineImports, the CRDs and then the other resources", func(ctx context.Context) {
			By("creating a VirtualMachineImport and waiting for the controller to add its finalizer")
			vmImport := newVirtImportCR(bundle)
			Expect(cli.Create(ctx, vmImport)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				err := cli.Delete(ctx, vmImport)
				if err != nil && !apierrors.IsNotFound(err) && !apimeta.IsNoMatchError(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			})
			Eventually(func(g Gomega, ctx context.Context) {
				found := vmImport.DeepCopy()
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(vmImport), found)).To(Succeed())
				g.Expect(found.GetFinalizers()).ToNot(BeEmpty())
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("disabling the deployVirtImport feature gate")
			tests.RestoreDefaultFeatureGates(ctx, cli)

			By("checking the CRDs, and with them the VirtualMachineImport, are removed")
			for _, crd := range bundle.CRDs {
				Eventually(func(ctx context.Context) error {
					return cli.Get(ctx, client.ObjectKey{Name: crd.Name}, &apiextensionsv1.CustomResourceDefinition{})
				}).WithTimeout(3 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
					MatchError(apierrors.IsNotFound, "not found error"),
				)
			}

			By("checking the other objects are removed")
			Eventually(func(ctx context.Context) error {
				_, err := getDeployment(ctx)
				return err
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
				MatchError(apierrors.IsNotFound, "not found error"),
			)
			for _, obj := range virtImportObjects(bundle) {
				Eventually(func(ctx context.Context) error {
					return cli.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
				}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
					MatchError(apierrors.IsNotFound, "not found error"), "%T %s", obj, obj.GetName(),
				)
			}

			By("checking the VirtImportReady condition is removed")
			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(apimeta.FindStatusCondition(hc.Status.Conditions, hcov1.ConditionVirtImportReady)).To(BeNil())
			}).WithTimeout(time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(Succeed())
		})
	})
})

// virtImportObjects returns all objects HCO deploys for virt-import besides the CRDs and the
// Deployment, keyed for a Get in the HCO namespace.
func virtImportObjects(bundle *virtimport.Bundle) []client.Object {
	var objs []client.Object
	for _, o := range bundle.ServiceAccounts {
		objs = append(objs, &v1.ServiceAccount{ObjectMeta: nameAndNS(o.Name, tests.InstallNamespace)})
	}
	for _, o := range bundle.Roles {
		objs = append(objs, &rbacv1.Role{ObjectMeta: nameAndNS(o.Name, tests.InstallNamespace)})
	}
	for _, o := range bundle.RoleBindings {
		objs = append(objs, &rbacv1.RoleBinding{ObjectMeta: nameAndNS(o.Name, tests.InstallNamespace)})
	}
	for _, o := range bundle.ClusterRoles {
		objs = append(objs, &rbacv1.ClusterRole{ObjectMeta: nameAndNS(o.Name, "")})
	}
	for _, o := range bundle.ClusterRoleBindings {
		objs = append(objs, &rbacv1.ClusterRoleBinding{ObjectMeta: nameAndNS(o.Name, "")})
	}
	for _, o := range bundle.ValidatingAdmissionPolicies {
		objs = append(objs, &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: nameAndNS(o.Name, "")})
	}
	for _, o := range bundle.ValidatingAdmissionPolicyBindings {
		objs = append(objs, &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: nameAndNS(o.Name, "")})
	}
	objs = append(objs, &networkingv1.NetworkPolicy{ObjectMeta: nameAndNS(bundle.Deployment.Name+"-np", tests.InstallNamespace)})
	return objs
}

// touchHCO changes an annotation of the HyperConverged, which makes HCO reconcile it.
func touchHCO(ctx context.Context, cli client.Client) {
	hc, err := tests.GetHCO(ctx, cli)
	Expect(err).ToNot(HaveOccurred())
	patch := client.MergeFrom(hc.DeepCopy())
	if hc.Annotations == nil {
		hc.Annotations = map[string]string{}
	}
	hc.Annotations[virtImportTouchAnnotation] = time.Now().Format(time.RFC3339Nano)
	Expect(cli.Patch(ctx, hc, patch)).To(Succeed())

	DeferCleanup(func(ctx context.Context) {
		hc, err := tests.GetHCO(ctx, cli)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(hc.DeepCopy())
		delete(hc.Annotations, virtImportTouchAnnotation)
		Expect(cli.Patch(ctx, hc, patch)).To(Succeed())
	})
}

func nameAndNS(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: namespace}
}

// newVirtImportCR returns a VirtualMachineImport. Importing is not expected to succeed, it only
// has to exist while the CRDs are removed.
func newVirtImportCR(bundle *virtimport.Bundle) *unstructured.Unstructured {
	crdIdx := slices.IndexFunc(bundle.CRDs, func(crd *apiextensionsv1.CustomResourceDefinition) bool {
		return crd.Spec.Names.Kind == virtImportImportKind
	})
	Expect(crdIdx).ToNot(Equal(-1), "the bundle has no %s CRD", virtImportImportKind)
	crd := bundle.CRDs[crdIdx]

	idx := slices.IndexFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
		return v.Storage
	})
	Expect(idx).ToNot(Equal(-1))

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crd.Spec.Versions[idx].Name,
		Kind:    crd.Spec.Names.Kind,
	})
	obj.SetName(virtImportImportName)
	obj.SetNamespace(tests.InstallNamespace)
	obj.Object["spec"] = map[string]any{
		"source": map[string]any{
			"registry": map[string]any{
				"url": "docker://registry.invalid/virt-import-functest:latest",
			},
		},
	}
	return obj
}

// getVirtImportImages returns the virt-import images set on the HCO Deployment.
func getVirtImportImages(ctx context.Context, cli client.Client) (string, string, error) {
	depList := &appsv1.DeploymentList{}
	err := cli.List(ctx, depList, client.InNamespace(tests.InstallNamespace), client.MatchingLabels{"name": "hyperconverged-cluster-operator"})
	if err != nil {
		return "", "", fmt.Errorf("failed to list operator deployments: %w", err)
	}
	if len(depList.Items) == 0 {
		return "", "", fmt.Errorf("no hyperconverged-cluster-operator deployment found in namespace %s", tests.InstallNamespace)
	}
	if len(depList.Items[0].Spec.Template.Spec.Containers) == 0 {
		return "", "", fmt.Errorf("operator deployment has no containers")
	}

	var controllerImage, fetcherImage string
	for _, e := range depList.Items[0].Spec.Template.Spec.Containers[0].Env {
		switch e.Name {
		case hcoutil.VirtImportControllerImageEnvV:
			controllerImage = e.Value
		case hcoutil.VirtImportFetcherImageEnvV:
			fetcherImage = e.Value
		}
	}
	return controllerImage, fetcherImage, nil
}
