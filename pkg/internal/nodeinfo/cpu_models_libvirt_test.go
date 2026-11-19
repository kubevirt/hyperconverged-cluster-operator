package nodeinfo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestLibvirtCPUFamiliesAreRanked(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the CPU model test file")
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "libvirt_x86_cpu_models.txt"))
	if err != nil {
		t.Fatalf("read collected libvirt CPU models: %v", err)
	}

	collected := sets.New[string]()
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		family, _ := parseCPUModel(line)
		collected.Insert(family)
	}
	if collected.Len() == 0 {
		t.Fatal("collected libvirt CPU model list is empty")
	}

	ranked := sets.New(cpuFamiliesNewestFirst...)
	if len(ranked) != len(cpuFamiliesNewestFirst) {
		t.Error("cpuFamiliesNewestFirst contains a duplicate family")
	}
	if missing := sets.List(collected.Difference(ranked)); len(missing) > 0 {
		t.Errorf("cpuFamiliesNewestFirst is missing libvirt families: %s", strings.Join(missing, ", "))
	}
	if extra := sets.List(ranked.Difference(collected)); len(extra) > 0 {
		t.Errorf("cpuFamiliesNewestFirst has families that are not in the libvirt cpu_map: %s", strings.Join(extra, ", "))
	}
}
