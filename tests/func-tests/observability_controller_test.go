package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"

	v1 "k8s.io/api/core/v1"
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
	})

	Context("PodDisruptionBudgetAtLimit", func() {
		It("should be silenced", func(ctx context.Context) {
			amAPI := alertmanager.NewAPI(httpClient, alertmanagerURL, cliConfig.BearerToken)

			amSilences, err := amAPI.ListSilences()
			Expect(err).ToNot(HaveOccurred())

			// PodDisruptionBudgetAtLimit silence should have been created by the controller
			podDisruptionBudgetAtLimitSilence := observability.FindPodDisruptionBudgetAtLimitSilence(amSilences)
			Expect(podDisruptionBudgetAtLimitSilence).ToNot(BeNil())

			err = amAPI.DeleteSilence(podDisruptionBudgetAtLimitSilence.ID)
			Expect(err).ToNot(HaveOccurred())

			// Restart pod to force reconcile (reconcile periodicity is 1h)
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

			// Wait for the controller to recreate the silence
			Eventually(func() bool {
				amSilences, err := amAPI.ListSilences()
				Expect(err).ToNot(HaveOccurred())

				return observability.FindPodDisruptionBudgetAtLimitSilence(amSilences) != nil
			}, "5m", "10s").Should(BeTrue())
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
