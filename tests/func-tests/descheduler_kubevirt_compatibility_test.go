package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	deschedulerCRDName       = "kubedeschedulers.operator.openshift.io"
	deschedulerLabelSelector = "app.kubernetes.io/name=descheduler"
	rawPolicyAPIVersion      = "descheduler/v1alpha2"
	graduatedProfile         = "KubeVirtRelieveAndMigrate"
	legacyProfile            = "DevKubeVirtRelieveAndMigrate"
	deschedulerMode          = "Automatic"
)

type deschedulerPolicyDocument struct {
	APIVersion string                   `json:"apiVersion"`
	Kind       string                   `json:"kind"`
	Profiles   []map[string]interface{} `json:"profiles"`
}

var _ = Describe("KubeVirt Descheduler documentation compatibility", Label("deschedulerCompatibility"), func() {
	tests.FlagParse()

	It("should expose the documented ACP policy and background-eviction contract", func(ctx context.Context) {
		k8sClient := tests.GetK8sClientSet()
		configMaps, err := k8sClient.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{LabelSelector: deschedulerLabelSelector})
		Expect(err).ToNot(HaveOccurred())
		if len(configMaps.Items) == 0 {
			Skip("optional Descheduler plugin is not installed; no labeled policy ConfigMap was found")
		}
		policyConfigMaps := make(map[string]struct{}, len(configMaps.Items))

		for _, configMap := range configMaps.Items {
			policyConfigMaps[configMap.Namespace+"/"+configMap.Name] = struct{}{}
			rawPolicy, ok := configMap.Data["policy.yaml"]
			Expect(ok).To(BeTrue(), "Descheduler ConfigMap %s/%s has no data.policy.yaml", configMap.Namespace, configMap.Name)
			Expect(strings.TrimSpace(rawPolicy)).ToNot(BeEmpty(), "Descheduler ConfigMap %s/%s has an empty data.policy.yaml", configMap.Namespace, configMap.Name)

			policy := deschedulerPolicyDocument{}
			Expect(yaml.Unmarshal([]byte(rawPolicy), &policy)).To(Succeed(),
				"Descheduler ConfigMap %s/%s contains invalid YAML", configMap.Namespace, configMap.Name)
			Expect(policy.APIVersion).To(Equal(rawPolicyAPIVersion),
				"update the policy howto when the ACP raw policy API changes")
			Expect(policy.Kind).To(Equal("DeschedulerPolicy"),
				"the policy howto requires a DeschedulerPolicy document")
			Expect(policy.Profiles).ToNot(BeEmpty(),
				"DeschedulerPolicy must contain at least one profile")
		}

		workloads, found, err := deschedulerWorkloads(ctx, k8sClient)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue(),
			"a labeled Descheduler ConfigMap exists but no CronJob, Deployment, or Pod consumes it")
		policyWorkloads := 0
		for _, workload := range workloads {
			if !workloadHasPolicyConfigFile(workload, policyConfigMaps) {
				continue
			}
			policyWorkloads++
			Expect(workloadPolicyContainersHaveBackgroundGate(workload, policyConfigMaps)).To(BeTrue(),
				"Descheduler workload %s must contain --feature-gates=EvictionsInBackground=true in every policy-consuming container", workload.identity)
		}
		Expect(policyWorkloads).To(BeNumerically(">", 0),
			"Descheduler workload must read /policy-dir/policy.yaml from the labeled policy ConfigMap")
	})

	It("should expose documented fields when the optional KubeDescheduler CRD is installed", func(ctx context.Context) {
		cli := tests.GetControllerRuntimeClient()
		crd := new(apiextensionsv1.CustomResourceDefinition)
		if err := cli.Get(ctx, client.ObjectKey{Name: deschedulerCRDName}, crd); apierrors.IsNotFound(err) {
			Skip(fmt.Sprintf("optional OpenShift Descheduler operator is not installed; CRD %q was not found", deschedulerCRDName))
		} else {
			Expect(err).ToNot(HaveOccurred())
		}

		schemas := servedDeschedulerSchemas(crd)
		Expect(schemas).ToNot(BeEmpty(), "the Descheduler CRD has no served OpenAPI schema")

		supported := false
		for _, schema := range schemas {
			if deschedulerSchemaSupportsKubeVirtProfile(schema) {
				supported = true
				break
			}
		}
		Expect(supported).To(BeTrue(),
			"the optional KubeDescheduler CRD no longer exposes the documented KubeVirt profile and fields")
	})
})

type deschedulerWorkload struct {
	identity   string
	namespace  string
	containers [][]string
	volumes    []corev1.Volume
	mounts     [][]corev1.VolumeMount
}

func deschedulerWorkloads(ctx context.Context, k8sClient kubernetes.Interface) ([]deschedulerWorkload, bool, error) {
	workloads := make([]deschedulerWorkload, 0)

	cronJobs, err := k8sClient.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{LabelSelector: deschedulerLabelSelector})
	if err != nil {
		return nil, false, err
	}
	for _, cronJob := range cronJobs.Items {
		workloads = append(workloads, deschedulerWorkload{
			identity:   fmt.Sprintf("CronJob %s/%s", cronJob.Namespace, cronJob.Name),
			namespace:  cronJob.Namespace,
			containers: containerArgsByContainer(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers),
			volumes:    cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes,
			mounts:     volumeMountsByContainer(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers),
		})
	}

	deployments, err := k8sClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{LabelSelector: deschedulerLabelSelector})
	if err != nil {
		return nil, false, err
	}
	for _, deployment := range deployments.Items {
		workloads = append(workloads, deschedulerWorkload{
			identity:   fmt.Sprintf("Deployment %s/%s", deployment.Namespace, deployment.Name),
			namespace:  deployment.Namespace,
			containers: containerArgsByContainer(deployment.Spec.Template.Spec.Containers),
			volumes:    deployment.Spec.Template.Spec.Volumes,
			mounts:     volumeMountsByContainer(deployment.Spec.Template.Spec.Containers),
		})
	}

	pods, err := k8sClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{LabelSelector: deschedulerLabelSelector})
	if err != nil {
		return nil, false, err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		workloads = append(workloads, deschedulerWorkload{
			identity:   fmt.Sprintf("Pod %s/%s", pod.Namespace, pod.Name),
			namespace:  pod.Namespace,
			containers: containerArgsByContainer(pod.Spec.Containers),
			volumes:    pod.Spec.Volumes,
			mounts:     volumeMountsByContainer(pod.Spec.Containers),
		})
	}

	return workloads, len(workloads) > 0, nil
}

func containerArgsByContainer(containers []corev1.Container) [][]string {
	result := make([][]string, 0, len(containers))
	for _, container := range containers {
		args := make([]string, 0, len(container.Command)+len(container.Args))
		args = append(args, container.Command...)
		args = append(args, container.Args...)
		result = append(result, args)
	}
	return result
}

func volumeMountsByContainer(containers []corev1.Container) [][]corev1.VolumeMount {
	result := make([][]corev1.VolumeMount, 0, len(containers))
	for _, container := range containers {
		result = append(result, container.VolumeMounts)
	}
	return result
}

func workloadHasPolicyConfigFile(workload deschedulerWorkload, policyConfigMaps map[string]struct{}) bool {
	for index, args := range workload.containers {
		if hasPolicyConfigFile(args) && policyMountIsConfigMap(workload, index, policyConfigMaps) {
			return true
		}
	}
	return false
}

func workloadPolicyContainersHaveBackgroundGate(workload deschedulerWorkload, policyConfigMaps map[string]struct{}) bool {
	policyContainerFound := false
	for index, args := range workload.containers {
		if !hasPolicyConfigFile(args) {
			continue
		}
		policyContainerFound = true
		if !policyMountIsConfigMap(workload, index, policyConfigMaps) || !hasBackgroundEvictionGate(args) {
			return false
		}
	}
	return policyContainerFound
}

func policyMountIsConfigMap(workload deschedulerWorkload, containerIndex int, policyConfigMaps map[string]struct{}) bool {
	if containerIndex >= len(workload.mounts) {
		return false
	}
	for _, mount := range workload.mounts[containerIndex] {
		if mount.MountPath != "/policy-dir" {
			continue
		}
		for _, volume := range workload.volumes {
			if volume.Name != mount.Name || volume.ConfigMap == nil {
				continue
			}
			_, found := policyConfigMaps[workload.namespace+"/"+volume.ConfigMap.LocalObjectReference.Name]
			if found {
				return true
			}
		}
	}
	return false
}

func hasBackgroundEvictionGate(args []string) bool {
	for index, arg := range args {
		if strings.HasPrefix(arg, "--feature-gates=") {
			arg = strings.TrimPrefix(arg, "--feature-gates=")
			if containsBackgroundEvictionGate(arg) {
				return true
			}
		}
		if arg == "--feature-gates" && index+1 < len(args) && containsBackgroundEvictionGate(args[index+1]) {
			return true
		}
	}
	return false
}

func hasPolicyConfigFile(args []string) bool {
	for index, arg := range args {
		if strings.HasPrefix(arg, "--policy-config-file=") && strings.TrimPrefix(arg, "--policy-config-file=") == "/policy-dir/policy.yaml" {
			return true
		}
		if arg == "--policy-config-file" && index+1 < len(args) && args[index+1] == "/policy-dir/policy.yaml" {
			return true
		}
	}
	return false
}

func containsBackgroundEvictionGate(value string) bool {
	for _, gate := range strings.Split(value, ",") {
		if strings.TrimSpace(gate) == "EvictionsInBackground=true" {
			return true
		}
	}
	return false
}

func servedDeschedulerSchemas(crd *apiextensionsv1.CustomResourceDefinition) []*apiextensionsv1.JSONSchemaProps {
	schemas := make([]*apiextensionsv1.JSONSchemaProps, 0, len(crd.Spec.Versions))
	for _, version := range crd.Spec.Versions {
		if version.Served && version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
			schemas = append(schemas, version.Schema.OpenAPIV3Schema)
		}
	}
	return schemas
}

func deschedulerSchemaSupportsKubeVirtProfile(schema *apiextensionsv1.JSONSchemaProps) bool {
	spec, ok := schema.Properties["spec"]
	if !ok {
		return false
	}
	profiles, ok := spec.Properties["profiles"]
	if !ok || profiles.Items == nil {
		return false
	}
	profilesAccepted := profileEnum(profiles.Items)
	if !containsAny(profilesAccepted, graduatedProfile, legacyProfile) {
		return false
	}

	customizations, ok := spec.Properties["profileCustomizations"]
	if !ok {
		return false
	}
	backgroundEvictions, ok := customizations.Properties["devEnableEvictionsInBackground"]
	if !ok || backgroundEvictions.Type != "boolean" {
		return false
	}

	mode, ok := spec.Properties["mode"]
	return ok && containsAny(jsonEnum(mode.Enum), deschedulerMode)
}

func containsAny(values []string, expected ...string) bool {
	for _, value := range values {
		for _, candidate := range expected {
			if value == candidate {
				return true
			}
		}
	}
	return false
}

func profileEnum(items *apiextensionsv1.JSONSchemaPropsOrArray) []string {
	if items == nil {
		return nil
	}
	if items.Schema != nil {
		return jsonEnum(items.Schema.Enum)
	}
	for i := range items.JSONSchemas {
		if len(items.JSONSchemas[i].Enum) > 0 {
			return jsonEnum(items.JSONSchemas[i].Enum)
		}
	}
	return nil
}

func jsonEnum(values []apiextensionsv1.JSON) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		var decoded string
		if err := json.Unmarshal(value.Raw, &decoded); err == nil {
			result = append(result, decoded)
		}
	}
	return result
}
