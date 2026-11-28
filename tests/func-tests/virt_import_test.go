package tests_test

import (
	"context"
	"fmt"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promModel "github.com/prometheus/common/model"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	virtimportapi "kubevirt.io/virt-import-api/core"
	virtimportv1alpha1 "kubevirt.io/virt-import-api/core/v1alpha1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	virtImportFGName          = "deployVirtImport"
	virtImportImportName      = "virt-import-functest"
	virtImportTouchAnnotation = "virt-import-functest/touched"
)

var _ = Describe("virt-import Deployment", Label("virt-import"), Serial, Ordered, func() {
	tests.FlagParse()

	var (
		cli             client.Client
		bundle          *virtimport.Bundle
		monitoring      bool
		openshift       bool
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

		err = cli.Get(ctx, client.ObjectKey{Name: "servicemonitors.monitoring.coreos.com"}, &apiextensionsv1.CustomResourceDefinition{})
		Expect(client.IgnoreNotFound(err)).To(Succeed())
		monitoring = err == nil

		openshift, err = tests.IsOpenShift(ctx, cli)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			Expect(tests.DisableFG(ctx, cli, virtImportFGName)).To(Succeed())
		})
	})

	getDeployment := func(ctx context.Context) (*appsv1.Deployment, error) {
		dep := &appsv1.Deployment{}
		err := cli.Get(ctx, client.ObjectKey{Name: bundle.Deployment.Name, Namespace: tests.InstallNamespace}, dep)
		return dep, err
	}

	getResourceVersions := func(ctx context.Context, objs []client.Object) (map[string]string, error) {
		versions := make(map[string]string, len(objs))
		for _, obj := range objs {
			found := obj.DeepCopyObject().(client.Object)
			if err := cli.Get(ctx, client.ObjectKeyFromObject(obj), found); err != nil {
				return nil, fmt.Errorf("failed to get %T %s: %w", obj, obj.GetName(), err)
			}
			version := found.GetResourceVersion()
			if _, ok := found.(*appsv1.Deployment); ok {
				// Status writes of the Deployment controller change the resourceVersion, but not these.
				version = fmt.Sprintf("generation=%d labels=%v", found.GetGeneration(), found.GetLabels())
			}
			versions[fmt.Sprintf("%T/%s", obj, obj.GetName())] = version
		}
		return versions, nil
	}

	When("deployVirtImport feature gate is enabled", func() {
		It("should deploy all virt-import resources", func(ctx context.Context) {
			By("enabling the deployVirtImport feature gate")
			Expect(tests.EnableFG(ctx, cli, virtImportFGName)).To(Succeed())

			By("checking the objects are created")
			for _, obj := range virtImportObjects(bundle, monitoring, openshift) {
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

		It("should be scraped by Prometheus", Label(tests.OpenshiftLabel), func(ctx context.Context) {
			tests.FailIfNotOpenShift(ctx, cli, "virt-import metrics")

			cliSet := tests.GetK8sClientSet()
			promClient := initializePromClient(
				getPrometheusURL(ctx, cliSet.RESTClient()),
				getAuthorizationTokenForPrometheus(ctx, cliSet),
			)

			query := fmt.Sprintf(`up{namespace=%q,service=%q}`, tests.InstallNamespace, bundle.MetricsService.Name)
			Eventually(func(g Gomega, ctx context.Context) {
				result, _, err := promClient.Query(ctx, query, time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				vector, ok := result.(promModel.Vector)
				g.Expect(ok).To(BeTrue(), "expected a Vector result for the %q query", query)
				g.Expect(vector).ToNot(BeEmpty(), "Prometheus has no target for the %q query", query)
				g.Expect(float64(vector[0].Value)).To(Equal(float64(1)),
					"the %q target is down: the scrape reaches the endpoint, but TLS or the client certificate is rejected", query)
			}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())
		})

		It("should not update the virt-import objects on every reconcile", func(ctx context.Context) {
			var objs []client.Object
			for _, obj := range virtImportObjects(bundle, monitoring, openshift) {
				// Kubernetes writes to the token Secret itself, e.g. the legacy-token-last-used label.
				if _, ok := obj.(*v1.Secret); !ok {
					objs = append(objs, obj)
				}
			}
			objs = append(objs, &appsv1.Deployment{ObjectMeta: nameAndNS(bundle.Deployment.Name, tests.InstallNamespace)})
			versions, err := getResourceVersions(ctx, objs)
			Expect(err).ToNot(HaveOccurred())

			By("triggering a reconcile of the HyperConverged")
			Expect(touchHCO(ctx, cli)).To(Succeed())

			Consistently(func(ctx context.Context) (map[string]string, error) {
				return getResourceVersions(ctx, objs)
			}).WithTimeout(time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(Equal(versions))
		})
	})

	When("deployVirtImport feature gate is disabled", func() {
		It("should remove the controller and keep the VirtualMachineImports", func(ctx context.Context) {
			By("creating a VirtualMachineImport and waiting for the controller to add its finalizer")
			vmImport := newVirtImportCR()
			Expect(cli.Create(ctx, vmImport)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				// The controller is gone, so its finalizer has to be removed by hand.
				found := vmImport.DeepCopy()
				err := cli.Get(ctx, client.ObjectKeyFromObject(vmImport), found)
				if apierrors.IsNotFound(err) {
					return
				}
				Expect(err).ToNot(HaveOccurred())
				found.Finalizers = nil
				Expect(cli.Update(ctx, found)).To(Succeed())
				Expect(client.IgnoreNotFound(cli.Delete(ctx, found))).To(Succeed())
			})
			Eventually(func(g Gomega, ctx context.Context) {
				found := vmImport.DeepCopy()
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(vmImport), found)).To(Succeed())
				g.Expect(found.GetFinalizers()).ToNot(BeEmpty())
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("disabling the deployVirtImport feature gate")
			Expect(tests.DisableFG(ctx, cli, virtImportFGName)).To(Succeed())

			By("checking the controller and its objects are removed")
			Eventually(func(ctx context.Context) error {
				_, err := getDeployment(ctx)
				return err
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
				MatchError(apierrors.IsNotFound, "not found error"),
			)
			for _, obj := range virtImportObjects(bundle, monitoring, openshift) {
				Eventually(func(ctx context.Context) error {
					return cli.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
				}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
					MatchError(apierrors.IsNotFound, "not found error"), "%T %s", obj, obj.GetName(),
				)
			}

			By("checking the CRD and the VirtualMachineImport are kept")
			crd := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: nameAndNS(virtimportapi.PluralResourceName+"."+virtimportapi.GroupName, "")}
			for _, obj := range []client.Object{crd, vmImport} {
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))).To(Succeed(), "%T %s", obj, obj.GetName())
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

// virtImportObjects returns the objects HCO deploys besides the Deployment. The
// ServiceMonitor is only deployed with monitoring, the token Secret not on OpenShift.
func virtImportObjects(bundle *virtimport.Bundle, monitoring, openshift bool) []client.Object {
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
	for _, o := range bundle.NetworkPolicies {
		objs = append(objs, &networkingv1.NetworkPolicy{ObjectMeta: nameAndNS(o.Name, tests.InstallNamespace)})
	}
	if !openshift {
		for _, o := range bundle.Secrets {
			objs = append(objs, &v1.Secret{ObjectMeta: nameAndNS(o.Name, tests.InstallNamespace)})
		}
	}
	objs = append(objs, &v1.Service{ObjectMeta: nameAndNS(bundle.MetricsService.Name, tests.InstallNamespace)})
	if monitoring {
		objs = append(objs, &monitoringv1.ServiceMonitor{ObjectMeta: nameAndNS(bundle.ServiceMonitor.Name, tests.InstallNamespace)})
	}
	return objs
}

// touchHCO changes an annotation of the HyperConverged, which makes HCO reconcile it. The
// annotation is removed again on cleanup.
func touchHCO(ctx context.Context, cli client.Client) error {
	GinkgoHelper()

	hc, err := tests.GetHCO(ctx, cli)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(hc.DeepCopy())
	if hc.Annotations == nil {
		hc.Annotations = map[string]string{}
	}
	hc.Annotations[virtImportTouchAnnotation] = time.Now().Format(time.RFC3339Nano)
	if err := cli.Patch(ctx, hc, patch); err != nil {
		return err
	}

	DeferCleanup(func(ctx context.Context) error {
		hc, err := tests.GetHCO(ctx, cli)
		if err != nil {
			return err
		}
		patch := client.MergeFrom(hc.DeepCopy())
		delete(hc.Annotations, virtImportTouchAnnotation)
		return cli.Patch(ctx, hc, patch)
	})
	return nil
}

func nameAndNS(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: namespace}
}

// newVirtImportCR returns a VirtualMachineImport. Importing is not expected to succeed, it only
// has to get the finalizer of the controller.
func newVirtImportCR() *virtimportv1alpha1.VirtualMachineImport {
	return &virtimportv1alpha1.VirtualMachineImport{
		ObjectMeta: metav1.ObjectMeta{Name: virtImportImportName, Namespace: tests.InstallNamespace},
		Spec: virtimportv1alpha1.VirtualMachineImportSpec{
			Source: virtimportv1alpha1.VirtualMachineImportSource{
				Registry: &virtimportv1alpha1.VirtualMachineImportRegistrySource{
					URL: "docker://registry.invalid/virt-import-functest:latest",
				},
			},
		},
	}
}

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
