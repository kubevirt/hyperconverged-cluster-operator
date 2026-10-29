package tests_test

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"iter"
	"net/http"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	openshiftroutev1 "github.com/openshift/api/route/v1"
	deschedulerv1 "github.com/openshift/cluster-kube-descheduler-operator/pkg/apis/descheduler/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promApi "github.com/prometheus/client_golang/api"
	promApiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promConfig "github.com/prometheus/common/config"
	promModel "github.com/prometheus/common/model"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoalerts "github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/rules/alerts"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var runbookClient = http.DefaultClient

const (
	prometheousTimeout = 5 * time.Minute
	prometheousPolling = 10 * time.Second
)

var _ = Describe("[crit:high][vendor:cnv-qe@redhat.com][level:system]Monitoring", Serial, Ordered, Label(tests.OpenshiftLabel, "monitoring"), func() {
	flag.Parse()

	var (
		cli            client.Client
		cliSet         *kubernetes.Clientset
		restClient     rest.Interface
		promClient     promApiv1.API
		prometheusRule monitoringv1.PrometheusRule
	)

	runbookClient.Timeout = time.Second * 3

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		cliSet = tests.GetK8sClientSet()
		restClient = cliSet.RESTClient()

		tests.FailIfNotOpenShift(ctx, cli, "Prometheus")
		promClient = initializePromClient(getPrometheusURL(ctx, restClient), getAuthorizationTokenForPrometheus(ctx, cliSet))
		prometheusRule = getPrometheusRule(ctx, restClient)
	})

	It("Alert rules should have all the required annotations", func() {
		for _, group := range prometheusRule.Spec.Groups {
			for _, rule := range group.Rules {
				if rule.Alert != "" {
					Expect(rule.Annotations).To(HaveKeyWithValue("summary", Not(BeEmpty())),
						"%s summary is missing or empty", rule.Alert)
					Expect(rule.Annotations).To(HaveKey("runbook_url"),
						"%s runbook_url is missing", rule.Alert)
					Expect(rule.Annotations).To(HaveKeyWithValue("runbook_url", ContainSubstring(rule.Alert)),
						"%s runbook_url doesn't include alert name", rule.Alert)
					checkRunbookURLAvailability(rule)
				}
			}
		}
	})

	It("Alert rules should have all the requried labels", func() {
		for _, group := range prometheusRule.Spec.Groups {
			for _, rule := range group.Rules {
				if rule.Alert != "" {
					Expect(rule.Labels).To(HaveKeyWithValue("severity", BeElementOf("info", "warning", "critical")),
						"%s severity label is missing or not valid", rule.Alert)
					Expect(rule.Labels).To(HaveKeyWithValue("kubernetes_operator_part_of", "kubevirt"),
						"%s kubernetes_operator_part_of label is missing or not valid", rule.Alert)
					Expect(rule.Labels).To(HaveKeyWithValue("kubernetes_operator_component", "hyperconverged-cluster-operator"),
						"%s kubernetes_operator_component label is missing or not valid", rule.Alert)
					Expect(rule.Labels).To(HaveKeyWithValue("operator_health_impact", BeElementOf("none", "warning", "critical")),
						"%s operator_health_impact label is missing or not valid", rule.Alert)
				}
			}
		}
	})

	It("KubeVirtCRModified alert should fired when there is a modification on a CR", Serial, func(ctx context.Context) {

		const (
			query     = `kubevirt_hco_out_of_band_modifications_total{component_name="kubevirt/kubevirt-kubevirt-hyperconverged"}`
			jsonPatch = `[{"op": "add", "path": "/spec/configuration/developerConfiguration/featureGates/-", "value": "fake-fg-for-testing"}]`
		)

		By(fmt.Sprintf("Reading the `%s` metric", query))
		var valueBefore float64
		Eventually(func(g Gomega, ctx context.Context) {
			val, found, err := getMetricValue(ctx, promClient, query)
			g.Expect(err).NotTo(HaveOccurred())
			if !found {
				valueBefore = 0
			} else {
				valueBefore = val
			}
		}).WithContext(ctx).
			WithTimeout(5 * time.Minute).
			WithPolling(10 * time.Second).
			Should(Succeed())

		GinkgoWriter.Printf("The metric value before the test is: %0.2f\n", valueBefore)

		By("Patching kubevirt object")
		patch := client.RawPatch(types.JSONPatchType, []byte(jsonPatch))

		kv := &kubevirtcorev1.KubeVirt{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubevirt-kubevirt-hyperconverged",
				Namespace: tests.InstallNamespace,
			},
		}

		Eventually(cli.Patch).
			WithContext(ctx).
			WithArguments(kv, patch).
			WithTimeout(time.Minute).
			WithPolling(time.Second).
			To(Succeed())

		By("checking that the HCO metric was increased by 1")
		waitForMetricValue(ctx, promClient, query, valueBefore+1)

		By("Checking the alert")
		Eventually(func(ctx context.Context) *promApiv1.Alert {
			alerts, err := promClient.Alerts(ctx)
			Expect(err).ToNot(HaveOccurred())
			alert := getAlertByName(alerts, "KubeVirtCRModified")
			return alert
		}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).ShouldNot(BeNil())
	})

	It("UnsupportedHCOModification alert should fired when there is a jsonpatch annotation to modify an operand CRs", func(ctx context.Context) {
		query := fmt.Sprintf(`kubevirt_hco_unsafe_modifications{annotation_name=%q}`, common.JSONPatchKVAnnotationName)

		hco, err := tests.GetHCO(ctx, cli)
		Expect(err).ToNot(HaveOccurred())

		if hco.Annotations == nil {
			hco.Annotations = make(map[string]string)
		}
		hco.Annotations[common.JSONPatchKVAnnotationName] = `[{"op": "add", "path": "/spec/configuration/migrations", "value": {"allowPostCopy": true}}]`

		DeferCleanup(func(ctx context.Context) {
			By("Cleaning up: removing the jsonpatch annotation from HCO")
			cleanHCO, err := tests.GetHCO(ctx, cli)
			Expect(err).ToNot(HaveOccurred())

			if cleanHCO.Annotations != nil {
				delete(cleanHCO.Annotations, common.JSONPatchKVAnnotationName)
			}

			tests.UpdateHCORetry(ctx, cli, cleanHCO)

			By("Waiting for the metric to return to 0")
			waitForMetricValue(ctx, promClient, query, float64(0))

			By("Waiting for the alert to clear")
			Eventually(func(ctx context.Context) *promApiv1.Alert {
				alerts, err := promClient.Alerts(ctx)
				Expect(err).ToNot(HaveOccurred())
				return getAlertByName(alerts, "UnsupportedHCOModification")
			}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).Should(BeNil())
		})

		tests.UpdateHCORetry(ctx, cli, hco)

		By(fmt.Sprintf("Reading the `%s` metric from HCO prometheus endpoint", query))
		waitForMetricValue(ctx, promClient, query, float64(1))

		By("Checking the alert in Prometheus")
		var alerts promApiv1.AlertsResult
		Eventually(func(g Gomega, ctx context.Context) *promApiv1.Alert {
			var err error
			alerts, err = promClient.Alerts(ctx)
			g.Expect(err).ToNot(HaveOccurred())
			return getAlertByName(alerts, "UnsupportedHCOModification")
		}).WithTimeout(prometheousTimeout).
			WithPolling(prometheousPolling).
			WithContext(ctx).
			ShouldNot(BeNil(), func() func() string {
				hc, err := tests.GetHCO(ctx, cli)
				if err != nil {
					return func() string {
						return "Error reading HyperConverged CR; " + err.Error()
					}
				}
				return tests.PrintHyperConvergedBecause(hc,
					`can't find the "UnsupportedHCOModification" alert in the list of firing alerts: %s`, getAlertNames(alerts),
				)
			})
	})

	It("should fire the DeprecatedMachineType alert when a VM is using a deprecated machine type", Label(tests.DeprecatedArchLabel), func(ctx context.Context) {
		archs, err := getArchs(ctx)
		Expect(err).ToNot(HaveOccurred())

		var deprecatedMachineType string
		switch {
		case slices.Contains(archs, "amd64"):
			deprecatedMachineType = "pc-q35-rhel7.6.0"
		case slices.Contains(archs, "s390x"):
			deprecatedMachineType = "s390-ccw-virtio-rhel7.6.0"
		default:
			Fail(fmt.Sprintf("no known deprecated machine type for architectures %v; Use the \"!%s\" label filter in order to skip this test", archs, tests.DeprecatedArchLabel))
		}

		By("Creating a VM with a deprecated machine type")
		vm := &kubevirtcorev1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deprecated-vm",
				Namespace: tests.TestNamespace,
			},
			Spec: kubevirtcorev1.VirtualMachineSpec{
				RunStrategy: new(kubevirtcorev1.RunStrategyAlways),
				Template: &kubevirtcorev1.VirtualMachineInstanceTemplateSpec{
					Spec: kubevirtcorev1.VirtualMachineInstanceSpec{
						Domain: kubevirtcorev1.DomainSpec{
							Machine: &kubevirtcorev1.Machine{Type: deprecatedMachineType},
							Resources: kubevirtcorev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
		}

		Eventually(cli.Create).
			WithContext(ctx).
			WithArguments(vm).
			WithTimeout(time.Minute).
			WithPolling(time.Second * 10).
			Should(Succeed())

		DeferCleanup(func(ctx context.Context) {
			Eventually(cli.Delete).
				WithContext(ctx).
				WithArguments(vm).
				WithTimeout(time.Minute * 5).
				WithPolling(time.Second * 10).
				Should(Succeed())
		})

		By("Ensuring the DeprecatedMachineType alert is firing")
		Eventually(func(ctx context.Context) *promApiv1.Alert {
			alerts, err := promClient.Alerts(ctx)
			Expect(err).ToNot(HaveOccurred())
			return getAlertByName(alerts, "DeprecatedMachineType")
		}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).ShouldNot(BeNil())
	})

	Describe("KubeDescheduler", Serial, Ordered, Label(tests.OpenshiftLabel, "monitoring"), func() {
		const query = `kubevirt_hco_misconfigured_descheduler`

		var (
			initialDescheduler = &deschedulerv1.KubeDescheduler{}
		)

		BeforeAll(func(ctx context.Context) {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			crdKey := client.ObjectKey{Name: hcoutil.DeschedulerCRDName}
			key := client.ObjectKey{Namespace: hcoutil.DeschedulerNamespace, Name: hcoutil.DeschedulerCRName}

			Eventually(func(g Gomega, ctx context.Context) {
				err := cli.Get(ctx, crdKey, crd)
				if apierrors.IsNotFound(err) {
					Skip("Skip test when KubeDescheduler CRD is not present")
				}
				g.Expect(err).NotTo(HaveOccurred())
				err = cli.Get(ctx, key, initialDescheduler)
				if apierrors.IsNotFound(err) {
					Skip("Skip test when KubeDescheduler CR is not present")
				}
				g.Expect(err).NotTo(HaveOccurred())
			}).WithTimeout(10 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())
		})

		AfterAll(func(ctx context.Context) {
			key := client.ObjectKey{Namespace: hcoutil.DeschedulerNamespace, Name: hcoutil.DeschedulerCRName}

			Eventually(func(g Gomega, ctx context.Context) {
				descheduler := &deschedulerv1.KubeDescheduler{}
				err := cli.Get(ctx, key, descheduler)
				g.Expect(err).NotTo(HaveOccurred())
				initialDescheduler.Spec.DeepCopyInto(&descheduler.Spec)
				descheduler.Annotations = initialDescheduler.Annotations
				err = cli.Update(ctx, descheduler)
				g.Expect(err).NotTo(HaveOccurred())
			}).WithTimeout(10 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			By("checking that the metric doesn't report it as misconfigured (0.0)")
			waitForMetricValue(ctx, promClient, query, float64(0))
		})

		It("HCOMisconfiguredDescheduler alert should fire when KubeDescheduler is installed and not properly configured for KubeVirt", Serial, func(ctx context.Context) {
			const (
				mergePatchMisconfigure = `{"metadata":{"annotations":{"platform.kubevirt.io/mode":"unmanaged"}},"spec":{"profiles":["AffinityAndTaints"]}}`
				mergePatchConfigure    = `{"spec":{"profiles":["DevKubeVirtRelieveAndMigrate"]}}`
			)

			By(fmt.Sprintf("Reading the `%s` metric from HCO prometheus endpoint", query))
			var valueBefore float64
			Eventually(func(g Gomega, ctx context.Context) {
				val, found, err := getMetricValue(ctx, promClient, query)
				g.Expect(err).NotTo(HaveOccurred())
				if !found {
					valueBefore = 0
				} else {
					valueBefore = val
				}
			}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(Succeed())

			GinkgoWriter.Printf("The metric value before the test is: %0.2f\n", valueBefore)

			patchMisconfigure := client.RawPatch(types.MergePatchType, []byte(mergePatchMisconfigure))
			patchConfigure := client.RawPatch(types.MergePatchType, []byte(mergePatchConfigure))

			descheduler := &deschedulerv1.KubeDescheduler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hcoutil.DeschedulerCRName,
					Namespace: hcoutil.DeschedulerNamespace,
				},
			}

			By("Misconfiguring the descheduler")
			Eventually(cli.Patch).
				WithContext(ctx).
				WithArguments(descheduler, patchMisconfigure).
				WithTimeout(10 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			By("checking that the prometheus metric reports it as misconfigured (1.0)")
			waitForMetricValue(ctx, promClient, query, float64(1))

			By("Checking the alert")
			Eventually(func(ctx context.Context) *promApiv1.Alert {
				alerts, err := promClient.Alerts(ctx)
				Expect(err).ToNot(HaveOccurred())
				alert := getAlertByName(alerts, hcoalerts.MisconfiguredDeschedulerAlert)
				return alert
			}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).ShouldNot(BeNil())

			By("Correctly configuring the descheduler for KubeVirt")
			Eventually(cli.Patch).
				WithContext(ctx).
				WithArguments(descheduler, patchConfigure).
				WithTimeout(10 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			By("checking that the metric doesn't report it as misconfigured (0.0)")
			waitForMetricValue(ctx, promClient, query, float64(0))

			By("Checking the alert is not firing")
			Eventually(func(ctx context.Context) *promApiv1.Alert {
				alerts, err := promClient.Alerts(ctx)
				Expect(err).ToNot(HaveOccurred())
				alert := getAlertByName(alerts, hcoalerts.MisconfiguredDeschedulerAlert)
				return alert
			}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).Should(BeNil())

			By("Misconfiguring a second time the descheduler")
			Eventually(cli.Patch).
				WithContext(ctx).
				WithArguments(descheduler, patchMisconfigure).
				WithTimeout(10 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			By("checking that the prometheus metric reports it as misconfigured (1.0)")
			waitForMetricValue(ctx, promClient, query, float64(1))

			By("Checking the alert")
			Eventually(func(ctx context.Context) *promApiv1.Alert {
				alerts, err := promClient.Alerts(ctx)
				Expect(err).ToNot(HaveOccurred())
				alert := getAlertByName(alerts, hcoalerts.MisconfiguredDeschedulerAlert)
				return alert
			}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).ShouldNot(BeNil())
		})
	})

	Describe("HCOOperatorConditionsUnhealthy", Serial, Ordered, Label(tests.OpenshiftLabel, "monitoring"), func() {
		const alertName = "HCOOperatorConditionsUnhealthy"

		BeforeAll(func(ctx context.Context) {
			By("Reducing alert pending time for HCOOperatorConditionsUnhealthy")
			pr := &monitoringv1.PrometheusRule{}
			Expect(cli.Get(ctx, client.ObjectKey{
				Namespace: tests.InstallNamespace,
				Name:      "kubevirt-hyperconverged-prometheus-rule",
			}, pr)).To(Succeed())

			originalSpec := pr.Spec.DeepCopy()

			DeferCleanup(func(ctx context.Context) {
				By("Restoring original HCO PrometheusRule")
				current := &monitoringv1.PrometheusRule{}
				Expect(cli.Get(ctx, client.ObjectKey{
					Namespace: tests.InstallNamespace,
					Name:      "kubevirt-hyperconverged-prometheus-rule",
				}, current)).To(Succeed())
				current.Spec = *originalSpec
				Expect(cli.Update(ctx, current)).To(Succeed())
			})

			for gi, group := range pr.Spec.Groups {
				for ri, rule := range group.Rules {
					if rule.Alert == alertName && rule.For != nil {
						pr.Spec.Groups[gi].Rules[ri].For = new(monitoringv1.Duration("0m"))
					}
				}
			}
			Expect(cli.Update(ctx, pr)).To(Succeed())
		})

		It("should fire when system health status is degraded", func(ctx context.Context) {
			By("Scaling down virt-operator to prevent it from overwriting KubeVirt status")
			var originalReplicas int32

			dep, err := cliSet.AppsV1().Deployments(tests.InstallNamespace).Get(ctx, "virt-operator", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			originalReplicas = *dep.Spec.Replicas

			DeferCleanup(func(ctx context.Context) {
				By("Restoring virt-operator replicas")
				scale, err := cliSet.AppsV1().Deployments(tests.InstallNamespace).GetScale(ctx, "virt-operator", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				scale.Spec.Replicas = originalReplicas
				_, err = cliSet.AppsV1().Deployments(tests.InstallNamespace).UpdateScale(ctx, "virt-operator", scale, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for system health to return to healthy")
				Eventually(func(ctx context.Context) *promApiv1.Alert {
					alerts, err := promClient.Alerts(ctx)
					Expect(err).ToNot(HaveOccurred())
					return getAlertByName(alerts, alertName)
				}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).Should(BeNil())
			})

			scale, err := cliSet.AppsV1().Deployments(tests.InstallNamespace).GetScale(ctx, "virt-operator", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			scale.Spec.Replicas = 0
			_, err = cliSet.AppsV1().Deployments(tests.InstallNamespace).UpdateScale(ctx, "virt-operator", scale, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for virt-operator pods to terminate")
			Eventually(func(ctx context.Context) int32 {
				dep, err := cliSet.AppsV1().Deployments(tests.InstallNamespace).Get(ctx, "virt-operator", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return dep.Status.ReadyReplicas
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(BeZero())

			By("Patching KubeVirt CR status to set Degraded=True")
			kvStatusPatch := []byte(`{"status":{"conditions":[{"type":"Degraded","status":"True","reason":"TestDegradation","message":"Degraded by e2e test","lastTransitionTime":"` + time.Now().UTC().Format(time.RFC3339) + `"}]}}`)

			kv := &kubevirtcorev1.KubeVirt{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubevirt-kubevirt-hyperconverged",
					Namespace: tests.InstallNamespace,
				},
			}
			Eventually(func(ctx context.Context) error {
				return cli.Status().Patch(ctx, kv, client.RawPatch(types.MergePatchType, kvStatusPatch))
			}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).WithContext(ctx).Should(Succeed())

			By("Checking that the prometheus metric reports unhealthy status")
			Eventually(func(g Gomega, ctx context.Context) float64 {
				val, found, err := getMetricValue(ctx, promClient, "kubevirt_hco_system_health_status")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				return val
			}).
				WithTimeout(5*time.Minute).
				WithPolling(10*time.Second).
				WithContext(ctx).
				Should(Or(Equal(float64(1)), Equal(float64(2))), "expected system health status to be warning (1) or error (2)")

			By("Checking that the alert fires")
			Eventually(func(ctx context.Context) *promApiv1.Alert {
				alerts, err := promClient.Alerts(ctx)
				Expect(err).ToNot(HaveOccurred())
				return getAlertByName(alerts, alertName)
			}).WithTimeout(prometheousTimeout).WithPolling(prometheousPolling).WithContext(ctx).ShouldNot(BeNil())
		})
	})

	Describe("Webhook monitoring", Label(tests.OpenshiftLabel, "monitoring", "webhook"), func() {
		const justARandomMetric = `controller_runtime_webhook_latency_seconds_sum{namespace="%s",container="hyperconverged-cluster-webhook",webhook="/validate-hco-kubevirt-io-v1-hyperconverged"}`

		It("should allow reading from the webhook metrics endpoint", func(ctx context.Context) {
			query := fmt.Sprintf(justARandomMetric, tests.InstallNamespace)
			Eventually(func(g Gomega, ctx context.Context) {
				vals, found, err := getMetricValues(ctx, promClient, query)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(vals).ToNot(BeEmpty())
				g.Expect(vals).To(ContainElement(BeNumerically(">", float64(0))))
			}).WithContext(ctx).
				WithPolling(5 * time.Second).
				WithTimeout(5 * time.Minute).
				Should(Succeed())
		})
	})
})

func getAlertByName(alerts promApiv1.AlertsResult, alertName string) *promApiv1.Alert {
	for _, alert := range alerts.Alerts {
		if string(alert.Labels["alertname"]) == alertName {
			return &alert
		}
	}
	return nil
}

func waitForMetricValue(ctx context.Context, promClient promApiv1.API, query string, expected float64) {
	GinkgoHelper()
	Eventually(func(g Gomega, ctx context.Context) {
		queryResult, _, err := promClient.Query(ctx, query, time.Now())
		g.Expect(err).NotTo(HaveOccurred())
		vector, ok := queryResult.(promModel.Vector)
		g.Expect(ok).To(BeTrue(), "expected a Vector result for the %q query", query)
		g.Expect(vector).To(HaveLen(1), "the %q query returned no sample; a missing series is not a zero value", query)
		val := float64(vector[0].Value)
		g.Expect(val).To(Equal(expected), "the %q query should eventually return %0.2f", query, expected)
	}).
		WithTimeout(5 * time.Minute).
		WithPolling(10 * time.Second).
		WithContext(ctx).
		Should(Succeed())
}

func getMetricValue(ctx context.Context, promClient promApiv1.API, metricName string) (val float64, found bool, err error) {
	queryResult, _, err := promClient.Query(ctx, metricName, time.Now())
	if err != nil {
		return 0, false, err
	}

	switch res := queryResult.(type) {
	case promModel.Vector:
		if len(res) == 0 {
			return 0, false, nil
		}

		if len(res) != 1 {
			return 0, true, fmt.Errorf("expected 1 vector result, got %d; result: %v", len(res), res)
		}

		return float64(res[0].Value), true, nil

	default:
		return 0, false, fmt.Errorf("expected a Vector result, got %T", queryResult)
	}
}

func getMetricValues(ctx context.Context, promClient promApiv1.API, metricName string) (vals []float64, found bool, err error) {
	queryResult, _, err := promClient.Query(ctx, metricName, time.Now())
	if err != nil {
		return nil, false, err
	}

	switch res := queryResult.(type) {
	case promModel.Vector:
		if len(res) == 0 {
			return nil, false, nil
		}

		vals = make([]float64, len(res))
		for i, v := range res {
			vals[i] = float64(v.Value)
		}

		return vals, true, nil

	default:
		return nil, false, fmt.Errorf("expected a Vector result, got %T", queryResult)
	}
}

func getPrometheusRule(ctx context.Context, cli rest.Interface) monitoringv1.PrometheusRule {
	GinkgoHelper()
	var prometheusRule monitoringv1.PrometheusRule

	Expect(cli.Get().
		Resource("prometheusrules").
		Name("kubevirt-hyperconverged-prometheus-rule").
		Namespace(tests.InstallNamespace).
		AbsPath("/apis", monitoringv1.SchemeGroupVersion.Group, monitoringv1.SchemeGroupVersion.Version).
		Timeout(10 * time.Second).
		Do(ctx).Into(&prometheusRule)).To(Succeed())
	return prometheusRule
}

func checkRunbookURLAvailability(rule monitoringv1.Rule) {
	resp, err := runbookClient.Head(rule.Annotations["runbook_url"])
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), fmt.Sprintf("%s runbook is not available", rule.Alert))
	ExpectWithOffset(1, resp.StatusCode).To(Equal(http.StatusOK), fmt.Sprintf("%s runbook is not available", rule.Alert))
}

func initializePromClient(prometheusURL string, token string) promApiv1.API {
	defaultRoundTripper := promApi.DefaultRoundTripper
	tripper := defaultRoundTripper.(*http.Transport)
	tripper.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	c, err := promApi.NewClient(promApi.Config{
		Address:      prometheusURL,
		RoundTripper: promConfig.NewAuthorizationCredentialsRoundTripper("Bearer", promConfig.NewInlineSecret(token), defaultRoundTripper),
	})

	Expect(err).ToNot(HaveOccurred())

	promClient := promApiv1.NewAPI(c)
	return promClient
}

func getAuthorizationTokenForPrometheus(ctx context.Context, cli *kubernetes.Clientset) string {
	var token string
	Eventually(func(ctx context.Context) bool {
		treq, err := cli.CoreV1().ServiceAccounts("openshift-monitoring").CreateToken(
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
			return false
		}
		token = treq.Status.Token
		return true
	}).WithTimeout(10 * time.Second).WithPolling(time.Second).WithContext(ctx).Should(BeTrue())
	return token
}

func getPrometheusURL(ctx context.Context, cli rest.Interface) string {
	s := scheme.Scheme
	_ = openshiftroutev1.Install(s)
	s.AddKnownTypes(openshiftroutev1.GroupVersion)

	var route openshiftroutev1.Route

	Eventually(func(ctx context.Context) error {
		return cli.Get().
			Resource("routes").
			Name("prometheus-k8s").
			Namespace("openshift-monitoring").
			AbsPath("/apis", openshiftroutev1.GroupVersion.Group, openshiftroutev1.GroupVersion.Version).
			Timeout(10 * time.Second).
			Do(ctx).Into(&route)
	}).WithTimeout(2 * time.Minute).
		WithPolling(15 * time.Second). // longer than the request timeout
		WithContext(ctx).
		Should(Succeed())

	return fmt.Sprintf("https://%s", route.Spec.Host)
}

func getAlertNames(alert promApiv1.AlertsResult) string {
	return "[" + strings.Join(slices.Collect(
		alertToAlertName(
			slices.Values(alert.Alerts),
		),
	), ", ") + "]"
}

func alertToAlertName(alerts iter.Seq[promApiv1.Alert]) iter.Seq[string] {
	return func(yield func(string) bool) {
		for alert := range alerts {
			if !yield(string(alert.Labels["alertname"])) {
				return
			}
		}
	}
}
