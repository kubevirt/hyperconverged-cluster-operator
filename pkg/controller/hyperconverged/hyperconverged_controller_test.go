package hyperconverged

import (
	networkaddons "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1alpha1"
	networkaddonsnames "github.com/kubevirt/cluster-network-addons-operator/pkg/names"
	hcov1alpha1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HyperconvergedController", func() {
	Describe("CR creation functions", func() {
		instance := &hcov1alpha1.HyperConverged{}
		instance.Name = "hyperconverged-cluster"
		appLabel := map[string]string{
			"app": instance.Name,
		}

		Context("KubeVirt Config CR", func() {
			It("should have metadata", func() {
				cr := newKubeVirtConfigForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal("kubevirt-config"))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
			})
		})

		Context("KubeVirt CR", func() {
			It("should have metadata", func() {
				cr := newKubeVirtForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal("kubevirt-" + instance.Name))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
			})
		})

		Context("CDI CR", func() {
			It("should have metadata", func() {
				cr := newCDIForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal("cdi-" + instance.Name))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
			})
		})

		Context("Network Addons CR", func() {
			It("should have metadata and spec", func() {
				cr := newNetworkAddonsForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal(networkaddonsnames.OPERATOR_CONFIG))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
				Expect(cr.Spec.Multus).To(Equal(&networkaddons.Multus{}))
				Expect(cr.Spec.LinuxBridge).To(Equal(&networkaddons.LinuxBridge{}))
				Expect(cr.Spec.KubeMacPool).To(Equal(&networkaddons.KubeMacPool{}))
			})
		})

		Context("KubeVirt Common Template Bundle CR", func() {
			It("should have metadata", func() {
				cr := newKubevirtCommonTemplateBundleForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal("common-templates-" + instance.Name))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
				Expect(cr.ObjectMeta.Namespace).To(Equal("openshift"))
			})
		})

		Context("KubeVirt Node Labeller Bundle CR", func() {
			It("should have metadata", func() {
				cr := newKubevirtNodeLabellerBundleForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal("node-labeller-" + instance.Name))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
			})
		})

		Context("KubeVirt Template Validator CR", func() {
			It("should have metadata", func() {
				cr := newKubevirtTemplateValidatorForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal("template-validator-" + instance.Name))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
			})
		})

		Context("KubeVirt Web UI CR", func() {
			It("should have metadata and spec", func() {
				cr := newKWebUIForCR(instance)
				Expect(cr.ObjectMeta.Name).To(Equal("kubevirt-web-ui-" + instance.Name))
				Expect(cr.ObjectMeta.Labels).To(Equal(appLabel))
				Expect(cr.Spec.OpenshiftMasterDefaultSubdomain).To(Equal(instance.Spec.KWebUIMasterDefaultSubdomain))
				Expect(cr.Spec.PublicMasterHostname).To(Equal(instance.Spec.KWebUIPublicMasterHostname))
				Expect(cr.Spec.Version).To(Equal("automatic"))
			})
		})
	})
})
