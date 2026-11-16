package virtimport

import (
	"encoding/json"

	"github.com/distribution/reference"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Defaults the apiserver sets on containers and probes, see k8s.io/kubernetes/pkg/apis/core/v1/defaults.go.
const (
	defaultProbeTimeoutSeconds   int32 = 1
	defaultProbeSuccessThreshold int32 = 1
	defaultProbeFailureThreshold int32 = 3
)

// normalizeDeployment makes the Deployment look like the apiserver stores it, so the generic
// Deployment handler, which compares with reflect.DeepEqual, does not update it on every reconcile.
func normalizeDeployment(deployment *appsv1.Deployment) error {
	data, err := json.Marshal(deployment)
	if err != nil {
		return err
	}
	normalized := &appsv1.Deployment{}
	if err = json.Unmarshal(data, normalized); err != nil {
		return err
	}
	*deployment = *normalized

	for i := range deployment.Spec.Template.Spec.Containers {
		normalizeContainer(&deployment.Spec.Template.Spec.Containers[i])
	}
	for i := range deployment.Spec.Template.Spec.Volumes {
		normalizeVolume(&deployment.Spec.Template.Spec.Volumes[i])
	}
	return nil
}

func normalizeVolume(v *corev1.Volume) {
	if v.Secret != nil && v.Secret.DefaultMode == nil {
		v.Secret.DefaultMode = new(corev1.SecretVolumeSourceDefaultMode)
	}
}

func normalizeContainer(c *corev1.Container) {
	if c.TerminationMessagePath == "" {
		c.TerminationMessagePath = corev1.TerminationMessagePathDefault
	}
	normalizeProbe(c.StartupProbe)
	normalizeProbe(c.LivenessProbe)
	normalizeProbe(c.ReadinessProbe)
}

func normalizeProbe(p *corev1.Probe) {
	if p == nil {
		return
	}
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = defaultProbeTimeoutSeconds
	}
	if p.SuccessThreshold == 0 {
		p.SuccessThreshold = defaultProbeSuccessThreshold
	}
	if p.FailureThreshold == 0 {
		p.FailureThreshold = defaultProbeFailureThreshold
	}
	if p.HTTPGet != nil && p.HTTPGet.Scheme == "" {
		p.HTTPGet.Scheme = corev1.URISchemeHTTP
	}
}

// DefaultPullPolicy returns the image pull policy the apiserver defaults for image.
func DefaultPullPolicy(image string) corev1.PullPolicy {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return corev1.PullIfNotPresent
	}
	tagged, hasTag := named.(reference.Tagged)
	_, hasDigest := named.(reference.Digested)
	if (hasTag && tagged.Tag() == "latest") || (!hasTag && !hasDigest) {
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
}
