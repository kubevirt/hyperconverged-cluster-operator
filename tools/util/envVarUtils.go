package util

import (
	ofv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	csvv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func AddEnvAcrossContainers(installStrategy *csvv1alpha1.StrategyDetailsDeployment, varName, varValue string) {
	for _, depSpec := range installStrategy.DeploymentSpecs {
		for index, container := range depSpec.Spec.Template.Spec.Containers {
			UpdateEnvVar(&container, varName, varValue)
			depSpec.Spec.Template.Spec.Containers[index] = container
		}
	}
}

func AddEnvAcrossContainersOf(installStrategy *ofv1alpha1.StrategyDetailsDeployment, varName, varValue string) {
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
