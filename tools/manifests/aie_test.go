package manifests

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func TestAIELauncherImageEnvironment(t *testing.T) {
	const image = "registry.example/virt-launcher-4-nv@sha256:0123456789abcdef"
	for _, test := range []struct {
		name  string
		build func(*DeploymentOperatorParams) []corev1.EnvVar
		image string
		count int
	}{
		{name: "operator with image", build: buildOperatorEnvVars, image: image, count: 1},
		{name: "operator without image", build: buildOperatorEnvVars},
		{name: "webhook with image", build: buildWebhookEnvVars, image: image, count: 1},
		{name: "webhook without image", build: buildWebhookEnvVars},
	} {
		t.Run(test.name, func(t *testing.T) {
			var count int
			for _, variable := range test.build(&DeploymentOperatorParams{AIELauncherImage: test.image}) {
				if variable.Name == "AIE_WEBHOOK_IMAGE" {
					t.Fatal("retired webhook image is still deployed")
				}
				if variable.Name == util.AIELauncherImageEnvV {
					count++
					if variable.Value != test.image || variable.Value == "" {
						t.Fatalf("launcher image = %q, want %q", variable.Value, test.image)
					}
				}
			}
			if count != test.count {
				t.Fatalf("launcher environment variable count = %d, want %d", count, test.count)
			}
		})
	}
}
