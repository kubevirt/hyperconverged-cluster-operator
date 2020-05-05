package util

import (
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
	corev1 "k8s.io/api/core/v1"
)

func AddEnvAcrossContainers(installStrategy *components.StrategyDetailsDeployment, varName, varValue string) {
	for _, depSpec := range installStrategy.DeploymentSpecs {
		for index, container := range depSpec.Spec.Template.Spec.Containers {
			UpdateEnvVar(&container, varName, varValue)
			depSpec.Spec.Template.Spec.Containers[index] = container
		}
	}
}

func UpdateEnvVar(container *corev1.Container, varName, varValue string) {
	if container.Env == nil {
		container.Env = make([]corev1.EnvVar, 0, 1)
	} else {
		for _, env := range container.Env {
			if env.Name == varName {
				env.Value = varValue
				return
			}
		}
	}

	container.Env = append(container.Env, corev1.EnvVar{Name: varName, Value: varValue})
}

