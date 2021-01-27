package tests_test

import (
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"time"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	testscore "kubevirt.io/kubevirt/tests"

	"github.com/kubevirt/cluster-network-addons-operator/pkg/apis"
	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/tests/flags"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("[rfe_id:4356][crit:medium][vendor:cnv-qe@redhat.com][level:system]Node Placement", func() {

	var workloadsNode *v1.Node

	tests.FlagParse()
	client, err := kubecli.GetKubevirtClient()
	testscore.PanicOnError(err)

	workloadsNodes, err := client.CoreV1().Nodes().List(k8smetav1.ListOptions{
		LabelSelector: "node.kubernetes.io/hco-test-node-type==workloads",
	})
	testscore.PanicOnError(err)

	if workloadsNodes != nil && len(workloadsNodes.Items) == 1 {
		workloadsNode = &workloadsNodes.Items[0]
		fmt.Fprintf(GinkgoWriter, "Found Workloads Node. Node name: %s; node labels:\n", workloadsNode.Name)
		w := json.NewEncoder(GinkgoWriter)
		w.SetIndent("", "  ")
		w.Encode(workloadsNode.Labels)
	}

	BeforeEach(func() {
		if workloadsNode == nil {
			Skip("Skipping Node Placement tests")
		}
		tests.BeforeEach()
	})

	Context("validate node placement in workloads nodes", func() {
		It("[test_id:5677] all expected 'workloads' pod must be on infra node", func() {
			expectedWorkloadsPods := map[string]bool{
				"bridge-marker": false,
				"cni-plugins":   false,
				//"kube-multus":     false,
				"nmstate-handler": false,
				"ovs-cni-marker":  false,
				"virt-handler":    false,
			}

			var cnaoCR networkaddonsv1.NetworkAddonsConfig

			s := scheme.Scheme
			_ = apis.AddToScheme(s)
			s.AddKnownTypes(networkaddonsv1.SchemeGroupVersion)
			opts := k8smetav1.GetOptions{}
			err = client.RestClient().Get().
				Resource("networkaddonsconfigs").
				Name("cluster").
				VersionedParams(&opts, scheme.ParameterCodec).
				Timeout(10 * time.Second).
				Do().Into(&cnaoCR)

			if cnaoCR.Spec.Ovs == nil {
				delete(expectedWorkloadsPods, "ovs-cni-marker")
			}

			pods, err := client.CoreV1().Pods(flags.KubeVirtInstallNamespace).List(k8smetav1.ListOptions{
				FieldSelector: fmt.Sprintf("spec.nodeName=%s", workloadsNode.Name),
			})

			Expect(err).ToNot(HaveOccurred())

			for _, pod := range pods.Items {
				podName := pod.Spec.Containers[0].Name
				fmt.Fprintf(GinkgoWriter, "Found %s pod '%s' in the 'workloads' node %s\n", podName, pod.Name, workloadsNode.Name)
				if found, ok := expectedWorkloadsPods[podName]; ok {
					if !found {
						expectedWorkloadsPods[podName] = true
					}
				}
			}

			Expect(expectedWorkloadsPods).ToNot(ContainElement(false))
		})
	})

	Context("validate node placement on infra nodes", func() {
		It("[test_id:5678] all expected 'infra' pod must be on infra node", func() {
			infraNodes, err := client.CoreV1().Nodes().List(k8smetav1.ListOptions{
				LabelSelector: "node.kubernetes.io/hco-test-node-type==infra",
			})

			Expect(err).ShouldNot(HaveOccurred())

			expectedInfraPods := map[string]bool{
				"cdi-apiserver":        false,
				"cdi-controller":       false,
				"cdi-uploadproxy":      false,
				"manager":              false,
				"nmstate-webhook":      false,
				"virt-api":             false,
				"virt-controller":      false,
				"vm-import-controller": false,
			}

			for _, node := range infraNodes.Items {
				pods, err := client.CoreV1().Pods(flags.KubeVirtInstallNamespace).List(k8smetav1.ListOptions{
					FieldSelector: fmt.Sprintf("spec.nodeName=%s", node.Name),
				})
				Expect(err).ToNot(HaveOccurred())

				for _, pod := range pods.Items {
					podName := pod.Spec.Containers[0].Name
					fmt.Fprintf(GinkgoWriter, "Found %s pod '%s' in the 'infra' node %s\n", podName, pod.Name, node.Name)
					if found, ok := expectedInfraPods[podName]; ok {
						if !found {
							expectedInfraPods[podName] = true
						}
					}
				}
			}

			Expect(expectedInfraPods).ToNot(ContainElement(false))
		})
	})

	Context("validate node placement for vmi", func() {
		It("[test_id:5679] should create, verify and delete VMIs with correct node placements", func() {
			// we are iterating many times to ensure that the vmi is not scheduled to the expected node by chance
			// this test may give false positive result. more iteration can give more accuracy.
			testAmount := 20
			for i := 0; i < testAmount; i++ {
				By(fmt.Sprintf("Run %d/%d", i+1, testAmount))
				vmiName := verifyVMICreation(client)
				vmi := verifyVMIRunning(client, vmiName)
				verifyVMINodePlacement(vmi, workloadsNode.Name)
				verifyVMIDeletion(client, vmiName)
			}
		})
	})

})

func verifyVMINodePlacement(vmi *kubevirtv1.VirtualMachineInstance, workloadNodeName string) {
	By("Verifying node placement of VMI")
	ExpectWithOffset(1, vmi.Labels["kubevirt.io/nodeName"]).Should(Equal(workloadNodeName))
	fmt.Fprintf(GinkgoWriter, "The VMI is running on the right node: %s\n", workloadNodeName)
}
