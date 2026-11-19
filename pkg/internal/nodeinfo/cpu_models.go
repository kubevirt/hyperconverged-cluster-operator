package nodeinfo

import (
	"cmp"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

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

	if cpuModels == nil {
		return nil
	}
	out := slices.Clone(cpuModels)
	for i := range out {
		out[i].UnsupportedNodes = slices.Clone(out[i].UnsupportedNodes)
	}
	return out
}

func setCPUModels(models []hcov1.CPUModelStatus) bool {
	cpuModelsLock.Lock()
	defer cpuModelsLock.Unlock()

	if len(models) == 0 {
		models = nil
	}
	if reflect.DeepEqual(cpuModels, models) {
		return false
	}
	cpuModels = models
	return true
}

func collectCPUModels(nodes []corev1.Node) []hcov1.CPUModelStatus {
	var schedulable []string
	supportedBy := map[string]sets.Set[string]{}

	for i := range nodes {
		if nodes[i].Labels[kubevirtv1.NodeSchedulable] != "true" {
			continue
		}
		name := nodes[i].Name
		schedulable = append(schedulable, name)

		for key, value := range nodes[i].Labels {
			model, ok := strings.CutPrefix(key, kubevirtv1.CPUModelLabel)
			if value != "true" || !ok || model == "" {
				continue
			}
			if supportedBy[model] == nil {
				supportedBy[model] = sets.New[string]()
			}
			supportedBy[model].Insert(name)
		}
	}
	if len(supportedBy) == 0 {
		return nil
	}

	names := slices.Collect(maps.Keys(supportedBy))
	slices.SortFunc(names, func(a, b string) int {
		af, av := parseCPUModel(a)
		bf, bv := parseCPUModel(b)
		return cmp.Or(
			cmp.Compare(cpuFamilyRank(af), cpuFamilyRank(bf)),
			cmp.Compare(bv, av),
			strings.Compare(a, b),
		)
	})

	out := make([]hcov1.CPUModelStatus, 0, len(names))
	for _, model := range names {
		var unsupported []string
		for _, node := range schedulable {
			if !supportedBy[model].Has(node) {
				unsupported = append(unsupported, node)
			}
		}
		if len(unsupported) == 0 {
			unsupported = nil
		} else {
			slices.Sort(unsupported)
		}
		out = append(out, hcov1.CPUModelStatus{Name: model, UnsupportedNodes: unsupported})
	}
	return out
}

// cpuFamiliesNewestFirst follows libvirt's x86 cpu_map groups, newest family first.
var cpuFamiliesNewestFirst = []string{
	// Intel CPU models
	"DiamondRapids",
	"KnightsMill",
	"Denverton",
	"ClearwaterForest",
	"SierraForest",
	"GraniteRapids",
	"SapphireRapids",
	"Snowridge",
	"Cooperlake",
	"Icelake-Server",
	"Icelake-Client",
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

	// AMD CPU models
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

	// Generic QEMU CPU models
	"kvm64",
	"qemu64",
	"cpu64-rhel6",
	"cpu64-rhel5",
	"kvm32",
	"qemu32",

	// Intel-based QEMU generic CPU models
	"core2duo",
	"n270",
	"coreduo",
	"pentiumpro",
	"pentium3",
	"pentium2",
	"pentium",

	// generic
	"486",
}

func parseCPUModel(model string) (family string, version int) {
	for _, name := range cpuFamiliesNewestFirst {
		if model != name && !strings.HasPrefix(model, name+"-") {
			continue
		}
		if len(name) <= len(family) {
			continue
		}
		family = name
		version = 0
		if rest, ok := strings.CutPrefix(model, name+"-v"); ok {
			if n, err := strconv.Atoi(rest); err == nil {
				version = n
			}
		}
	}
	if family == "" {
		return model, 0
	}
	return family, version
}

func cpuFamilyRank(family string) int {
	if i := slices.Index(cpuFamiliesNewestFirst, family); i >= 0 {
		return i
	}
	return len(cpuFamiliesNewestFirst)
}
