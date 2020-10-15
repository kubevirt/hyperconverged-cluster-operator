package tests_test

import (
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"time"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	testscore "kubevirt.io/kubevirt/tests"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
)

const timeout = 360 * time.Second
const pollingInterval = 5 * time.Second

var _ = Describe("Virtual Machines", func() {
	tests.FlagParse()
	client, err := kubecli.GetKubevirtClient()
	testscore.PanicOnError(err)

	var (
		workloadsNode      *corev1.Node
		checkNodePlacement = false
	)
	workloadsNodes, err := client.CoreV1().Nodes().List(k8smetav1.ListOptions{
		LabelSelector: "node.kubernetes.io/hco-test-node-type==workloads",
	})

	if err == nil && workloadsNodes != nil && len(workloadsNodes.Items) > 0 {
		checkNodePlacement = true
		workloadsNode = &workloadsNodes.Items[0]

		fmt.Fprintf(GinkgoWriter, "Found Workloads Node. Node name: %s; node labels:\n", workloadsNode.Name)
		w := json.NewEncoder(GinkgoWriter)
		w.SetIndent("", "  ")

		w.Encode(workloadsNode.Labels)
	}

	BeforeEach(func() {
		tests.BeforeEach()
	})

	Context("vmi testing", func() {
		for i := 0; i < 20; i++ {
			It(fmt.Sprintf("should create, verify and delete a vmi; run #%d", i), func() {
				vmi := testscore.NewRandomVMI()
				vmiName := vmi.Name
				Eventually(func() error {
					_, err := client.VirtualMachineInstance(testscore.NamespaceTestDefault).Create(vmi)
					return err
				}, timeout, pollingInterval).Should(Not(HaveOccurred()), "failed to create a vmi")
				Eventually(func() bool {
					vmi, err = client.VirtualMachineInstance(testscore.NamespaceTestDefault).Get(vmiName, &k8smetav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if vmi.Status.Phase == kubevirtv1.Running && checkNodePlacement {
						Expect(vmi.Labels["kubevirt.io/nodeName"]).Should(Equal(workloadsNode.Name))
						fmt.Fprintf(GinkgoWriter, "The VMI is running on the right node: %s\n", workloadsNode.Name)
					}
					return vmi.Status.Phase == kubevirtv1.Running
				}, timeout, pollingInterval).Should(BeTrue(), "failed to get the vmi Running")
				Eventually(func() error {
					err := client.VirtualMachineInstance(testscore.NamespaceTestDefault).Delete(vmiName, &k8smetav1.DeleteOptions{})
					return err
				}, timeout, pollingInterval).Should(Not(HaveOccurred()), "failed to delete a vmi")
			})
		}
	})
})
