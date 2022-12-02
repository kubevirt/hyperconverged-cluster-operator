package tests

import (
	"context"
	"fmt"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	v1 "kubevirt.io/api/core/v1"
	migrationsv1 "kubevirt.io/api/migrations/v1alpha1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/tests/framework/cleanup"
	util2 "kubevirt.io/kubevirt/tests/util"

	. "github.com/onsi/gomega"
)

// If matchingNSLabels is zero, namespace parameter is being ignored and can be nil
func PreparePolicyAndVMIWithNsAndVmiLabels(vmi *v1.VirtualMachineInstance, namespace *k8sv1.Namespace, matchingVmiLabels, matchingNSLabels int) *migrationsv1.MigrationPolicy {
	Expect(vmi).ToNot(BeNil())
	if matchingNSLabels > 0 {
		Expect(namespace).ToNot(BeNil())
	}

	policyName := fmt.Sprintf("testpolicy-%s", rand.String(5))
	policy := kubecli.NewMinimalMigrationPolicy(policyName)
	if policy.Labels == nil {
		policy.Labels = map[string]string{}
	}
	policy.Labels[cleanup.TestLabelForNamespace(util2.NamespaceTestDefault)] = ""

	if vmi.Labels == nil {
		vmi.Labels = make(map[string]string)
	}

	var namespaceLabels map[string]string
	if namespace != nil {
		if namespace.Labels == nil {
			namespace.Labels = make(map[string]string)
		}

		namespaceLabels = namespace.Labels
	}

	if policy.Spec.Selectors == nil {
		policy.Spec.Selectors = &migrationsv1.Selectors{
			VirtualMachineInstanceSelector: migrationsv1.LabelSelector{},
			NamespaceSelector:              migrationsv1.LabelSelector{},
		}
	} else if policy.Spec.Selectors.VirtualMachineInstanceSelector == nil {
		policy.Spec.Selectors.VirtualMachineInstanceSelector = migrationsv1.LabelSelector{}
	} else if policy.Spec.Selectors.NamespaceSelector == nil {
		policy.Spec.Selectors.NamespaceSelector = migrationsv1.LabelSelector{}
	}

	labelKeyPattern := policyName + "-key-%d"
	labelValuePattern := policyName + "-value-%d"

	applyLabels := func(policyLabels, vmiOrNSLabels map[string]string, labelCount int) {
		for i := 0; i < labelCount; i++ {
			labelKey := fmt.Sprintf(labelKeyPattern, i)
			labelValue := fmt.Sprintf(labelValuePattern, i)

			vmiOrNSLabels[labelKey] = labelValue
			policyLabels[labelKey] = labelValue
		}
	}

	applyLabels(policy.Spec.Selectors.VirtualMachineInstanceSelector, vmi.Labels, matchingVmiLabels)
	applyLabels(policy.Spec.Selectors.NamespaceSelector, namespaceLabels, matchingNSLabels)

	if namespace != nil {
		namespace.Labels = namespaceLabels
	}

	return policy
}

// PreparePolicyAndVMI mutates the given vmi parameter by adding labels to it. Therefore, it's recommended
// to use this function before creating the vmi. Otherwise, its labels need to be updated.
func PreparePolicyAndVMI(vmi *v1.VirtualMachineInstance) *migrationsv1.MigrationPolicy {
	return PreparePolicyAndVMIWithNsAndVmiLabels(vmi, nil, 1, 0)
}

func PreparePolicyAndVMIWithBandwidthLimitation(vmi *v1.VirtualMachineInstance, bandwidth resource.Quantity) *migrationsv1.MigrationPolicy {
	policy := PreparePolicyAndVMI(vmi)
	policy.Spec.BandwidthPerMigration = &bandwidth

	return policy
}

func CreateMigrationPolicy(virtClient kubecli.KubevirtClient, policy *migrationsv1.MigrationPolicy) *migrationsv1.MigrationPolicy {
	var err error

	policy, err = virtClient.MigrationPolicy().Create(context.Background(), policy, metav1.CreateOptions{})
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), "migration policy creation failed")

	return policy
}
