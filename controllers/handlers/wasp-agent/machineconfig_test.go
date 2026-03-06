package wasp_agent

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mcv1 "github.com/openshift/api/machineconfiguration/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

// ignitionConfig represents the Ignition config structure embedded in the MachineConfig spec.
type ignitionConfig struct {
	Ignition struct {
		Version string `json:"version"`
	} `json:"ignition"`
	Systemd struct {
		Units []ignitionSystemdUnit `json:"units"`
	} `json:"systemd"`
}

type ignitionSystemdUnit struct {
	Name     string `json:"name"`
	Enabled  *bool  `json:"enabled,omitempty"`
	Contents string `json:"contents,omitempty"`
}

func parseIgnitionConfig(mc *mcv1.MachineConfig) (*ignitionConfig, error) {
	ic := &ignitionConfig{}
	if err := json.Unmarshal(mc.Spec.Config.Raw, ic); err != nil {
		return nil, err
	}
	return ic, nil
}

var _ = Describe("Wasp Agent MachineConfig", func() {
	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("Wasp MachineConfig deployment", func() {
		It("should not create MachineConfig when EnableOpenShiftSwap feature gate is disabled", func() {
			enableOpenShiftSwap := false
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &enableOpenShiftSwap
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundMC := &mcv1.MachineConfigList{}
			Expect(cl.List(context.Background(), foundMC)).To(Succeed())
			Expect(foundMC.Items).To(BeEmpty())
		})

		It("should not create MachineConfig when EnableOpenShiftSwap feature gate is nil", func() {
			hco.Spec.FeatureGates.EnableOpenShiftSwap = nil
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundMC := &mcv1.MachineConfigList{}
			Expect(cl.List(context.Background(), foundMC)).To(Succeed())
			Expect(foundMC.Items).To(BeEmpty())
		})

		It("should delete MachineConfig when EnableOpenShiftSwap feature gate is disabled", func() {
			enableOpenShiftSwap := false
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &enableOpenShiftSwap
			mc := newWaspAgentMachineConfig()
			cl = commontestutils.InitClient([]client.Object{hco, mc})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(mc.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundMC := &mcv1.MachineConfigList{}
			Expect(cl.List(context.Background(), foundMC)).To(Succeed())
			Expect(foundMC.Items).To(BeEmpty())
		})

		It("should create MachineConfig when EnableOpenShiftSwap feature gate is enabled", func() {
			enableOpenShiftSwap := true
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &enableOpenShiftSwap
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(machineConfigName))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundMC := &mcv1.MachineConfigList{}
			Expect(cl.List(context.Background(), foundMC)).To(Succeed())
			Expect(foundMC.Items).To(HaveLen(1))
			Expect(foundMC.Items[0].Name).To(Equal(machineConfigName))
		})

		It("should create MachineConfig with correct spec from embedded YAML", func() {
			enableOpenShiftSwap := true
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &enableOpenShiftSwap
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundMC := &mcv1.MachineConfig{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Name: machineConfigName}, foundMC)).To(Succeed())

			// Verify it has the expected spec from the YAML
			Expect(foundMC.Spec.Config).ToNot(BeNil())
			// Verify labels from YAML are present
			Expect(foundMC.Labels).To(HaveKey("machineconfiguration.openshift.io/role"))
			Expect(foundMC.Labels["machineconfiguration.openshift.io/role"]).To(Equal("worker"))
		})
	})

	Context("Wasp agent MachineConfig update", func() {
		It("should update MachineConfig spec if modified", func() {
			enableOpenShiftSwap := true
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &enableOpenShiftSwap
			originalMC := newWaspAgentMachineConfig()
			modifiedMC := originalMC.DeepCopy()
			// Modify the spec
			modifiedMC.Spec.OSImageURL = "modified-url"
			cl = commontestutils.InitClient([]client.Object{hco, modifiedMC})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			reconciledMC := &mcv1.MachineConfig{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Name: machineConfigName}, reconciledMC)).To(Succeed())

			// Should be reconciled back to the original spec
			Expect(reconciledMC.Spec.OSImageURL).To(Equal(originalMC.Spec.OSImageURL))
			Expect(reconciledMC.Spec).To(Equal(originalMC.Spec))
		})

		It("should reconcile labels if modified", func() {
			enableOpenShiftSwap := true
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &enableOpenShiftSwap
			mc := newWaspAgentMachineConfig()
			// Modify a label
			mc.Labels["app.kubernetes.io/managed-by"] = "wrong-value"
			mc.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, mc})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundMC := &mcv1.MachineConfig{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Name: machineConfigName}, foundMC)).To(Succeed())

			// Check user label is preserved
			Expect(foundMC.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})

		It("should not update if MachineConfig spec is already correct", func() {
			enableOpenShiftSwap := true
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &enableOpenShiftSwap

			mc := newWaspAgentMachineConfig()
			cl = commontestutils.InitClient([]client.Object{hco, mc})

			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())
		})
	})

	Context("MachineConfig helper functions", func() {
		It("NewWaspAgentMachineConfigWithNameOnly should create minimal MachineConfig", func() {
			mc := NewWaspAgentMachineConfigWithNameOnly()

			Expect(mc).ToNot(BeNil())
			Expect(mc.Name).To(Equal(machineConfigName))
			Expect(mc.Labels).To(HaveKey("app"))
			Expect(mc.Labels).To(HaveKey("app.kubernetes.io/managed-by"))
			Expect(mc.Labels).To(HaveKey("app.kubernetes.io/component"))
			Expect(mc.Labels["app.kubernetes.io/component"]).To(Equal(AppComponentWaspAgent))
			Expect(mc.Labels).To(HaveKeyWithValue("machineconfiguration.openshift.io/role", "worker"))
			Expect(mc.Spec.Config.Raw).To(BeNil())
		})

		It("newWaspAgentMachineConfig should create MachineConfig with proper spec", func() {
			mc := newWaspAgentMachineConfig()

			Expect(mc).ToNot(BeNil())
			Expect(mc.Name).To(Equal(machineConfigName))
			Expect(mc.Spec.Config).ToNot(BeNil())
			Expect(mc.Labels).To(HaveKey("app.kubernetes.io/component"))
			Expect(mc.Labels).To(HaveKey("app"))
			Expect(mc.Labels).To(HaveKeyWithValue("machineconfiguration.openshift.io/role", "worker"))
		})

		It("newWaspAgentMachineConfig with nil HyperConverged should not panic", func() {
			Expect(func() {
				mc := newWaspAgentMachineConfig()
				Expect(mc).ToNot(BeNil())
				Expect(mc.Name).To(Equal(machineConfigName))
			}).ToNot(Panic())
		})

		It("getMachineConfig should successfully unmarshal embedded YAML", func() {
			mc, err := getMachineConfig()

			Expect(err).ToNot(HaveOccurred())
			Expect(mc).ToNot(BeNil())
			Expect(mc.Kind).To(Equal("MachineConfig"))
			Expect(mc.APIVersion).To(Equal("machineconfiguration.openshift.io/v1"))
			Expect(mc.Spec.Config).ToNot(BeNil())
		})

		It("getMachineConfig should load swap configuration from YAML", func() {
			mc, err := getMachineConfig()

			Expect(err).ToNot(HaveOccurred())
			Expect(mc.ObjectMeta.Name).To(Equal(machineConfigName))

			Expect(mc.Labels).To(HaveKey("machineconfiguration.openshift.io/role"))
			Expect(mc.Labels["machineconfiguration.openshift.io/role"]).To(Equal("worker"))

			Expect(mc.Spec.Config).ToNot(BeNil())
		})
	})

	Context("MachineConfig embedded YAML content integrity", func() {
		var (
			mc *mcv1.MachineConfig
			ic *ignitionConfig
		)

		BeforeEach(func() {
			var err error
			mc, err = getMachineConfig()
			Expect(err).ToNot(HaveOccurred())
			Expect(mc).ToNot(BeNil())
			Expect(mc.Spec.Config.Raw).ToNot(BeEmpty())

			ic, err = parseIgnitionConfig(mc)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have the correct ignition version", func() {
			Expect(ic.Ignition.Version).To(Equal("3.5.0"))
		})

		It("should contain exactly the expected systemd units", func() {
			expectedUnitNames := []string{
				"swap-disk-enable.service",
				"ocpswap-file-enable.service",
			}

			unitNames := make([]string, len(ic.Systemd.Units))
			for i, u := range ic.Systemd.Units {
				unitNames[i] = u.Name
			}

			Expect(unitNames).To(ConsistOf(expectedUnitNames))
		})

		It("should have swap-disk-enable.service with correct configuration", func() {
			unit := findSystemdUnit(ic, "swap-disk-enable.service")
			Expect(unit).ToNot(BeNil(), "swap-disk-enable.service should exist")
			Expect(unit.Enabled).ToNot(BeNil())
			Expect(*unit.Enabled).To(BeTrue())
			Expect(unit.Contents).To(ContainSubstring("Description=Enable swap"))
			Expect(unit.Contents).To(ContainSubstring("ConditionPathExists=/dev/disk/by-partlabel/OCPSWAP"))
			Expect(unit.Contents).To(ContainSubstring("swapon --priority 100 /dev/disk/by-partlabel/OCPSWAP"))
			Expect(unit.Contents).To(ContainSubstring("RequiredBy=kubelet-dependencies.target"))
		})

		It("should have ocpswap-file-enable.service with correct configuration", func() {
			unit := findSystemdUnit(ic, "ocpswap-file-enable.service")
			Expect(unit).ToNot(BeNil(), "ocpswap-file-enable.service should exist")
			Expect(unit.Enabled).ToNot(BeNil())
			Expect(*unit.Enabled).To(BeTrue())
			Expect(unit.Contents).To(ContainSubstring("Description=Enable OCP file swap"))
			Expect(unit.Contents).To(ContainSubstring("ConditionPathExists=/var/tmp/ocpswap.file"))
			Expect(unit.Contents).To(ContainSubstring("swapon --priority 10 /var/tmp/ocpswap.file"))
			Expect(unit.Contents).To(ContainSubstring("RequiredBy=kubelet-dependencies.target"))
		})
	})

	Context("shouldDeployOpenShiftSwap condition", func() {
		It("should return false when EnableOpenShiftSwap is nil", func() {
			hco.Spec.FeatureGates.EnableOpenShiftSwap = nil
			Expect(shouldDeployOpenShiftSwap(hco)).To(BeFalse())
		})

		It("should return false when EnableOpenShiftSwap is false", func() {
			falseValue := false
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &falseValue
			Expect(shouldDeployOpenShiftSwap(hco)).To(BeFalse())
		})

		It("should return true when EnableOpenShiftSwap is true", func() {
			trueValue := true
			hco.Spec.FeatureGates.EnableOpenShiftSwap = &trueValue
			Expect(shouldDeployOpenShiftSwap(hco)).To(BeTrue())
		})
	})

	Context("MachineConfig name constant", func() {
		It("should have the correct name", func() {
			Expect(machineConfigName).To(Equal("90-worker-swap-online"))
		})

		It("should match the name in the embedded YAML", func() {
			mc, err := getMachineConfig()
			Expect(err).ToNot(HaveOccurred())
			Expect(mc.Name).To(Equal(machineConfigName))
		})
	})

	Context("Integration with operands package", func() {
		It("should use correct static labels", func() {
			mc := newWaspAgentMachineConfig()

			Expect(mc.Labels).To(HaveKey("app"))
			Expect(mc.Labels).To(HaveKey("app.kubernetes.io/managed-by"))
			Expect(mc.Labels).To(HaveKey("app.kubernetes.io/part-of"))
			Expect(mc.Labels).To(HaveKey("app.kubernetes.io/component"))
			Expect(mc.Labels).To(HaveKey("app.kubernetes.io/version"))
			Expect(mc.Labels).To(HaveKeyWithValue("machineconfiguration.openshift.io/role", "worker"))
		})

		It("should create handler that properly implements Operand interface", func() {
			cl = commontestutils.InitClient([]client.Object{hco})
			handler := NewWaspAgentMachineConfigHandler(cl, commontestutils.GetScheme())

			Expect(handler).ToNot(BeNil())
			Expect(func() {
				handler.Ensure(req)
			}).ToNot(Panic())
		})
	})
})

func findSystemdUnit(ic *ignitionConfig, name string) *ignitionSystemdUnit {
	for i := range ic.Systemd.Units {
		if ic.Systemd.Units[i].Name == name {
			return &ic.Systemd.Units[i]
		}
	}
	return nil
}
