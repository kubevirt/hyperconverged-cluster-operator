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
	defaultProbePeriodSeconds    int32 = 10
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

	if deployment.Spec.Replicas == nil {
		deployment.Spec.Replicas = new(int32(1))
	}
	for i := range deployment.Spec.Template.Spec.Containers {
		normalizeContainer(&deployment.Spec.Template.Spec.Containers[i])
	}
	for i := range deployment.Spec.Template.Spec.Volumes {
		normalizeVolume(&deployment.Spec.Template.Spec.Volumes[i])
	}
	return nil
}

func normalizeVolume(v *corev1.Volume) {
	switch {
	case v.Secret != nil && v.Secret.DefaultMode == nil:
		v.Secret.DefaultMode = new(corev1.SecretVolumeSourceDefaultMode)
	case v.ConfigMap != nil && v.ConfigMap.DefaultMode == nil:
		v.ConfigMap.DefaultMode = new(corev1.ConfigMapVolumeSourceDefaultMode)
	case v.Projected != nil && v.Projected.DefaultMode == nil:
		v.Projected.DefaultMode = new(corev1.ProjectedVolumeSourceDefaultMode)
	case v.DownwardAPI != nil && v.DownwardAPI.DefaultMode == nil:
		v.DownwardAPI.DefaultMode = new(corev1.DownwardAPIVolumeSourceDefaultMode)
	}
}

func normalizeContainer(c *corev1.Container) {
	if c.TerminationMessagePath == "" {
		c.TerminationMessagePath = corev1.TerminationMessagePathDefault
	}
	if c.TerminationMessagePolicy == "" {
		c.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	}
	for i := range c.Env {
		if ref := c.Env[i].ValueFrom; ref != nil && ref.FieldRef != nil && ref.FieldRef.APIVersion == "" {
			ref.FieldRef.APIVersion = "v1"
		}
	}
	for i := range c.Ports {
		if c.Ports[i].Protocol == "" {
			c.Ports[i].Protocol = corev1.ProtocolTCP
		}
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
	if p.PeriodSeconds == 0 {
		p.PeriodSeconds = defaultProbePeriodSeconds
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
