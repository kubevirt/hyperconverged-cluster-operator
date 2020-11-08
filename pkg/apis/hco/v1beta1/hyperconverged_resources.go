package v1beta1

import (
	"fmt"
	"os"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	consolev1 "github.com/openshift/api/console/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *HyperConverged) getNamespace(defaultNamespace string, opts []string) string {
	if len(opts) > 0 {
		return opts[0]
	}
	return defaultNamespace
}

func (r *HyperConverged) getLabels() map[string]string {
	hcoName := HyperConvergedName

	if r.Name != "" {
		hcoName = r.Name
	}

	return map[string]string{
		hcoutil.AppLabel: hcoName,
	}
}

func (r *HyperConverged) NewKubeVirtPriorityClass() *schedulingv1.PriorityClass {
	return &schedulingv1.PriorityClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "scheduling.k8s.io/v1",
			Kind:       "PriorityClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "kubevirt-cluster-critical",
			Labels: r.getLabels(),
		},
		// 1 billion is the highest value we can set
		// https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass
		Value:         1000000000,
		GlobalDefault: false,
		Description:   "This priority class should be used for KubeVirt core components only.",
	}
}

func (r *HyperConverged) NewConsoleCLIDownload() *consolev1.ConsoleCLIDownload {
	kv := os.Getenv(hcoutil.KubevirtVersionEnvV)
	url := fmt.Sprintf("https://github.com/kubevirt/kubevirt/releases/%s", kv)
	text := fmt.Sprintf("KubeVirt %s release downloads", kv)

	if val, ok := os.LookupEnv("VIRTCTL_DOWNLOAD_URL"); ok && val != "" {
		url = val
	}

	if val, ok := os.LookupEnv("VIRTCTL_DOWNLOAD_TEXT"); ok && val != "" {
		text = val
	}

	return &consolev1.ConsoleCLIDownload{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "virtctl-clidownloads-" + r.Name,
			Labels: r.getLabels(),
		},

		Spec: consolev1.ConsoleCLIDownloadSpec{
			Description: "The virtctl client is a supplemental command-line utility for managing virtualization resources from the command line.",
			DisplayName: "virtctl - KubeVirt command line interface",
			Links: []consolev1.CLIDownloadLink{
				{
					Href: url,
					Text: text,
				},
			},
		},
	}
}
