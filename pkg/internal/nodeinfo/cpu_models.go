package nodeinfo

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"sync"

	kubevirtv1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

var (
	cpuModels     []hcov1.CPUModelStatus
	cpuModelsLock sync.RWMutex
)

func GetCPUModels() []hcov1.CPUModelStatus {
	cpuModelsLock.RLock()
	defer cpuModelsLock.RUnlock()

	return slices.Clone(cpuModels)
}

// ResetCPUModelsCache clears the stored CPU model list. Intended for tests.
func ResetCPUModelsCache() {
	cpuModelsLock.Lock()
	defer cpuModelsLock.Unlock()
	cpuModels = nil
}

// addNodeCPUModels records cpu-model labels from a schedulable node.
func addNodeCPUModels(labels map[string]string, seen map[string]struct{}) {
	if labels[kubevirtv1.NodeSchedulable] != "true" {
		return
	}
	for key, value := range labels {
		model, ok := strings.CutPrefix(key, kubevirtv1.CPUModelLabel)
		if value != "true" || !ok || model == "" {
			continue
		}
		seen[model] = struct{}{}
	}
}

// setCPUModels stores the sorted model list and reports whether it changed.
func setCPUModels(seen map[string]struct{}) bool {
	cpuModelsLock.Lock()
	defer cpuModelsLock.Unlock()

	var out []hcov1.CPUModelStatus
	if len(seen) > 0 {
		names := make([]string, 0, len(seen))
		for model := range seen {
			names = append(names, model)
		}
		sortCPUModelNames(names)
		out = make([]hcov1.CPUModelStatus, len(names))
		for i, model := range names {
			out[i] = hcov1.CPUModelStatus{Name: model}
		}
	}
	if slices.EqualFunc(cpuModels, out, func(a, b hcov1.CPUModelStatus) bool {
		return a.Name == b.Name
	}) {
		return false
	}
	cpuModels = out
	return true
}

func sortCPUModelNames(names []string) {
	slices.SortFunc(names, func(a, b string) int {
		af, av := parseCPUModel(a)
		bf, bv := parseCPUModel(b)
		ai, bi := slices.Index(cpuFamiliesNewestFirst, af), slices.Index(cpuFamiliesNewestFirst, bf)
		if ai < 0 {
			ai = len(cpuFamiliesNewestFirst)
		}
		if bi < 0 {
			bi = len(cpuFamiliesNewestFirst)
		}
		// Alias, then higher -vN, then reduced variants (-noTSX, -IBRS).
		return cmp.Or(
			cmp.Compare(ai, bi),
			cmp.Compare(cpuModelVariantRank(a, af, av), cpuModelVariantRank(b, bf, bv)),
			cmp.Compare(bv, av),
			strings.Compare(a, b),
		)
	})
}

// cpuModelVariantRank orders the unsuffixed alias ahead of -vN versions,
// and those ahead of reduced variants.
func cpuModelVariantRank(model, family string, version int) int {
	switch {
	case model == family:
		return 0
	case version > 0:
		return 1
	default:
		return 2
	}
}

// cpuFamiliesNewestFirst is the recommendation order, newest family first.
var cpuFamiliesNewestFirst = []string{
	// Intel server and client, newest first.
	"DiamondRapids",
	"KnightsMill",
	"Denverton",
	"GraniteRapids",
	"ClearwaterForest",
	"SierraForest",
	"SapphireRapids",
	"Snowridge",
	"Icelake-Server",
	"Icelake-Client",
	"Cooperlake",
	"Cascadelake-Server",
	"Skylake-Server",
	"Skylake-Client",
	"Broadwell",
	"Haswell",
	"IvyBridge",
	"SandyBridge",
	"Westmere",
	"Nehalem",
	"Penryn",
	"Conroe",

	// AMD, newest first.
	"EPYC-Turin",
	"EPYC-Genoa",
	"EPYC-Milan",
	"EPYC-Rome",
	"EPYC",
	"Opteron_G5",
	"Opteron_G4",
	"Opteron_G3",
	"Opteron_G2",
	"Opteron_G1",
	"phenom",
	"athlon",

	// Hygon CPU models
	"Dhyana",

	// Zhaoxin CentaurHauls CPU models
	"YongFeng",

	// ARM, newest server models first.
	"Neoverse-V2",
	"Neoverse-V1",
	"Neoverse-N2",
	"Neoverse-N1",
	"Ampere-1a",
	"Ampere-1",
	"Kunpeng-920",
	"a64fx",
	"Tengyun-S2500",
	"FT-2000plus",
	"cortex-a72",
	"cortex-a57",
	"cortex-a53",
	"Falkor",
	"ThunderX299xx",

	// POWER, newest first, then Freescale.
	"POWER11",
	"POWER10",
	"POWER9",
	"POWER8",
	"POWER7",
	"POWER6",
	"POWERPC_e6500",
	"POWERPC_e5500",

	// Generic QEMU models, then the baseline.
	"kvm64",
	"qemu64",
	"cpu64-rhel6",
	"cpu64-rhel5",
	"kvm32",
	"qemu32",
	"core2duo",
	"n270",
	"coreduo",
	"pentiumpro",
	"pentium3",
	"pentium2",
	"pentium",
	"486",
}

func parseCPUModel(model string) (family string, version int) {
	for _, name := range cpuFamiliesNewestFirst {
		if (model == name || strings.HasPrefix(model, name+"-")) && len(name) > len(family) {
			family = name
		}
	}
	if family == "" {
		return model, 0
	}
	if rest, ok := strings.CutPrefix(model, family+"-v"); ok {
		version, _ = strconv.Atoi(rest)
	}
	return family, version
}
