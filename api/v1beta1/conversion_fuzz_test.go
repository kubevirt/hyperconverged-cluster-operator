package v1beta1

import (
	"math/rand/v2"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubevirtv1 "kubevirt.io/api/core/v1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcofg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
)

// FuzzV1beta1ToV1RoundTrip verifies that converting a v1beta1 HyperConverged CR to v1 and back does not lose or
// corrupt data. A direct comparison of the v1beta1 original to the round-tripped v1beta1 is not possible because the
// two API versions have structural differences (e.g. deprecated fields, reorganized structs). Instead, we compare the
// canonical v1 form: we convert v1beta1 → v1 (first), then v1 → v1beta1 → v1 (second), and assert that the two v1
// specs are equal. If the round-trip is lossless, both v1 representations must be identical.
//
// The fuzz engine mutates the int64 seed, which drives a deterministic random generator to produce HyperConverged CRs
// with varying field combinations.
func FuzzV1beta1ToV1RoundTrip(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(42))
	f.Add(int64(12345))

	f.Fuzz(func(t *testing.T, seed int64) {
		g := NewWithT(t)
		r := rand.New(rand.NewPCG(uint64(seed), uint64(seed>>1)))

		// v1beta1 -> v1 (first)
		original := randomV1beta1HC(r)
		v1First := &hcov1.HyperConverged{}
		g.Expect(original.ConvertTo(v1First)).To(Succeed())

		// v1 -> v1beta1 (round-trip back)
		roundTripped := &HyperConverged{}
		g.Expect(roundTripped.ConvertFrom(v1First)).To(Succeed())

		// v1beta1 -> v1 (second)
		v1Second := &hcov1.HyperConverged{}
		g.Expect(roundTripped.ConvertTo(v1Second)).To(Succeed())

		// the two v1 representations should be equal
		g.Expect(v1Second.Spec).To(Equal(v1First.Spec))
	})
}

// FuzzV1ToV1beta1RoundTrip is the mirror of FuzzV1beta1ToV1RoundTrip: it starts from a random v1 HyperConverged CR
// and verifies that converting v1 → v1beta1 and back does not lose or corrupt data. We compare the canonical v1beta1
// form: v1 → v1beta1 (first), then v1beta1 → v1 → v1beta1 (second), and assert that the two v1beta1 specs are equal.
func FuzzV1ToV1beta1RoundTrip(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(42))
	f.Add(int64(12345))

	f.Fuzz(func(t *testing.T, seed int64) {
		g := NewWithT(t)
		r := rand.New(rand.NewPCG(uint64(seed), uint64(seed>>1)))

		// v1 -> v1beta1 (first)
		original := randomV1HC(r)
		v1beta1First := &HyperConverged{}
		g.Expect(v1beta1First.ConvertFrom(original)).To(Succeed())

		// v1beta1 -> v1 (round-trip back)
		roundTripped := &hcov1.HyperConverged{}
		g.Expect(v1beta1First.ConvertTo(roundTripped)).To(Succeed())

		// v1 -> v1beta1 (second)
		v1beta1Second := &HyperConverged{}
		g.Expect(v1beta1Second.ConvertFrom(roundTripped)).To(Succeed())

		// the two v1beta1 representations should be equal
		g.Expect(v1beta1Second.Spec).To(Equal(v1beta1First.Spec))
	})
}

// randomV1beta1HC creates a HyperConverged v1beta1 CR with randomly populated fields.
func randomV1beta1HC(r *rand.Rand) *HyperConverged {
	hc := &HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-hyperconverged",
			Namespace: "kubevirt-hyperconverged",
		},
		Spec: HyperConvergedSpec{
			LiveMigrationConfig: hcov1.LiveMigrationConfigurations{
				ParallelMigrationsPerCluster:      randPtr(r, r.Uint32()),
				ParallelOutboundMigrationsPerNode: randPtr(r, r.Uint32()),
				BandwidthPerMigration:             randPtr(r, randString(r)),
				CompletionTimeoutPerGiB:           randPtr(r, r.Int64()),
				ProgressTimeout:                   randPtr(r, r.Int64()),
				AllowAutoConverge:                 randPtr(r, r.IntN(2) == 1),
				AllowPostCopy:                     randPtr(r, r.IntN(2) == 1),
			},
			WorkloadUpdateStrategy: hcov1.HyperConvergedWorkloadUpdateStrategy{
				WorkloadUpdateMethods: randStringSlice(r),
				BatchEvictionSize:     randPtr(r, r.IntN(100)),
				BatchEvictionInterval: randPtr(r, metav1.Duration{Duration: 30000000000}),
			},
			EvictionStrategy: randPtr(r, kubevirtv1.EvictionStrategyLiveMigrate),
		},
	}

	if r.IntN(2) == 1 {
		hc.Spec.Infra = HyperConvergedConfig{
			NodePlacement: &sdkapi.NodePlacement{
				NodeSelector: map[string]string{randString(r): randString(r)},
			},
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.Workloads = HyperConvergedConfig{
			NodePlacement: &sdkapi.NodePlacement{
				NodeSelector: map[string]string{randString(r): randString(r)},
			},
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.PermittedHostDevices = &hcov1.PermittedHostDevices{
			PciHostDevices: []hcov1.PciHostDevice{
				{
					PCIDeviceSelector:        randString(r),
					ResourceName:             randString(r),
					ExternalResourceProvider: r.IntN(2) == 1,
					Disabled:                 r.IntN(2) == 1,
				},
			},
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.MediatedDevicesConfiguration = &MediatedDevicesConfiguration{
			MediatedDeviceTypes: randStringSlice(r),
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.ObsoleteCPUs = &HyperConvergedObsoleteCPUs{
			CPUModels: randStringSlice(r),
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.VirtualMachineOptions = &VirtualMachineOptions{
			DisableFreePageReporting: randPtr(r, r.IntN(2) == 1),
			DisableSerialConsoleLog:  randPtr(r, r.IntN(2) == 1),
		}
		hc.Spec.DefaultCPUModel = randPtr(r, randString(r))
		hc.Spec.DefaultRuntimeClass = randPtr(r, randString(r))
	}

	if r.IntN(2) == 1 {
		hc.Spec.HigherWorkloadDensity = &hcov1.HigherWorkloadDensityConfiguration{
			MemoryOvercommitPercentage: r.IntN(200) + 10,
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.LiveUpdateConfiguration = &kubevirtv1.LiveUpdateConfiguration{
			MaxHotplugRatio: r.Uint32(),
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.KSMConfiguration = &kubevirtv1.KSMConfiguration{
			NodeLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{randString(r): randString(r)},
			},
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.FeatureGates = HyperConvergedFeatureGates{
			DownwardMetrics: randPtr(r, r.IntN(2) == 1),
			AlignCPUs:       randPtr(r, r.IntN(2) == 1),
		}
	}

	return hc
}

// randomV1HC creates a HyperConverged v1 CR with randomly populated fields.
func randomV1HC(r *rand.Rand) *hcov1.HyperConverged {
	hc := &hcov1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-hyperconverged",
			Namespace: "kubevirt-hyperconverged",
		},
		Spec: hcov1.HyperConvergedSpec{
			Virtualization: hcov1.VirtualizationConfig{
				LiveMigrationConfig: hcov1.LiveMigrationConfigurations{
					ParallelMigrationsPerCluster:      randPtr(r, r.Uint32()),
					ParallelOutboundMigrationsPerNode: randPtr(r, r.Uint32()),
					BandwidthPerMigration:             randPtr(r, randString(r)),
					CompletionTimeoutPerGiB:           randPtr(r, r.Int64()),
					ProgressTimeout:                   randPtr(r, r.Int64()),
					AllowAutoConverge:                 randPtr(r, r.IntN(2) == 1),
					AllowPostCopy:                     randPtr(r, r.IntN(2) == 1),
				},
				WorkloadUpdateStrategy: hcov1.HyperConvergedWorkloadUpdateStrategy{
					WorkloadUpdateMethods: randStringSlice(r),
					BatchEvictionSize:     randPtr(r, r.IntN(100)),
					BatchEvictionInterval: randPtr(r, metav1.Duration{Duration: 30000000000}),
				},
				EvictionStrategy: randPtr(r, kubevirtv1.EvictionStrategyLiveMigrate),
			},
		},
	}

	if r.IntN(2) == 1 {
		hc.Spec.NodePlacements = &hcov1.NodePlacements{
			Infra: &sdkapi.NodePlacement{
				NodeSelector: map[string]string{randString(r): randString(r)},
			},
			Workload: &sdkapi.NodePlacement{
				NodeSelector: map[string]string{randString(r): randString(r)},
			},
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.Virtualization.PermittedHostDevices = &hcov1.PermittedHostDevices{
			PciHostDevices: []hcov1.PciHostDevice{
				{
					PCIDeviceSelector:        randString(r),
					ResourceName:             randString(r),
					ExternalResourceProvider: r.IntN(2) == 1,
					Disabled:                 r.IntN(2) == 1,
				},
			},
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.Virtualization.MediatedDevicesConfiguration = &hcov1.MediatedDevicesConfiguration{
			MediatedDeviceTypes: randStringSlice(r),
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.Virtualization.ObsoleteCPUModels = randStringSlice(r)
	}

	if r.IntN(2) == 1 {
		hc.Spec.Virtualization.VirtualMachineOptions = &hcov1.VirtualMachineOptions{
			DisableFreePageReporting: randPtr(r, r.IntN(2) == 1),
			DisableSerialConsoleLog:  randPtr(r, r.IntN(2) == 1),
			DefaultCPUModel:          randPtr(r, randString(r)),
			DefaultRuntimeClass:      randPtr(r, randString(r)),
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.Virtualization.HigherWorkloadDensity = &hcov1.HigherWorkloadDensityConfiguration{
			MemoryOvercommitPercentage: r.IntN(200) + 10,
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.Virtualization.LiveUpdateConfiguration = &kubevirtv1.LiveUpdateConfiguration{
			MaxHotplugRatio: r.Uint32(),
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.Virtualization.KSMConfiguration = &kubevirtv1.KSMConfiguration{
			NodeLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{randString(r): randString(r)},
			},
		}
	}

	if r.IntN(2) == 1 {
		hc.Spec.FeatureGates = hcofg.HyperConvergedFeatureGates{}
		if r.IntN(2) == 1 {
			hc.Spec.FeatureGates.Enable("downwardMetrics")
		}
		if r.IntN(2) == 1 {
			hc.Spec.FeatureGates.Disable("decentralizedLiveMigration")
		}
	}

	return hc
}

func randString(r *rand.Rand) string {
	const chars = "abcdefghijklmnopqrstuvwxyz"
	length := r.IntN(10) + 1
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[r.IntN(len(chars))]
	}
	return string(b)
}

func randStringSlice(r *rand.Rand) []string {
	n := r.IntN(4) + 1
	s := make([]string, n)
	for i := range s {
		s[i] = randString(r)
	}
	return s
}

func randPtr[T any](r *rand.Rand, value T) *T {
	if r.IntN(3) == 0 {
		return nil
	}
	return &value
}
