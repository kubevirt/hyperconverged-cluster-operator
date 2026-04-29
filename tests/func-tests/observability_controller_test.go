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

	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/observability"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/alertmanager"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const testName = "observability_controller"

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
