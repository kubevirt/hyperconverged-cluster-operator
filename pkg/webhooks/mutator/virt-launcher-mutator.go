package mutator

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	v1 "kubevirt.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

var _ admission.Handler = &VirtLauncherMutator{}

const (
	kubevirtIoAnnotationPrefix = "kubevirt.io/"

	cpuLimitToRequestRatioAnnotation    = kubevirtIoAnnotationPrefix + "cpu-limit-to-request-ratio"
	memoryLimitToRequestRatioAnnotation = kubevirtIoAnnotationPrefix + "memory-limit-to-request-ratio"

	enableMemoryHeadroomAnnotation = kubevirtIoAnnotationPrefix + "enable-guest-to-request-memory-headroom"
	customMemoryHeadroomAnnotation = kubevirtIoAnnotationPrefix + "custom-guest-to-request-memory-headroom"

	launcherMutatorStr = "virtLauncherMutator"

	virtLauncherVmiNameLabel = "vm.kubevirt.io/name"
)

type VirtLauncherMutator struct {
	cli          client.Client
	hcoNamespace string
	decoder      *admission.Decoder
	logger       logr.Logger
}

type virtLauncherCreationConfig struct {
	enforceCpuLimits    bool
	cpuRatioStr         string
	enforceMemoryLimits bool
	memRatioStr         string

	addMemoryHeadroom bool
	memoryHeadroom    string
}

func NewVirtLauncherMutator(cli client.Client, hcoNamespace string) *VirtLauncherMutator {
	return &VirtLauncherMutator{
		cli:          cli,
		hcoNamespace: hcoNamespace,
		logger:       log.Log.WithName("virt-launcher mutator"),
	}
}

func (m *VirtLauncherMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	m.logInfo("Starting virt-launcher mutator handling")

	if req.Operation != admissionv1.Create {
		m.logInfo("not a pod creation - ignoring")
		return admission.Allowed(ignoreOperationMessage)
	}

	launcherPod := &k8sv1.Pod{}
	err := m.decoder.Decode(req, launcherPod)
	if err != nil {
		m.logErr(err, "cannot decode virt-launcher pod")
		return admission.Errored(http.StatusBadRequest, err)
	}
	originalPod := launcherPod.DeepCopy()

	hco, err := getHcoObject(ctx, m.cli, m.hcoNamespace)
	if err != nil {
		m.logErr(err, "cannot get the HyperConverged object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	creationConfig, err := m.generateCreationConfig(ctx, launcherPod.Namespace, hco)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := m.handleVirtLauncherCreation(ctx, launcherPod, creationConfig); err != nil {
		m.logErr(err, "failed handling launcher pod %s", launcherPod.Name)
		return admission.Errored(http.StatusBadRequest, err)
	}

	allowResponse := m.getAllowedResponseWithPatches(launcherPod, originalPod)
	m.logInfo("mutation completed successfully for pod %s", launcherPod.Name)
	return allowResponse
}

func (m *VirtLauncherMutator) generateCreationConfig(ctx context.Context, namespace string, hco *v1beta1.HyperConverged) (creationConfig virtLauncherCreationConfig, err error) {
	creationConfig, err = m.getResourcesToEnforce(ctx, namespace, hco, creationConfig)
	if err != nil {
		return
	}

	if val, exists := hco.Annotations[enableMemoryHeadroomAnnotation]; exists && val == "true" {
		creationConfig.addMemoryHeadroom = true

		if customSystemMemory, customSystemMemoryExists := hco.Annotations[customMemoryHeadroomAnnotation]; customSystemMemoryExists {
			creationConfig.memoryHeadroom = customSystemMemory
		} else {
			creationConfig.memoryHeadroom = "2G"
		}
	}

	return
}

func (m *VirtLauncherMutator) handleVirtLauncherCreation(ctx context.Context, launcherPod *k8sv1.Pod, creationConfig virtLauncherCreationConfig) error {
	err := m.handleVirtLauncherResourceRatio(launcherPod, creationConfig)
	if err != nil {
		return err
	}

	err = m.handleVirtLauncherMemoryHeadroom(ctx, launcherPod, creationConfig)
	if err != nil {
		m.logger.Error(err, "setting memory headroom for pod %s/%s is skipped", launcherPod.Namespace, launcherPod.Name)
	}

	return nil
}

func (m *VirtLauncherMutator) handleVirtLauncherResourceRatio(launcherPod *k8sv1.Pod, creationConfig virtLauncherCreationConfig) error {
	if creationConfig.enforceCpuLimits {
		err := m.setResourceRatio(launcherPod, creationConfig.cpuRatioStr, cpuLimitToRequestRatioAnnotation, k8sv1.ResourceCPU)
		if err != nil {
			return err
		}
	}
	if creationConfig.enforceMemoryLimits {
		err := m.setResourceRatio(launcherPod, creationConfig.memRatioStr, memoryLimitToRequestRatioAnnotation, k8sv1.ResourceMemory)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *VirtLauncherMutator) handleVirtLauncherMemoryHeadroom(ctx context.Context, launcherPod *k8sv1.Pod, creationConfig virtLauncherCreationConfig) error {
	if !creationConfig.addMemoryHeadroom {
		return nil
	}

	var err error
	var memoryRequest resource.Quantity
	var computeContainerIdx = -1

	// fetch VMI
	var vmi *v1.VirtualMachineInstance
	if vmiName, exists := launcherPod.Labels[virtLauncherVmiNameLabel]; exists {
		vmi, err = getVmi(ctx, m.cli, vmiName, launcherPod.Namespace)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("warning: cannot find label key %s in virt launcher pod %s/%s", virtLauncherVmiNameLabel, launcherPod.Namespace, launcherPod.Name)
	}

	if _, memoryLimitExists := vmi.Spec.Domain.Resources.Limits[k8sv1.ResourceMemory]; memoryLimitExists {
		m.logger.Info("memory limit exists for vmi %s/%s. skipping setting memory memory headroom", vmi.Namespace, vmi.Name)
		return nil
	}

	// Find compute container and original memory request
	for idx, container := range launcherPod.Spec.Containers {
		if container.Name != "compute" {
			continue
		}

		var exists bool
		memoryRequest, exists = container.Resources.Requests[k8sv1.ResourceMemory]
		if !exists {
			return fmt.Errorf("compute container doesn't request any memory. skipping reserving system memory")
		}

		computeContainerIdx = idx
		break
	}

	if computeContainerIdx == -1 {
		return fmt.Errorf("could not find compute container for pod %s/%s", launcherPod.Namespace, launcherPod.Name)
	}

	memoryRequest.Add(resource.MustParse(creationConfig.memoryHeadroom))
	launcherPod.Spec.Containers[computeContainerIdx].Resources.Requests[k8sv1.ResourceMemory] = memoryRequest

	return nil
}

func (m *VirtLauncherMutator) setResourceRatio(launcherPod *k8sv1.Pod, ratioStr, annotationKey string, resourceName k8sv1.ResourceName) error {
	ratio, err := strconv.ParseFloat(ratioStr, 64)
	if err != nil {
		return fmt.Errorf("%s can't parse ratio %s to float: %w. The ratio is the value of annotation key %s", launcherMutatorStr, ratioStr, err, annotationKey)
	}

	if ratio < 1 {
		return fmt.Errorf("%s doesn't support negative ratio: %v. The ratio is the value of annotation key %s", launcherMutatorStr, ratio, annotationKey)
	}

	for i, container := range launcherPod.Spec.Containers {
		request, requestExists := container.Resources.Requests[resourceName]
		_, limitExists := container.Resources.Limits[resourceName]

		if requestExists && !limitExists {
			newQuantity := m.multiplyResource(request, ratio)
			m.logInfo("Replacing %s old quantity (%s) with new quantity (%s) for pod %s/%s, UID: %s, accodring to ratio: %v",
				resourceName, request.String(), newQuantity.String(), launcherPod.Namespace, launcherPod.Name, launcherPod.UID, ratio)

			launcherPod.Spec.Containers[i].Resources.Limits[resourceName] = newQuantity
		}
	}

	return nil
}

func (m *VirtLauncherMutator) multiplyResource(quantity resource.Quantity, ratio float64) resource.Quantity {
	oldValue := quantity.ScaledValue(resource.Milli)
	newValue := ratio * float64(oldValue)
	newQuantity := *resource.NewScaledQuantity(int64(newValue), resource.Milli)

	return newQuantity
}

// InjectDecoder injects the decoder.
// WebhookHandler implements admission.DecoderInjector so a decoder will be automatically injected.
func (m *VirtLauncherMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

func (m *VirtLauncherMutator) logInfo(format string, a ...any) {
	m.logger.Info(fmt.Sprintf(format, a...))
}

func (m *VirtLauncherMutator) logErr(err error, format string, a ...any) {
	m.logger.Error(err, fmt.Sprintf(format, a...))
}

func (m *VirtLauncherMutator) getAllowedResponseWithPatches(launcherPod, originalPod *k8sv1.Pod) admission.Response {
	const patchReplaceOp = "replace"
	allowedResponse := admission.Allowed("")

	if !equality.Semantic.DeepEqual(launcherPod.Spec, originalPod.Spec) {
		m.logInfo("generating spec replace patch for pod %s", launcherPod.Name)
		allowedResponse.Patches = append(allowedResponse.Patches,
			jsonpatch.Operation{
				Operation: patchReplaceOp,
				Path:      "/spec",
				Value:     launcherPod.Spec,
			},
		)
	}

	if !equality.Semantic.DeepEqual(launcherPod.ObjectMeta, originalPod.ObjectMeta) {
		m.logInfo("generating metadata replace patch for pod %s", launcherPod.Name)
		allowedResponse.Patches = append(allowedResponse.Patches,
			jsonpatch.Operation{
				Operation: patchReplaceOp,
				Path:      "/metadata",
				Value:     launcherPod.ObjectMeta,
			},
		)
	}

	return allowedResponse
}

func (m *VirtLauncherMutator) listResourceQuotas(ctx context.Context, namespace string) ([]k8sv1.ResourceQuota, error) {
	quotaList := &k8sv1.ResourceQuotaList{}
	err := m.cli.List(ctx, quotaList, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	return quotaList.Items, nil
}

func (m *VirtLauncherMutator) getResourcesToEnforce(ctx context.Context, namespace string, hco *v1beta1.HyperConverged, config virtLauncherCreationConfig) (virtLauncherCreationConfig, error) {
	cpuRatioStr, cpuRatioExists := hco.Annotations[cpuLimitToRequestRatioAnnotation]
	memRatioStr, memRatioExists := hco.Annotations[memoryLimitToRequestRatioAnnotation]

	if !cpuRatioExists && !memRatioExists {
		return config, nil
	}

	resourceQuotaList, err := m.listResourceQuotas(ctx, namespace)
	if err != nil {
		m.logErr(err, "could not list resource quotas")
		return config, err
	}

	isQuotaEnforcingResource := func(resourceQuota k8sv1.ResourceQuota, resource k8sv1.ResourceName) bool {
		_, exists := resourceQuota.Spec.Hard[resource]
		return exists
	}

	for _, resourceQuota := range resourceQuotaList {
		if cpuRatioExists && isQuotaEnforcingResource(resourceQuota, "limits.cpu") {
			config.enforceCpuLimits = true
			config.cpuRatioStr = cpuRatioStr
		}
		if memRatioExists && isQuotaEnforcingResource(resourceQuota, "limits.memory") {
			config.enforceMemoryLimits = true
			config.memRatioStr = memRatioStr
		}

		if config.enforceCpuLimits && config.enforceMemoryLimits {
			break
		}
	}

	return config, nil
}
