package manifests

import (
	"testing"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func TestAIELauncherImageEnvironment(t *testing.T) {
	const image = "registry.example/virt-launcher-4-nv@sha256:0123456789abcdef"
	env := buildOperatorEnvVars(&DeploymentOperatorParams{AIELauncherImage: image})
	for _, variable := range env {
		if variable.Name == "AIE_WEBHOOK_IMAGE" {
			t.Fatal("retired webhook image is still deployed")
		}
		if variable.Name == util.AIELauncherImageEnvV {
			if variable.Value != image {
				t.Fatalf("launcher image = %q, want %q", variable.Value, image)
			}
			return
		}
	}
	t.Fatal("bundle launcher image is missing from the HCO environment")
}
