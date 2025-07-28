package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
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
		portForwardCmd  *exec.Cmd
		alertmanagerURL string
	)

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		cliConfig = tests.GetClientConfig()
		tests.FailIfNotOpenShift(ctx, cli, testName)

		httpClient = http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}

		if isAlertmanagerAccessible(httpClient, observability.AlertmanagerSvcHost, cliConfig.BearerToken) {
			alertmanagerURL = observability.AlertmanagerSvcHost
		} else {
			var localPort int
			var err error

			portForwardCmd, localPort, err = setupPortForward()
			Expect(err).ToNot(HaveOccurred())

			alertmanagerURL = fmt.Sprintf("https://localhost:%d", localPort)
		}
	})

	AfterEach(func(ctx context.Context) {
		cleanupPortForward(portForwardCmd)
	})

	Context("PodDisruptionBudgetAtLimit", func() {
		BeforeEach(func(ctx context.Context) {
			cli := tests.GetControllerRuntimeClient()
			tests.FailIfNotOpenShift(ctx, cli, testName)
		})

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

func isAlertmanagerAccessible(httpClient http.Client, svcHost string, bearerToken string) bool {
	req, err := http.NewRequest("GET", svcHost+"/-/healthy", nil)
	if err != nil {
		return false
	}

	if bearerToken != "" {
		req.Header.Add("Authorization", "Bearer "+bearerToken)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func setupPortForward() (*exec.Cmd, int, error) {
	serviceName := "alertmanager-main"
	localPort := 9094

	portForwardCmd := exec.Command("oc", "port-forward", "-n", "openshift-monitoring", fmt.Sprintf("service/%s", serviceName), fmt.Sprintf("%d:9094", localPort))

	if err := portForwardCmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("failed to start port-forward to service %s: %w", serviceName, err)
	}

	// Wait a bit for port-forward to establish
	time.Sleep(3 * time.Second)

	return portForwardCmd, localPort, nil
}

func cleanupPortForward(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}
