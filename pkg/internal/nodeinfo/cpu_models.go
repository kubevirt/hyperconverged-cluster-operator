package nodeinfo

import (
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

const (
	// CPU model labels prefix used by KubeVirt
	CpuModelLabelPrefix = "cpu-model.node.kubevirt.io/"

	// Weights for CPU model recommendation scoring
	benchmarkWeight = 0.50
	cpuWeight       = 0.20
	memoryWeight    = 0.15
	nodeWeight      = 0.15
)

// cpuModelPassMarkScores maps libvirt CPU model names to approximate PassMark scores.
// Keys must match exactly the cpu-model.node.kubevirt.io/* label values.
// The precise values do not matter much, this is intended only as a rough heuristic.
var cpuModelPassMarkScores = map[string]int{
	// Intel Broadwell
	"Broadwell":            7800,
	"Broadwell-IBRS":       7800,
	"Broadwell-noTSX":      7800,
	"Broadwell-noTSX-IBRS": 7800,
	"Broadwell-v1":         7800,
	"Broadwell-v2":         7800,
	"Broadwell-v3":         7800,
	"Broadwell-v4":         7800,

	// Intel Cascadelake
	"Cascadelake-Server":       22000,
	"Cascadelake-Server-noTSX": 22000,
	"Cascadelake-Server-v2":    22000,
	"Cascadelake-Server-v3":    22000,
	"Cascadelake-Server-v4":    22000,
	"Cascadelake-Server-v5":    22000,

	// Intel Cooperlake
	"Cooperlake":    24000,
	"Cooperlake-v1": 24000,
	"Cooperlake-v2": 24000,

	// Intel Denverton
	"Denverton":    3500,
	"Denverton-v1": 3500,
	"Denverton-v2": 3500,
	"Denverton-v3": 3500,

	// Intel Haswell
	"Haswell":            7200,
	"Haswell-IBRS":       7200,
	"Haswell-noTSX":      7200,
	"Haswell-noTSX-IBRS": 7200,
	"Haswell-v1":         7200,
	"Haswell-v2":         7200,
	"Haswell-v3":         7200,
	"Haswell-v4":         7200,

	// Intel Icelake
	"Icelake-Server":       25000,
	"Icelake-Server-noTSX": 25000,
	"Icelake-Server-v1":    25000,
	"Icelake-Server-v2":    25000,
	"Icelake-Server-v3":    25000,
	"Icelake-Server-v4":    25000,
	"Icelake-Server-v5":    25000,
	"Icelake-Server-v6":    25000,

	// Intel IvyBridge
	"IvyBridge":      6400,
	"IvyBridge-IBRS": 6400,
	"IvyBridge-v1":   6400,
	"IvyBridge-v2":   6400,

	// Intel Nehalem
	"Nehalem":      3800,
	"Nehalem-IBRS": 3800,
	"Nehalem-v1":   3800,
	"Nehalem-v2":   3800,

	// Intel Penryn
	"Penryn":    2400,
	"Penryn-v1": 2400,

	// Intel SandyBridge
	"SandyBridge":      5600,
	"SandyBridge-IBRS": 5600,
	"SandyBridge-v1":   5600,
	"SandyBridge-v2":   5600,

	// Intel SapphireRapids
	"SapphireRapids":    35000,
	"SapphireRapids-v1": 35000,
	"SapphireRapids-v2": 35000,

	// Intel Skylake-Client
	"Skylake-Client":            8900,
	"Skylake-Client-IBRS":       8900,
	"Skylake-Client-noTSX-IBRS": 8900,
	"Skylake-Client-v1":         8900,
	"Skylake-Client-v2":         8900,
	"Skylake-Client-v3":         8900,
	"Skylake-Client-v4":         8900,

	// Intel Skylake-Server
	"Skylake-Server":            15000,
	"Skylake-Server-IBRS":       15000,
	"Skylake-Server-noTSX-IBRS": 15000,
	"Skylake-Server-v1":         15000,
	"Skylake-Server-v2":         15000,
	"Skylake-Server-v3":         15000,
	"Skylake-Server-v4":         15000,
	"Skylake-Server-v5":         15000,

	// Intel Snowridge
	"Snowridge":    4500,
	"Snowridge-v1": 4500,
	"Snowridge-v2": 4500,
	"Snowridge-v3": 4500,
	"Snowridge-v4": 4500,

	// Intel Westmere
	"Westmere":      4200,
	"Westmere-IBRS": 4200,
	"Westmere-v1":   4200,
	"Westmere-v2":   4200,

	// Intel older
	"Conroe":    1800,
	"Conroe-v1": 1800,

	// AMD EPYC
	"EPYC":          25000,
	"EPYC-IBPB":     25000,
	"EPYC-v1":       25000,
	"EPYC-v2":       25000,
	"EPYC-v3":       25000,
	"EPYC-v4":       25000,
	"EPYC-Rome":     35000,
	"EPYC-Rome-v1":  35000,
	"EPYC-Rome-v2":  35000,
	"EPYC-Rome-v3":  35000,
	"EPYC-Rome-v4":  35000,
	"EPYC-Milan":    45000,
	"EPYC-Milan-v1": 45000,
	"EPYC-Milan-v2": 45000,
	"EPYC-Genoa":    55000,
	"EPYC-Genoa-v1": 55000,

	// AMD Opteron
	"Opteron_G1":    800,
	"Opteron_G1-v1": 800,
	"Opteron_G2":    1000,
	"Opteron_G2-v1": 1000,
	"Opteron_G3":    1400,
	"Opteron_G3-v1": 1400,
	"Opteron_G4":    4200,
	"Opteron_G4-v1": 4200,
	"Opteron_G5":    6800,
	"Opteron_G5-v1": 6800,

	// AMD Dhyana (Hygon)
	"Dhyana":    20000,
	"Dhyana-v1": 20000,
	"Dhyana-v2": 20000,

	// AMD phenom
	"phenom":    2800,
	"phenom-v1": 2800,
}

var cpuModelInfo = &CpuModelCache{
	lock: &sync.RWMutex{},
}

func maxPassMark() int {
	m := 0
	for _, score := range cpuModelPassMarkScores {
		if score > m {
			m = score
		}
	}
	return m
}

type CpuModelCache struct {
	lock *sync.RWMutex
	// Pre-computed sorted list of top recommended models
	recommendedModels []v1beta1.CpuModelInfo
}

func GetRecommendedCpuModels() []v1beta1.CpuModelInfo {
	cpuModelInfo.lock.RLock()
	defer cpuModelInfo.lock.RUnlock()

	// Return a deep copy of the pre-computed result
	result := make([]v1beta1.CpuModelInfo, len(cpuModelInfo.recommendedModels))
	for i := range cpuModelInfo.recommendedModels {
		cpuModelInfo.recommendedModels[i].DeepCopyInto(&result[i])
	}
	return result
}

// calculateWeightedScore computes a weighted recommendation score considering:
// - PassMark performance - CPU performance indicator
// - Available CPU cores - fraction of cluster CPU
// - Memory - fraction of cluster memory
// - Number of nodes - fraction of cluster nodes
func calculateWeightedScore(model v1beta1.CpuModelInfo, totalNodes int, totalCpu, totalMemory float64) float64 {
	// PassMark score - normalized to 0-100 scale
	passMark := float64(model.Benchmark)
	passMarkScore := (passMark / float64(maxPassMark())) * 100.0 * benchmarkWeight

	// CPU - fraction of total cluster CPU
	cpuScore := 0.0
	if totalCpu > 0 && model.CPU != nil {
		cpuScore = (model.CPU.AsApproximateFloat64() / totalCpu) * 100.0 * cpuWeight
	}

	// Memory - fraction of total cluster memory
	memoryScore := 0.0
	if totalMemory > 0 && model.Memory != nil {
		memoryGB := model.Memory.AsApproximateFloat64() / (1024 * 1024 * 1024)
		memoryScore = (memoryGB / totalMemory) * 100.0 * memoryWeight
	}

	// Number of nodes - fraction of total cluster nodes
	nodeScore := 0.0
	if totalNodes > 0 {
		nodeScore = (float64(model.Nodes) / float64(totalNodes)) * 100.0 * nodeWeight
	}

	return passMarkScore + cpuScore + memoryScore + nodeScore
}

func processCpuModels(nodes []corev1.Node) bool {
	cpuModelCounts := make(map[string]int)
	cpuModelCores := make(map[string]*resource.Quantity)
	cpuModelMemory := make(map[string]*resource.Quantity)

	// Calculate cluster totals for normalization
	totalNodes := len(nodes)
	var totalCpu float64
	var totalMemory float64
	for _, node := range nodes {
		if cpu := node.Status.Capacity.Cpu(); cpu != nil {
			totalCpu += cpu.AsApproximateFloat64()
		}
		if mem := node.Status.Capacity.Memory(); mem != nil {
			totalMemory += mem.AsApproximateFloat64() / (1024 * 1024 * 1024) // Convert to GB
		}
	}

	for _, node := range nodes {
		cpuModels := extractCpuModels(node.Labels)
		memoryCapacity := node.Status.Capacity.Memory()
		cpuCapacity := node.Status.Capacity.Cpu()

		for _, cpuModel := range cpuModels {
			cpuModelCounts[cpuModel]++

			if cpuCapacity != nil {
				if cpuModelCores[cpuModel] == nil {
					cpuModelCores[cpuModel] = resource.NewQuantity(0, resource.DecimalSI)
				}
				cpuModelCores[cpuModel].Add(*cpuCapacity)
			}

			if memoryCapacity != nil {
				if cpuModelMemory[cpuModel] == nil {
					cpuModelMemory[cpuModel] = resource.NewQuantity(0, resource.BinarySI)
				}
				cpuModelMemory[cpuModel].Add(*memoryCapacity)
			}
		}
	}

	models := make([]v1beta1.CpuModelInfo, 0, len(cpuModelCounts))
	for cpuModel, count := range cpuModelCounts {
		models = append(models, v1beta1.CpuModelInfo{
			Name:      cpuModel,
			Benchmark: cpuModelPassMarkScores[cpuModel],
			Nodes:     count,
			CPU:       cpuModelCores[cpuModel],
			Memory:    cpuModelMemory[cpuModel],
		})
	}

	// Sort by weighted score (highest first), then by name for stability
	sort.Slice(models, func(i, j int) bool {
		scoreI := calculateWeightedScore(models[i], totalNodes, totalCpu, totalMemory)
		scoreJ := calculateWeightedScore(models[j], totalNodes, totalCpu, totalMemory)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return models[i].Name < models[j].Name
	})

	// Keep top 4 models
	if len(models) > 4 {
		models = models[:4]
	}

	cpuModelInfo.lock.Lock()
	defer cpuModelInfo.lock.Unlock()

	changed := !recommendedModelsEqual(cpuModelInfo.recommendedModels, models)
	cpuModelInfo.recommendedModels = models
	return changed
}

func recommendedModelsEqual(a, b []v1beta1.CpuModelInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Benchmark != b[i].Benchmark || a[i].Nodes != b[i].Nodes {
			return false
		}
		if !quantityEqual(a[i].CPU, b[i].CPU) || !quantityEqual(a[i].Memory, b[i].Memory) {
			return false
		}
	}
	return true
}

func quantityEqual(a, b *resource.Quantity) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func extractCpuModels(labels map[string]string) []string {
	cpuModels := sets.New[string]()

	for key, value := range labels {
		if strings.HasPrefix(key, CpuModelLabelPrefix) {
			cpuModel := strings.TrimPrefix(key, CpuModelLabelPrefix)
			// Only include if the value is "true" (meaning the node supports this model)
			if value == "true" {
				cpuModels.Insert(cpuModel)
			}
		}
	}

	return cpuModels.UnsortedList()
}
