package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"kubevirt.io/kubevirt/tests/flags"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kvtutil "kubevirt.io/kubevirt/tests/util"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
	v1 "k8s.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

const (
	nodePlacementSamplingSize = 20
	hcoLabel                  = "node.kubernetes.io/hco-test-node-type"
	infra                     = "infra"
	workloads                 = "workloads"
	group                     = "hco.kubevirt.io"
	version                   = "v1beta1"
	kind                      = "HyperConverged"
	hcoResource               = "hyperconvergeds"
	name                      = "kubevirt-hyperconverged"
)

var _ = Describe("[rfe_id:4356][crit:medium][vendor:cnv-qe@redhat.com][level:system]Node Placement", Ordered, func() {
	var workloadsNode *v1.Node

	tests.FlagParse()
	client, err := kubecli.GetKubevirtClient()
	kvtutil.PanicOnError(err)

	// store the existing HCO CR obtained in BeforeALl and revert in AfterAll stage
	originalHco := &hcov1beta1.HyperConverged{}

	BeforeAll(func() {
		nodes, err := client.CoreV1().Nodes().List(context.TODO(), k8smetav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
		kvtutil.PanicOnError(err)
		totalNodes := len(nodes.Items)

		// Label all but first node with "node.kubernetes.io/hco-test-node-type=infra"
		// We are doing this to remove dependency of this Describe block on a shell script that
		// labels the nodes this way
		for i := 0; i < totalNodes-1; i++ {
			err = setHcoNodeTypeLabel(client, &nodes.Items[i], infra)
			kvtutil.PanicOnError(err)
		}
		// Label the last node with "node.kubernetes.io/hco-test-node-type=workloads"
		err = setHcoNodeTypeLabel(client, &nodes.Items[totalNodes-1], workloads)
		kvtutil.PanicOnError(err)

		Expect(client.RestClient().
			Get().
			Resource(hcoResource).
			Name(name).
			Namespace(flags.KubeVirtInstallNamespace).
			AbsPath("/apis", hcov1beta1.SchemeGroupVersion.Group, hcov1beta1.SchemeGroupVersion.Version).
			Timeout(10 * time.Second).
			Do(context.TODO()).
			Into(originalHco),
		).To(Succeed())
		// modify the "infra" and "workloads" keys
		infraVal := hcov1beta1.HyperConvergedConfig{NodePlacement: &sdkapi.NodePlacement{}}
		workloadsVal := hcov1beta1.HyperConvergedConfig{NodePlacement: &sdkapi.NodePlacement{}}

		infraVal.NodePlacement.NodeSelector = map[string]string{hcoLabel: infra}
		workloadsVal.NodePlacement.NodeSelector = map[string]string{hcoLabel: workloads}

		hco := &hcov1beta1.HyperConverged{}
		originalHco.DeepCopyInto(hco)

		hco.Spec.Infra = infraVal
		hco.Spec.Workloads = workloadsVal
		hco.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		})
		r := client.RestClient().Put().
			Resource(hcoResource).
			Name(name).
			Namespace(flags.KubeVirtInstallNamespace).
			AbsPath("/apis", hcov1beta1.SchemeGroupVersion.Group, hcov1beta1.SchemeGroupVersion.Version).
			Timeout(10 * time.Second).
			Body(hco).
			Do(context.TODO())
		Expect(r.Error()).ToNot(HaveOccurred())

		workloadsNode = &nodes.Items[0]
		fmt.Fprintf(GinkgoWriter, "Found Workloads Node. Node name: %s; node labels:\n", workloadsNode.Name)
		w := json.NewEncoder(GinkgoWriter)
		w.SetIndent("", "  ")
		_ = w.Encode(workloadsNode.Labels)
	})

	AfterAll(func() {
		// undo the modification to HCO CR done in BeforeAll stage
		originalHco.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		})
		r := client.RestClient().Put().
			Resource(hcoResource).
			Name(name).
			Namespace(flags.KubeVirtInstallNamespace).
			AbsPath("/apis", hcov1beta1.SchemeGroupVersion.Group, hcov1beta1.SchemeGroupVersion.Version).
			Timeout(10 * time.Second).
			Body(originalHco).
			Do(context.TODO())
		Expect(r.Error()).ToNot(HaveOccurred())

		// unlabel the nodes
		nodes, err := client.CoreV1().Nodes().List(context.TODO(), k8smetav1.ListOptions{LabelSelector: hcoLabel})
		kvtutil.PanicOnError(err)
		for i := 0; i < len(nodes.Items); i++ {
			node := &nodes.Items[i]
			labels := node.GetLabels()
			delete(labels, hcoLabel)
			node, err = client.CoreV1().Nodes().Get(context.TODO(), node.Name, k8smetav1.GetOptions{})
			kvtutil.PanicOnError(err)
			node.SetLabels(labels)
			_, err = client.CoreV1().Nodes().Update(context.TODO(), node, k8smetav1.UpdateOptions{})
			kvtutil.PanicOnError(err)
		}
	})

	BeforeEach(func() {
		tests.BeforeEach()
	})

	Context("validate node placement in workloads nodes", func() {
		It("[test_id:5677] all expected 'workloads' pod must be on infra node", func() {
			expectedWorkloadsPods := map[string]bool{
				"bridge-marker": false,
				"cni-plugins":   false,
				// "kube-multus":     false,
				"ovs-cni-marker": false,
				"virt-handler":   false,
				"secondary-dns":  false,
			}

			By("Getting Network Addons Configs")
			cnaoCR := getNetworkAddonsConfigs(client)
			if cnaoCR.Spec.Ovs == nil {
				delete(expectedWorkloadsPods, "ovs-cni-marker")
			}
			if cnaoCR.Spec.KubeSecondaryDNS == nil {
				delete(expectedWorkloadsPods, "secondary-dns")
			}

			By("Listing pods in infra node")
			pods := listPodsInNode(client, workloadsNode.Name)

			By("Collecting nodes of pods")
			updatePodAssignments(pods, expectedWorkloadsPods, "workload", workloadsNode.Name)

			By("Verifying that all expected workload pods exist in workload nodes")
			Expect(expectedWorkloadsPods).ToNot(ContainElement(false))
		})
	})

	Context("validate node placement on infra nodes", func() {
		It("[test_id:5678] all expected 'infra' pod must be on infra node", func() {
			expectedInfraPods := map[string]bool{
				"cdi-apiserver":   false,
				"cdi-controller":  false,
				"cdi-uploadproxy": false,
				"manager":         false,
				"virt-api":        false,
				"virt-controller": false,
			}

			By("Listing infra nodes")
			infraNodes := listInfraNodes(client)

			for _, node := range infraNodes.Items {
				By("Listing pods in " + node.Name)
				pods := listPodsInNode(client, node.Name)

				By("Collecting nodes of pods")
				updatePodAssignments(pods, expectedInfraPods, "infra", node.Name)
			}

			By("Verifying that all expected infra pods exist in infra nodes")
			Expect(expectedInfraPods).ToNot(ContainElement(false))
		})
	})
})

func updatePodAssignments(pods *v1.PodList, podMap map[string]bool, nodeType string, nodeName string) {
	for _, pod := range pods.Items {
		podName := pod.Spec.Containers[0].Name
		fmt.Fprintf(GinkgoWriter, "Found %s pod '%s' in the '%s' node %s\n", podName, pod.Name, nodeType, nodeName)
		if found, ok := podMap[podName]; ok {
			if !found {
				podMap[podName] = true
			}
		}
	}
}

func listPodsInNode(client kubecli.KubevirtClient, nodeName string) *v1.PodList {
	pods, err := client.CoreV1().Pods(flags.KubeVirtInstallNamespace).List(context.TODO(), k8smetav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	return pods
}

func listInfraNodes(client kubecli.KubevirtClient) *v1.NodeList {
	infraNodes, err := client.CoreV1().Nodes().List(context.TODO(), k8smetav1.ListOptions{
		LabelSelector: "node.kubernetes.io/hco-test-node-type==infra",
	})
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	return infraNodes
}

func getNetworkAddonsConfigs(client kubecli.KubevirtClient) *networkaddonsv1.NetworkAddonsConfig {
	var cnaoCR networkaddonsv1.NetworkAddonsConfig

	s := scheme.Scheme
	_ = networkaddonsv1.AddToScheme(s)
	s.AddKnownTypes(networkaddonsv1.GroupVersion)

	ExpectWithOffset(1, client.RestClient().Get().
		Resource("networkaddonsconfigs").
		Name("cluster").
		AbsPath("/apis", networkaddonsv1.GroupVersion.Group, networkaddonsv1.GroupVersion.Version).
		Timeout(10*time.Second).
		Do(context.TODO()).Into(&cnaoCR)).To(Succeed())

	return &cnaoCR
}

func setHcoNodeTypeLabel(client kubecli.KubevirtClient, node *v1.Node, value string) error {
	labels := node.GetLabels()
	labels[hcoLabel] = value
	node, err := client.CoreV1().Nodes().Get(context.TODO(), node.Name, k8smetav1.GetOptions{})
	if err != nil {
		return err
	}
	node.SetLabels(labels)
	_, err = client.CoreV1().Nodes().Update(context.TODO(), node, k8smetav1.UpdateOptions{})
	return err
}
