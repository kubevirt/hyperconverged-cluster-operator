package collectors

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
)

var _ = Describe("HyperConverged Collectors", func() {
	var hco *hcov1.HyperConverged

	BeforeEach(func() {
		hco = commontestutils.NewHco()

		origNodeInfoFunc := nodeinfo.GetWorkloadsArchitectures

		DeferCleanup(func() {
			nodeinfo.GetWorkloadsArchitectures = origNodeInfoFunc
		})
	})

	Describe("kubevirt_hco_multi_arch_boot_images_enabled", func() {
		When("cluster is multi architectures", func() {
			BeforeEach(func() {
				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"arch1", "arch2"}
				}
			})

			When("we deploy DICTs", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = []hcov1.DataImportCronTemplateStatus{
						{
							DataImportCronTemplate: hcov1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image1",
								},
							},
						},
						{
							DataImportCronTemplate: hcov1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image2",
								},
							},
						},
					}
				})

				It("should be set and enabled, if multi-arch dict enabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeTrue())
					Expect(isEnabled).To(BeTrue())
				})

				It("should be set and disabled, if multi-arch dict disabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeTrue())
					Expect(isEnabled).To(BeFalse())
				})

				It("should be set and disabled, if multi-arch dict is not set", func() {
					hco.Spec.FeatureGates = featuregates.HyperConvergedFeatureGates{}

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeTrue())
					Expect(isEnabled).To(BeFalse())
				})
			})

			When("we don't deploy DICTS", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = nil
				})

				It("should not be set, if multi-arch dict enabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict disabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})
			})
		})

		When("cluster is with single architecture", func() {
			BeforeEach(func() {
				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"single-arch"}
				}
			})

			When("we deploy DICTs", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = []hcov1.DataImportCronTemplateStatus{
						{
							DataImportCronTemplate: hcov1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image1",
								},
							},
						},
						{
							DataImportCronTemplate: hcov1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image2",
								},
							},
						},
					}
				})

				It("should not be set, if multi-arch dict enabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict disabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict is not set", func() {
					hco.Spec.FeatureGates = featuregates.HyperConvergedFeatureGates{}

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})
			})

			When("we don't deploy DICTs", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = nil
				})

				It("should not be set, if multi-arch dict enabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict disabled", func() {
					hco.Spec.WorkloadSources.EnableMultiArchBootImageImport = new(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})
			})
		})
	})
})

func isMultiArchBootImagesFeatureEnabled(cli client.Client) (isSet, isEnabled bool) {
	callback := getMultiArchBootImagesStatusCallback(cli, commontestutils.Namespace)

	res := callback()
	if len(res) == 0 {
		return false, false
	}

	isEnabled = res[0].Value == multiArchBootImagesFeatureEnabled

	return true, isEnabled
}

var _ = Describe("kubevirt_hco_feature_gate_enabled", func() {
	var hco *hcov1.HyperConverged

	BeforeEach(func() {
		hco = commontestutils.NewHco()
	})

	It("emits a 0 or 1 series for every known configurable feature gate", func() {
		cli := commontestutils.InitClient([]client.Object{hco})
		results := getFeatureGateEnabledCallback(cli, commontestutils.Namespace)()
		configurable := featuregatedetails.ListConfigurableFeatureGates()

		Expect(results).To(HaveLen(len(configurable)))
		for _, fg := range configurable {
			sample := featureGateSample(results, fg.Name)
			Expect(sample.Labels).To(Equal([]string{fg.Name, fg.Phase.String()}))
			expected := featureGateDisabledValue
			if hco.Spec.FeatureGates.IsEnabled(fg.Name) {
				expected = featureGateEnabledValue
			}
			Expect(sample.Value).To(Equal(expected), "unexpected value for feature gate %s", fg.Name)
		}
	})

	It("reports 1 for an explicitly enabled alpha gate and 0 for a disabled beta gate", func() {
		hco.Spec.FeatureGates.Enable("alignCPUs")
		hco.Spec.FeatureGates.Disable("decentralizedLiveMigration")

		cli := commontestutils.InitClient([]client.Object{hco})
		results := getFeatureGateEnabledCallback(cli, commontestutils.Namespace)()

		Expect(featureGateSample(results, "alignCPUs").Value).To(Equal(featureGateEnabledValue))
		Expect(featureGateSample(results, "alignCPUs").Labels).To(Equal([]string{"alignCPUs", "alpha"}))
		Expect(featureGateSample(results, "decentralizedLiveMigration").Value).To(Equal(featureGateDisabledValue))
		Expect(featureGateSample(results, "downwardMetrics").Value).To(Equal(featureGateDisabledValue))
	})

	It("emits 0 for every known configurable gate when the HyperConverged CR is missing", func() {
		cli := commontestutils.InitClient([]client.Object{})
		results := getFeatureGateEnabledCallback(cli, commontestutils.Namespace)()
		configurable := featuregatedetails.ListConfigurableFeatureGates()

		Expect(results).To(HaveLen(len(configurable)))
		for _, result := range results {
			Expect(result.Value).To(Equal(featureGateDisabledValue))
		}
	})

	It("emits no series when reading the HyperConverged CR fails", func() {
		cli := commontestutils.InitClient([]client.Object{hco})
		cli.InitiateGetErrors(func(_ client.ObjectKey) error {
			return fmt.Errorf("get failed")
		})

		results := getFeatureGateEnabledCallback(cli, commontestutils.Namespace)()
		Expect(results).To(BeEmpty())
	})

	It("lists the collector metrics used by docs and the linter", func() {
		Expect(collectorMetricNames(CollectorMetrics())).To(Equal([]string{
			"kubevirt_hco_multi_arch_boot_images_enabled",
			"kubevirt_hco_feature_gate_enabled",
		}))
	})

	It("registers the collector so scrapes include every configurable gate", func() {
		hco.Spec.FeatureGates.Enable("alignCPUs")
		cli := commontestutils.InitClient([]client.Object{hco})

		Expect(operatormetrics.CleanRegistry()).To(Succeed())
		DeferCleanup(func() {
			Expect(operatormetrics.CleanRegistry()).To(Succeed())
		})
		Expect(SetupCollectors(cli, commontestutils.Namespace)).To(Succeed())

		Expect(collectorMetricNames(operatormetrics.ListMetrics())).To(
			ContainElement("kubevirt_hco_feature_gate_enabled"),
		)

		family := gatheredFeatureGateFamily()
		Expect(family.GetHelp()).To(Equal(featureGateEnabled.GetOpts().Help))
		Expect(family.GetType()).To(Equal(dto.MetricType_GAUGE))

		samples := family.GetMetric()
		configurable := featuregatedetails.ListConfigurableFeatureGates()
		Expect(samples).To(HaveLen(len(configurable)))

		byName := map[string]*dto.Metric{}
		for _, sample := range samples {
			name := prometheusLabel(sample, featureGateLabelName)
			Expect(name).NotTo(BeEmpty())
			byName[name] = sample
		}

		for _, fg := range configurable {
			sample, ok := byName[fg.Name]
			Expect(ok).To(BeTrue(), "missing gathered series for feature gate %s", fg.Name)
			Expect(prometheusLabel(sample, featureGateLabelPhase)).To(Equal(fg.Phase.String()))
			expected := featureGateDisabledValue
			if hco.Spec.FeatureGates.IsEnabled(fg.Name) {
				expected = featureGateEnabledValue
			}
			Expect(sample.GetGauge().GetValue()).To(Equal(expected), "unexpected value for feature gate %s", fg.Name)
		}
	})
})

func featureGateSample(results []operatormetrics.CollectorResult, name string) operatormetrics.CollectorResult {
	for _, result := range results {
		if len(result.Labels) > 0 && result.Labels[0] == name {
			return result
		}
	}

	Fail(fmt.Sprintf("missing series for feature gate %s", name))
	return operatormetrics.CollectorResult{}
}

func collectorMetricNames(metrics []operatormetrics.Metric) []string {
	names := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		names = append(names, metric.GetOpts().Name)
	}
	return names
}

func gatheredFeatureGateFamily() *dto.MetricFamily {
	families, err := prometheus.DefaultGatherer.Gather()
	Expect(err).ToNot(HaveOccurred())
	for _, family := range families {
		if family.GetName() == "kubevirt_hco_feature_gate_enabled" {
			return family
		}
	}

	Fail("kubevirt_hco_feature_gate_enabled was not gathered after registration")
	return nil
}

func prometheusLabel(sample *dto.Metric, name string) string {
	for _, label := range sample.GetLabel() {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}
