/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2018 Red Hat, Inc.
 *
 */
package components

import (
	"github.com/coreos/prometheus-operator/pkg/apis/monitoring"
	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	virtv1 "kubevirt.io/client-go/api/v1"
)

func newBlankCrd() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				virtv1.AppLabel: "",
			},
		},
	}
}

func NewVirtualMachineInstanceCrd() *extv1beta1.CustomResourceDefinition {
	crd := newBlankCrd()

	crd.ObjectMeta.Name = "virtualmachineinstances." + virtv1.VirtualMachineInstanceGroupVersionKind.Group
	crd.Spec = extv1beta1.CustomResourceDefinitionSpec{
		Group:    virtv1.VirtualMachineInstanceGroupVersionKind.Group,
		Version:  virtv1.ApiSupportedVersions[0].Name,
		Versions: virtv1.ApiSupportedVersions,
		Scope:    "Namespaced",

		Names: extv1beta1.CustomResourceDefinitionNames{
			Plural:     "virtualmachineinstances",
			Singular:   "virtualmachineinstance",
			Kind:       virtv1.VirtualMachineInstanceGroupVersionKind.Kind,
			ShortNames: []string{"vmi", "vmis"},
			Categories: []string{
				"all",
			},
		},
		AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
			{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
			{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
			{Name: "IP", Type: "string", JSONPath: ".status.interfaces[0].ipAddress"},
			{Name: "NodeName", Type: "string", JSONPath: ".status.nodeName"},
		},
	}

	return crd
}

func NewVirtualMachineCrd() *extv1beta1.CustomResourceDefinition {
	crd := newBlankCrd()

	crd.ObjectMeta.Name = "virtualmachines." + virtv1.VirtualMachineGroupVersionKind.Group
	crd.Spec = extv1beta1.CustomResourceDefinitionSpec{
		Group:    virtv1.VirtualMachineGroupVersionKind.Group,
		Version:  virtv1.ApiSupportedVersions[0].Name,
		Versions: virtv1.ApiSupportedVersions,
		Scope:    "Namespaced",

		Names: extv1beta1.CustomResourceDefinitionNames{
			Plural:     "virtualmachines",
			Singular:   "virtualmachine",
			Kind:       virtv1.VirtualMachineGroupVersionKind.Kind,
			ShortNames: []string{"vm", "vms"},
			Categories: []string{
				"all",
			},
		},
		AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
			{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
			{Name: "Running", Type: "boolean", JSONPath: ".spec.running"},
			{Name: "Volume", Description: "Primary Volume", Type: "string", JSONPath: ".spec.volumes[0].name"},
		},
	}

	return crd
}

func NewPresetCrd() *extv1beta1.CustomResourceDefinition {
	crd := newBlankCrd()

	crd.ObjectMeta.Name = "virtualmachineinstancepresets." + virtv1.VirtualMachineInstancePresetGroupVersionKind.Group
	crd.Spec = extv1beta1.CustomResourceDefinitionSpec{
		Group:    virtv1.VirtualMachineInstancePresetGroupVersionKind.Group,
		Version:  virtv1.ApiSupportedVersions[0].Name,
		Versions: virtv1.ApiSupportedVersions,
		Scope:    "Namespaced",

		Names: extv1beta1.CustomResourceDefinitionNames{
			Plural:     "virtualmachineinstancepresets",
			Singular:   "virtualmachineinstancepreset",
			Kind:       virtv1.VirtualMachineInstancePresetGroupVersionKind.Kind,
			ShortNames: []string{"vmipreset", "vmipresets"},
			Categories: []string{
				"all",
			},
		},
	}

	return crd
}

func NewReplicaSetCrd() *extv1beta1.CustomResourceDefinition {
	crd := newBlankCrd()
	labelSelector := ".status.labelSelector"

	crd.ObjectMeta.Name = "virtualmachineinstancereplicasets." + virtv1.VirtualMachineInstanceReplicaSetGroupVersionKind.Group
	crd.Spec = extv1beta1.CustomResourceDefinitionSpec{
		Group:    virtv1.VirtualMachineInstanceReplicaSetGroupVersionKind.Group,
		Version:  virtv1.ApiSupportedVersions[0].Name,
		Versions: virtv1.ApiSupportedVersions,
		Scope:    "Namespaced",

		Names: extv1beta1.CustomResourceDefinitionNames{
			Plural:     "virtualmachineinstancereplicasets",
			Singular:   "virtualmachineinstancereplicaset",
			Kind:       virtv1.VirtualMachineInstanceReplicaSetGroupVersionKind.Kind,
			ShortNames: []string{"vmirs", "vmirss"},
			Categories: []string{
				"all",
			},
		},
		AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
			{Name: "Desired", Type: "integer", JSONPath: ".spec.replicas",
				Description: "Number of desired VirtualMachineInstances"},
			{Name: "Current", Type: "integer", JSONPath: ".status.replicas",
				Description: "Number of managed and not final or deleted VirtualMachineInstances"},
			{Name: "Ready", Type: "integer", JSONPath: ".status.readyReplicas",
				Description: "Number of managed VirtualMachineInstances which are ready to receive traffic"},
			{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
		},
		Subresources: &extv1beta1.CustomResourceSubresources{
			Scale: &extv1beta1.CustomResourceSubresourceScale{
				SpecReplicasPath:   ".spec.replicas",
				StatusReplicasPath: ".status.replicas",
				LabelSelectorPath:  &labelSelector,
			},
		},
	}

	return crd
}

func NewVirtualMachineInstanceMigrationCrd() *extv1beta1.CustomResourceDefinition {
	crd := newBlankCrd()

	crd.ObjectMeta.Name = "virtualmachineinstancemigrations." + virtv1.VirtualMachineInstanceMigrationGroupVersionKind.Group
	crd.Spec = extv1beta1.CustomResourceDefinitionSpec{
		Group:    virtv1.VirtualMachineInstanceMigrationGroupVersionKind.Group,
		Version:  virtv1.ApiSupportedVersions[0].Name,
		Versions: virtv1.ApiSupportedVersions,
		Scope:    "Namespaced",

		Names: extv1beta1.CustomResourceDefinitionNames{
			Plural:     "virtualmachineinstancemigrations",
			Singular:   "virtualmachineinstancemigration",
			Kind:       virtv1.VirtualMachineInstanceMigrationGroupVersionKind.Kind,
			ShortNames: []string{"vmim", "vmims"},
			Categories: []string{
				"all",
			},
		},
	}

	return crd
}

// Used by manifest generation
// If you change something here, you probably need to change the CSV manifest too,
// see /manifests/release/kubevirt.VERSION.csv.yaml.in
func NewKubeVirtCrd() *extv1beta1.CustomResourceDefinition {

	// we use a different label here, so no newBlankCrd()
	crd := &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"operator.kubevirt.io": "",
			},
		},
	}

	crd.ObjectMeta.Name = "kubevirts." + virtv1.KubeVirtGroupVersionKind.Group
	crd.Spec = extv1beta1.CustomResourceDefinitionSpec{
		Group:    virtv1.KubeVirtGroupVersionKind.Group,
		Version:  virtv1.ApiSupportedVersions[0].Name,
		Versions: virtv1.ApiSupportedVersions,
		Scope:    "Namespaced",

		Names: extv1beta1.CustomResourceDefinitionNames{
			Plural:     "kubevirts",
			Singular:   "kubevirt",
			Kind:       virtv1.KubeVirtGroupVersionKind.Kind,
			ShortNames: []string{"kv", "kvs"},
			Categories: []string{
				"all",
			},
		},
		AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
			{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
			{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
		},
	}

	return crd
}

func NewServiceMonitorCR(namespace string, monitorNamespace string, insecureSkipVerify bool) *promv1.ServiceMonitor {
	return &promv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoring.GroupName,
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: monitorNamespace,
			Name:      "kubevirt",
			Labels: map[string]string{
				"openshift.io/cluster-monitoring": "",
				"prometheus.kubevirt.io":          "",
				"k8s-app":                         "kubevirt",
			},
		},
		Spec: promv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"prometheus.kubevirt.io": "",
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{namespace},
			},
			Endpoints: []promv1.Endpoint{
				promv1.Endpoint{
					Port:   "metrics",
					Scheme: "https",
					TLSConfig: &promv1.TLSConfig{
						InsecureSkipVerify: insecureSkipVerify,
					},
				},
			},
		},
	}
}

// Used by manifest generation
func NewKubeVirtCR(namespace string, pullPolicy corev1.PullPolicy) *virtv1.KubeVirt {
	return &virtv1.KubeVirt{
		TypeMeta: metav1.TypeMeta{
			APIVersion: virtv1.GroupVersion.String(),
			Kind:       "KubeVirt",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "kubevirt",
		},
		Spec: virtv1.KubeVirtSpec{
			ImagePullPolicy: pullPolicy,
		},
	}
}
