package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"

	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/observability"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/alertmanager"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	testName                       = "observability_controller"
	observabilityControllerName    = "virt-observability-controller"
	observabilityControllerCRBName = "virt-observability-controller-rolebinding"
	observabilityControllerFGName  = "deployObservabilityController"
)

var _ = Describe("Observability Controller", Label(tests.OpenshiftLabel, testName), func() {
	var (
		cli             client.Client
		cliConfig       *rest.Config
		httpClient      http.Client
		alertmanagerURL string
	)

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		cliConfig = tests.GetClientConfig()
		tests.FailIfNotOpenShift(ctx, cli, testName)

		httpClient = http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}

		routeHost, err := getAlertmanagerRouteHost(ctx, cli)
		Expect(err).ToNot(HaveOccurred())
		Expect(routeHost).ToNot(BeEmpty())
		alertmanagerURL = fmt.Sprintf("https://%s", routeHost)

		// Ensure we have a valid bearer token for authentication
		if cliConfig.BearerToken == "" {
			token, err := getServiceAccountToken(ctx)
			Expect(err).ToNot(HaveOccurred())
			cliConfig.BearerToken = token
		}
	})

	AfterEach(func(ctx context.Context) {
		tests.WaitForHCOOperatorRollout(ctx)
	})

	Context("PodDisruptionBudgetAtLimit", func() {
		It("should be silenced", func(ctx context.Context) {
			amAPI := alertmanager.NewAPI(httpClient, alertmanagerURL, cliConfig.BearerToken)

			By("Verifying the PodDisruptionBudgetAtLimit silence exists")
			amSilences, err := amAPI.ListSilences()
			Expect(err).ToNot(HaveOccurred())

			podDisruptionBudgetAtLimitSilence := observability.FindPodDisruptionBudgetAtLimitSilence(amSilences)
			Expect(podDisruptionBudgetAtLimitSilence).ToNot(BeNil())

			By("Deleting the silence and waiting for it to be removed")
			err = amAPI.DeleteSilence(podDisruptionBudgetAtLimitSilence.ID)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				amSilences, err := amAPI.ListSilences()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(observability.FindPodDisruptionBudgetAtLimitSilence(amSilences)).To(BeNil())
			}).WithTimeout(30 * time.Second).WithPolling(time.Second).Should(Succeed())

			By("Restarting the HCO operator pods to force reconciliation")
			var hcoPods v1.PodList
			err = cli.List(ctx, &hcoPods, &client.MatchingLabels{
				"name": "hyperconverged-cluster-operator",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(hcoPods.Items).ToNot(BeEmpty())

			for _, pod := range hcoPods.Items {
				err = cli.Delete(ctx, &pod)
				Expect(err).ToNot(HaveOccurred())
			}

			By("Waiting for the HCO operator to roll out")
			tests.WaitForHCOOperatorRollout(ctx)

			By("Waiting for the controller to recreate the silence")
			Eventually(func(g Gomega) {
				amSilences, err := amAPI.ListSilences()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(observability.FindPodDisruptionBudgetAtLimitSilence(amSilences)).ToNot(BeNil())
			}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
		})
	})
})

var _ = Describe("Observability Controller Deployment", Label(tests.OpenshiftLabel, "observability-controller"), Serial, Ordered, func() {
	tests.FlagParse()

	var cli client.Client

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
	})

	AfterAll(func(ctx context.Context) {
		tests.RestoreDefaultFeatureGates(ctx, cli)
	})

	When("deployObservabilityController feature gate is enabled", func() {
		It("should deploy observability controller resources", func(ctx context.Context) {
			By("enabling the deployObservabilityController feature gate")
			Expect(tests.EnableFG(ctx, cli, observabilityControllerFGName)).To(Succeed())

			By("checking the ServiceAccount is created")
			Eventually(func(ctx context.Context) error {
				sa := &v1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      observabilityControllerName,
						Namespace: tests.InstallNamespace,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(sa), sa)
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("checking the ClusterRole is created")
			Eventually(func(ctx context.Context) error {
				cr := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: observabilityControllerName,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(cr), cr)
			}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("checking the ClusterRoleBinding is created")
			Eventually(func(ctx context.Context) error {
				crb := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: observabilityControllerCRBName,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(crb), crb)
			}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("checking the Deployment is created")
			Eventually(func(ctx context.Context) error {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      observabilityControllerName,
						Namespace: tests.InstallNamespace,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())
		})
	})

	When("deployObservabilityController feature gate is disabled", func() {
		It("should remove observability controller resources", func(ctx context.Context) {
			By("disabling the deployObservabilityController feature gate")
			tests.RestoreDefaultFeatureGates(ctx, cli)

			By("checking the Deployment is removed")
			Eventually(func(ctx context.Context) error {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      observabilityControllerName,
						Namespace: tests.InstallNamespace,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
				MatchError(ContainSubstring("not found")),
			)

			By("checking the ClusterRole is removed")
			Eventually(func(ctx context.Context) error {
				cr := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: observabilityControllerName,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(cr), cr)
			}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
				MatchError(ContainSubstring("not found")),
			)
		})
	})
})

var _ = Describe("Observability Controller Allowlist Configuration", Label(tests.OpenshiftLabel, "observability-controller"), Serial, Ordered, func() {
	tests.FlagParse()

	var cli client.Client

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		By("enabling the deployObservabilityController feature gate")
		Expect(tests.EnableFG(ctx, cli, observabilityControllerFGName)).To(Succeed())

		By("waiting for the deployment to be created")
		Eventually(func(ctx context.Context) error {
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      observabilityControllerName,
					Namespace: tests.InstallNamespace,
				},
			}
			return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
		}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		tests.PatchHCO(ctx, cli, []byte(`[{"op": "remove", "path": "/spec/observability"}]`))
		tests.RestoreDefaultFeatureGates(ctx, cli)
	})

	getDeploymentArgs := func(ctx context.Context) ([]string, error) {
		dep := &appsv1.Deployment{}
		err := cli.Get(ctx, types.NamespacedName{
			Name:      observabilityControllerName,
			Namespace: tests.InstallNamespace,
		}, dep)
		if err != nil {
			return nil, err
		}
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return nil, fmt.Errorf("deployment has no containers")
		}
		return dep.Spec.Template.Spec.Containers[0].Args, nil
	}

	It("should not add allowlist flags when observability is not configured (means all)", func(ctx context.Context) {
		Eventually(func(g Gomega, ctx context.Context) {
			args, err := getDeploymentArgs(ctx)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(args).ToNot(ContainElement(ContainSubstring("--metrics-allowlist")))
			g.Expect(args).ToNot(ContainElement(ContainSubstring("--alerts-allowlist")))
			g.Expect(args).ToNot(ContainElement(ContainSubstring("--recording-rules-allowlist")))
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).WithContext(ctx).Should(Succeed())
	})

	It("should propagate specific metrics allowlist to the deployment", func(ctx context.Context) {
		patch := []byte(`[{"op": "add", "path": "/spec/observability", "value": {"workloads": {"allowedMetrics": ["kubevirt_vmi_memory_used_bytes", "kubevirt_vmi_cpu_usage_seconds_total"]}, "allowedAlerts": ["KubeVirtVMDown"], "allowedRecordingRules": ["kubevirt_vmi_phase_count:sum"]}}]`)
		tests.PatchHCO(ctx, cli, patch)

		Eventually(func(g Gomega, ctx context.Context) {
			args, err := getDeploymentArgs(ctx)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(args).To(ContainElement("--metrics-allowlist=kubevirt_vmi_memory_used_bytes,kubevirt_vmi_cpu_usage_seconds_total"))
			g.Expect(args).To(ContainElement("--alerts-allowlist=KubeVirtVMDown"))
			g.Expect(args).To(ContainElement("--recording-rules-allowlist=kubevirt_vmi_phase_count:sum"))
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).WithContext(ctx).Should(Succeed())
	})

	It("should reject mixed 'none' with other values", func(ctx context.Context) {
		hco := tests.HCOWithNameOnly()
		patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op": "replace", "path": "/spec/observability", "value": {"allowedAlerts": ["none", "KubeVirtVMDown"]}}]`))
		err := cli.Patch(ctx, hco, patch)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot be combined"))
	})
})

func getAlertmanagerRouteHost(ctx context.Context, cli client.Client) (string, error) {
	route := &routev1.Route{}
	err := cli.Get(ctx, types.NamespacedName{
		Name:      "alertmanager-main",
		Namespace: "openshift-monitoring",
	}, route)
	if err != nil {
		return "", err
	}

	if len(route.Status.Ingress) > 0 {
		return route.Status.Ingress[0].Host, nil
	}

	return "", fmt.Errorf("route has no ingress status")
}

// getServiceAccountToken uses the prometheus-k8s service account from openshift-monitoring
// to get a token that can be used to access the Alertmanager API
// This follows the same pattern as the monitoring_test.go
func getServiceAccountToken(ctx context.Context) (string, error) {
	k8sClientSet := tests.GetK8sClientSet()

	treq, err := k8sClientSet.CoreV1().ServiceAccounts("openshift-monitoring").CreateToken(
		ctx,
		"prometheus-k8s",
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				// Avoid specifying any audiences so that the token will be
				// issued for the default audience of the issuer.
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}

	if treq.Status.Token == "" {
		return "", fmt.Errorf("received empty token from TokenRequest")
	}

	return treq.Status.Token, nil
}
