package mutator

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

const (
	launcherPodLabel = "kubevirt.io"
	launcherPodValue = "virt-launcher"
)

var (
	launcherPodMutatorLogger = logf.Log.WithName("launcher-pod mutator")

	_ admission.Handler = &LauncherPodMutator{}
)

// LauncherPodMutator removes specified annotations from kubevirt virt-launcher pods
type LauncherPodMutator struct {
	decoder   admission.Decoder
	cli       client.Client
	namespace string
}

func NewLauncherPodMutator(cli client.Client, decoder admission.Decoder, namespace string) *LauncherPodMutator {
	return &LauncherPodMutator{
		cli:       cli,
		decoder:   decoder,
		namespace: namespace,
	}
}

func (lpm *LauncherPodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	launcherPodMutatorLogger.Info("reaching LauncherPodMutator.Handle")

	if req.Operation != admissionv1.Create {
		return admission.Allowed(ignoreOperationMessage)
	}

	return lpm.mutateLauncherPod(ctx, req)
}

func (lpm *LauncherPodMutator) mutateLauncherPod(ctx context.Context, req admission.Request) admission.Response {
	hc, err := lpm.getHyperConverged(ctx)
	if err != nil {
		launcherPodMutatorLogger.Error(err, "failed to get HyperConverged CR")
		// allow the pod to be created even if we can't get the HyperConverged CR
		return admission.Allowed("")
	}

	if !isAnnotationCleanupEnabled(hc) {
		return admission.Allowed("")
	}

	pod := &corev1.Pod{}
	err = lpm.decoder.Decode(req, pod)
	if err != nil {
		launcherPodMutatorLogger.Error(err, "failed to decode pod")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse Pod"))
	}

	if !isLauncherPod(pod) {
		return admission.Allowed("")
	}

	patches := generateVeleroAnnotationRemovalPatches(pod)
	if len(patches) > 0 {
		launcherPodMutatorLogger.Info("removing velero backup hook annotations from launcher pod",
			"pod", pod.Name,
			"namespace", pod.Namespace)
		return admission.Patched("removed velero backup hook annotations", patches...)
	}

	return admission.Allowed("")
}

func isLauncherPod(pod *corev1.Pod) bool {
	if pod.Labels == nil {
		return false
	}
	value, exists := pod.Labels[launcherPodLabel]
	return exists && value == launcherPodValue
}

func (lpm *LauncherPodMutator) getHyperConverged(ctx context.Context) (*hcov1beta1.HyperConverged, error) {
	hcList := &hcov1beta1.HyperConvergedList{}
	err := lpm.cli.List(ctx, hcList, client.InNamespace(lpm.namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list HyperConverged CRs: %w", err)
	}

	if len(hcList.Items) == 0 {
		return nil, fmt.Errorf("no HyperConverged CR found in namespace %s", lpm.namespace)
	}

	return &hcList.Items[0], nil
}

func isAnnotationCleanupEnabled(hc *hcov1beta1.HyperConverged) bool {
	return hc.Spec.OptionalWebhooks != nil &&
		hc.Spec.OptionalWebhooks.LauncherPodMutator != nil
}

func veleroBackupHookAnnotations() []string {
	return []string{
		"pre.hook.backup.velero.io/command",
		"pre.hook.backup.velero.io/container",
		"pre.hook.backup.velero.io/on-error",
		"pre.hook.backup.velero.io/timeout",
		"post.hook.backup.velero.io/command",
		"post.hook.backup.velero.io/container",
	}
}

// escapeJSONPointer escapes a string for use in a JSON Pointer (RFC 6901)
func escapeJSONPointer(s string) string {
	// Per RFC 6901, escape ~ as ~0 and / as ~1
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

func generateVeleroAnnotationRemovalPatches(pod *corev1.Pod) []jsonpatch.JsonPatchOperation {
	if pod.Annotations == nil {
		return nil
	}

	var patches []jsonpatch.JsonPatchOperation
	for _, key := range veleroBackupHookAnnotations() {
		if _, exists := pod.Annotations[key]; exists {
			// Use JSON pointer format with ~ escaping for special characters
			// The jsonpatch library requires "/" to be escaped as "~1" and "~" as "~0"
			escapedKey := escapeJSONPointer(key)
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "remove",
				Path:      fmt.Sprintf("/metadata/annotations/%s", escapedKey),
			})
		}
	}

	return patches
}
