package mutator

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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
	cpuLimitToRequestRatioAnnotation    = "kubevirt.io/cpu-limit-to-request-ratio"
	memoryLimitToRequestRatioAnnotation = "kubevirt.io/memory-limit-to-request-ratio"

	launcherMutatorStr = "virtLauncherMutator"
)

type VirtLauncherMutator struct {
	cli          client.Client
	hcoNamespace string
	decoder      *admission.Decoder
	logger       logr.Logger
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

	if err := m.handleVirtLauncherCreation(launcherPod, hco); err != nil {
		m.logErr(err, "failed handling launcher pod %s", launcherPod.Name)
		return admission.Errored(http.StatusBadRequest, err)
	}

	allowResponse := m.getAllowedResponseWithPatches(launcherPod, originalPod)
	m.logInfo("mutation completed successfully for pod %s", launcherPod.Name)
	return allowResponse
}

func (m *VirtLauncherMutator) handleVirtLauncherCreation(launcherPod *k8sv1.Pod, hco *v1beta1.HyperConverged) error {
	cpuRatioStr, cpuRatioExists := hco.Annotations[cpuLimitToRequestRatioAnnotation]
	memRatioStr, memRatioExists := hco.Annotations[memoryLimitToRequestRatioAnnotation]

	if !cpuRatioExists && !memRatioExists {
		return nil
	}

	if cpuRatioExists {
		err := m.setResourceRatio(launcherPod, cpuRatioStr, cpuLimitToRequestRatioAnnotation, k8sv1.ResourceCPU)
		if err != nil {
			return err
		}
	}
	if memRatioExists {
		err := m.setResourceRatio(launcherPod, memRatioStr, memoryLimitToRequestRatioAnnotation, k8sv1.ResourceMemory)
		if err != nil {
			return err
		}
	}

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
