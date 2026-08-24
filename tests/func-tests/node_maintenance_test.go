package tests_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
	"github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests/libnode"
)

const (
	nodeMaintenanceTimeout  = 10 * time.Minute
	nodeMaintenancePolling  = 5 * time.Second
	virtLauncherDomainLabel = kubevirtcorev1.DomainAnnotation
)

var _ = Describe("KubeVirt node maintenance", Serial, Label(tests.HighlyAvailableClusterLabel, tests.DestructiveLabel, "nodeMaintenance"), func() {
	tests.FlagParse()

	var (
		cli    client.Client
		cliSet *kubernetes.Clientset
	)

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		cliSet = tests.GetK8sClientSet()
		tests.BeforeEach(ctx)
	})

	It("should live migrate a VM after its node is cordoned and its launcher is evicted", func(ctx context.Context) {
		workers, err := libnode.ListWorkerNodes(ctx, cli)
		Expect(err).ToNot(HaveOccurred())
		tests.FailIfSingleNodeCluster(len(workers) < 2)

		availableWorkers := 0
		for i := range workers {
			if !workers[i].Spec.Unschedulable && nodeReady(&workers[i]) {
				availableWorkers++
			}
		}
		Expect(availableWorkers).To(BeNumerically(">=", 2), "node maintenance requires two Ready, schedulable worker nodes")

		vm := nodeMaintenanceVM(fmt.Sprintf("node-maintenance-vm-%d", time.Now().UnixNano()))
		By("creating a VM with an explicit LiveMigrate eviction strategy")
		Eventually(cli.Create).
			WithContext(ctx).
			WithArguments(vm).
			WithTimeout(nodeMaintenanceTimeout).
			WithPolling(nodeMaintenancePolling).
			Should(Succeed())

		cordonedNode := ""
		sourceWasUnschedulable := false
		DeferCleanup(func(cleanupCtx context.Context) {
			if cordonedNode != "" && !sourceWasUnschedulable {
				By(fmt.Sprintf("uncordoning node %s", cordonedNode))
				Eventually(func() error {
					return patchNodeUnschedulable(cleanupCtx, cliSet, cordonedNode, false)
				}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(cleanupCtx).Should(Succeed())
			}

			By("deleting the test VM")
			Eventually(func() error {
				err := cli.Delete(cleanupCtx, vm)
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(cleanupCtx).Should(Succeed())
		})

		vmi := &kubevirtcorev1.VirtualMachineInstance{}
		By("waiting for the VM's VMI to become Running and live-migratable")
		Eventually(func(g Gomega, pollCtx context.Context) {
			g.Expect(cli.Get(pollCtx, client.ObjectKey{Namespace: tests.TestNamespace, Name: vm.Name}, vmi)).To(Succeed())
			g.Expect(vmi.Status.Phase).To(Equal(kubevirtcorev1.Running), vmiFailureMessage(vmi))
			g.Expect(vmi.Status.NodeName).ToNot(BeEmpty())
			g.Expect(vmi.IsMigratable()).To(BeTrue(), vmiFailureMessage(vmi))
		}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(ctx).Should(Succeed())

		sourceNode := vmi.Status.NodeName
		vmiUID := vmi.UID
		Expect(sourceNode).ToNot(BeEmpty())
		Expect(vmiUID).ToNot(BeEmpty())
		sourceNodeObject, err := cliSet.CoreV1().Nodes().Get(ctx, sourceNode, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeReady(sourceNodeObject)).To(BeTrue(), "VMI source node %s is no longer Ready", sourceNode)
		sourceWasUnschedulable = sourceNodeObject.Spec.Unschedulable
		Expect(sourceWasUnschedulable).To(BeFalse(), "VMI started on an already cordoned node")

		launcherPod := &corev1.Pod{}
		By("finding the virt-launcher pod on the source node")
		Eventually(func(g Gomega, pollCtx context.Context) {
			pods := &corev1.PodList{}
			g.Expect(cli.List(pollCtx, pods, client.InNamespace(tests.TestNamespace))).To(Succeed())
			for i := range pods.Items {
				if pods.Items[i].Spec.NodeName == sourceNode && pods.Items[i].Status.Phase == corev1.PodRunning && isVirtLauncherForVMI(&pods.Items[i], vm.Name, vmiUID) {
					*launcherPod = pods.Items[i]
					return
				}
			}
			g.Expect(launcherPod.Name).ToNot(BeEmpty(), "virt-launcher pod was not Running on VMI source node")
		}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(ctx).Should(Succeed())

		cordonedNode = sourceNode
		By(fmt.Sprintf("cordoning source node %s", sourceNode))
		Eventually(func() error {
			return patchNodeUnschedulable(ctx, cliSet, sourceNode, true)
		}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(ctx).Should(Succeed())

		By(fmt.Sprintf("evicting virt-launcher pod %s through the Kubernetes eviction API", launcherPod.Name))
		Eventually(func() bool {
			err := cliSet.PolicyV1().Evictions(tests.TestNamespace).Evict(ctx, &policyv1.Eviction{
				ObjectMeta: metav1.ObjectMeta{Name: launcherPod.Name, Namespace: tests.TestNamespace},
			})
			if err == nil || apierrors.IsNotFound(err) {
				return true
			}
			GinkgoWriter.Printf("waiting for eviction of %s: %v\n", launcherPod.Name, err)
			return false
		}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(ctx).Should(BeTrue())

		By("verifying that the same VMI UID is Running on a different node")
		var observedMigration *kubevirtcorev1.VirtualMachineInstanceMigration
		Eventually(func(g Gomega, pollCtx context.Context) bool {
			// Capture the VMIM before checking the VMI node: completed migrations may be
			// garbage-collected shortly after the VMI is observed on its target node.
			migrations := &kubevirtcorev1.VirtualMachineInstanceMigrationList{}
			g.Expect(cli.List(pollCtx, migrations, client.InNamespace(tests.TestNamespace))).To(Succeed())
			if candidate := preferredMigrationForVMI(migrations.Items, vm.Name); candidate != nil && shouldRecordMigration(candidate, observedMigration) {
				observedMigration = candidate.DeepCopy()
			}

			current := &kubevirtcorev1.VirtualMachineInstance{}
			g.Expect(cli.Get(pollCtx, client.ObjectKey{Namespace: tests.TestNamespace, Name: vm.Name}, current)).To(Succeed())
			g.Expect(current.UID).To(Equal(vmiUID), vmiFailureMessage(current))
			g.Expect(current.Status.Phase).To(Equal(kubevirtcorev1.Running), vmiFailureMessage(current))
			g.Expect(current.IsMigratable()).To(BeTrue(), vmiFailureMessage(current))
			if current.Status.NodeName == sourceNode {
				return false
			}

			return observedMigration != nil && observedMigration.Status.Phase == kubevirtcorev1.MigrationSucceeded
		}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(ctx).Should(BeTrue(), func() string {
			if observedMigration == nil {
				return fmt.Sprintf("VMI did not complete migration from cordoned node %s; no matching VMIM was observed", sourceNode)
			}
			return fmt.Sprintf("VMI did not complete migration from cordoned node %s; VMIM=%s phase=%s", sourceNode, observedMigration.Name, observedMigration.Status.Phase)
		})
		Expect(observedMigration).ToNot(BeNil(), "the eviction did not create a VMIM for the test VMI")
		Expect(observedMigration.Status.Phase).To(Equal(kubevirtcorev1.MigrationSucceeded),
			fmt.Sprintf("VMIM %s did not complete successfully", observedMigration.Name))
		observedMigrationState := observedMigration.Status.MigrationState
		Expect(observedMigrationState).ToNot(BeNil(),
			fmt.Sprintf("completed VMIM %s did not expose migrationState with sourceNode and targetNode", observedMigration.Name))
		Expect(observedMigrationState.SourceNode).ToNot(BeEmpty(),
			fmt.Sprintf("completed VMIM %s migrationState.sourceNode is empty", observedMigration.Name))
		Expect(observedMigrationState.SourceNode).To(Equal(sourceNode),
			fmt.Sprintf("completed VMIM %s migrationState.sourceNode does not match the VMI source", observedMigration.Name))
		Expect(observedMigrationState.TargetNode).ToNot(BeEmpty(),
			fmt.Sprintf("completed VMIM %s migrationState.targetNode is empty", observedMigration.Name))
		Expect(observedMigrationState.TargetNode).ToNot(Equal(sourceNode))

		By("verifying that exactly one virt-launcher remains active")
		Eventually(func(g Gomega, pollCtx context.Context) int {
			pods := &corev1.PodList{}
			g.Expect(cli.List(pollCtx, pods, client.InNamespace(tests.TestNamespace))).To(Succeed())
			return activeVirtLauncherCount(pods, vm.Name, vmiUID)
		}).WithTimeout(nodeMaintenanceTimeout).WithPolling(nodeMaintenancePolling).WithContext(ctx).Should(Equal(1))
	})
})

func nodeMaintenanceVM(name string) *kubevirtcorev1.VirtualMachine {
	strategy := kubevirtcorev1.EvictionStrategyLiveMigrate
	runStrategy := kubevirtcorev1.RunStrategyAlways
	return &kubevirtcorev1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tests.TestNamespace,
		},
		Spec: kubevirtcorev1.VirtualMachineSpec{
			RunStrategy: &runStrategy,
			Template: &kubevirtcorev1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{virtLauncherDomainLabel: name},
				},
				Spec: kubevirtcorev1.VirtualMachineInstanceSpec{
					EvictionStrategy: &strategy,
					Domain: kubevirtcorev1.DomainSpec{
						Resources: kubevirtcorev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")},
						},
						Devices: kubevirtcorev1.Devices{Interfaces: []kubevirtcorev1.Interface{{
							Name:                   kubevirtcorev1.DefaultPodNetwork().Name,
							InterfaceBindingMethod: kubevirtcorev1.InterfaceBindingMethod{Masquerade: &kubevirtcorev1.InterfaceMasquerade{}},
						}}},
					},
					Networks: []kubevirtcorev1.Network{*kubevirtcorev1.DefaultPodNetwork()},
				},
			},
		},
	}
}

func isVirtLauncherForVMI(pod *corev1.Pod, vmName string, vmiUID types.UID) bool {
	if pod.Annotations[virtLauncherDomainLabel] == vmName || pod.Labels[virtLauncherDomainLabel] == vmName {
		return true
	}
	if pod.Labels[kubevirtcorev1.CreatedByLabel] == string(vmiUID) {
		return true
	}
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "VirtualMachineInstance" && (owner.Name == vmName || owner.UID == vmiUID) {
			return true
		}
	}
	return false
}

func activeVirtLauncherCount(pods *corev1.PodList, vmName string, vmiUID types.UID) int {
	count := 0
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == corev1.PodRunning && isVirtLauncherForVMI(pod, vmName, vmiUID) {
			count++
		}
	}
	return count
}

func preferredMigrationForVMI(migrations []kubevirtcorev1.VirtualMachineInstanceMigration, vmiName string) *kubevirtcorev1.VirtualMachineInstanceMigration {
	var preferred *kubevirtcorev1.VirtualMachineInstanceMigration
	for i := range migrations {
		candidate := &migrations[i]
		if candidate.Spec.VMIName != vmiName {
			continue
		}
		if preferred == nil || migrationIsPreferred(candidate, preferred) {
			preferred = candidate
		}
	}
	return preferred
}

func migrationIsPreferred(candidate, current *kubevirtcorev1.VirtualMachineInstanceMigration) bool {
	candidateSucceeded := candidate.Status.Phase == kubevirtcorev1.MigrationSucceeded
	currentSucceeded := current.Status.Phase == kubevirtcorev1.MigrationSucceeded
	if candidateSucceeded != currentSucceeded {
		return candidateSucceeded
	}
	return candidate.CreationTimestamp.Time.After(current.CreationTimestamp.Time)
}

func shouldRecordMigration(candidate, observed *kubevirtcorev1.VirtualMachineInstanceMigration) bool {
	if observed == nil {
		return true
	}
	// Keep updating the same object as its phase/state changes, while retaining
	// a previously observed successful migration if that object is garbage-collected.
	if candidate.Name == observed.Name {
		return true
	}
	return migrationIsPreferred(candidate, observed)
}

func nodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func patchNodeUnschedulable(ctx context.Context, cli *kubernetes.Clientset, nodeName string, unschedulable bool) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable))
	_, err := cli.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func vmiFailureMessage(vmi *kubevirtcorev1.VirtualMachineInstance) string {
	conditions := make([]string, 0, len(vmi.Status.Conditions))
	for _, condition := range vmi.Status.Conditions {
		conditions = append(conditions, fmt.Sprintf("%s=%s (%s): %s", condition.Type, condition.Status, condition.Reason, condition.Message))
	}
	return fmt.Sprintf("VMI phase=%s node=%s migrationMethod=%s conditions=%s", vmi.Status.Phase, vmi.Status.NodeName, vmi.Status.MigrationMethod, strings.Join(conditions, "; "))
}
