package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kvtutil "kubevirt.io/kubevirt/tests/util"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/tests/flags"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	nodePlacementSamplingSize = 20
	hcoLabel                  = "node.kubernetes.io/hco-test-node-type"
	infra                     = "infra"
	workloads                 = "workloads"
	pathToFile                = "../../_out"
	fileName                  = "hco.cr.yaml"
	deployPath                = "../../deploy"
	group                     = "hco.kubevirt.io"
	version                   = "v1beta1"
	kind                      = "HyperConverged"
	resource                  = "hyperconvergeds"
	namespace                 = "kubevirt-hyperconverged"
)

var _ = Describe("[rfe_id:4356][crit:medium][vendor:cnv-qe@redhat.com][level:system]Node Placement", Ordered, func() {
	var workloadsNode *v1.Node
	infraVal := map[string]interface{}{
		"nodePlacement": map[string]interface{}{
			"nodeSelector": map[string]interface{}{
				hcoLabel: infra,
			},
		},
	}
	workloadsVal := map[string]interface{}{
		"nodePlacement": map[string]interface{}{
			"nodeSelector": map[string]interface{}{
				hcoLabel: workloads,
			},
		},
	}

	tests.FlagParse()
	client, err := kubecli.GetKubevirtClient()
	kvtutil.PanicOnError(err)
	dynamicClient, err := dynamic.NewForConfig(client.Config())
	kvtutil.PanicOnError(err)

	var hco unstructured.Unstructured

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

		// read the hco CR
		file, err := os.ReadFile(filepath.Join(deployPath, fileName))
		kvtutil.PanicOnError(err)
		yamlFile := make(map[string]interface{})
		err = yaml.Unmarshal(file, &yamlFile)
		kvtutil.PanicOnError(err)
		// get the "spec"
		data := yamlFile["spec"].(map[string]interface{})
		// modify the "infra" and "workloads" keys
		data["infra"] = infraVal
		data["workloads"] = workloadsVal
		file, err = yaml.Marshal(yamlFile)
		kvtutil.PanicOnError(err)
		// create directory "_out" if it doesn't already exist
		if err = os.Mkdir(pathToFile, os.ModePerm); !os.IsExist(err) {
			kvtutil.PanicOnError(err)
		}
		// write the modified yaml
		err = os.WriteFile(filepath.Join(pathToFile, fileName), file, os.ModePerm)
		kvtutil.PanicOnError(err)
		// use the same yaml to create resources on the cluster
		hco = unstructured.Unstructured{Object: yamlFile}
		hco.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		})
		r, err := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: resource,
		}).Namespace(namespace).Create(context.TODO(), &hco, k8smetav1.CreateOptions{})
		kvtutil.PanicOnError(err)
		err = wait.PollImmediate(5*time.Second, 1200*time.Second, func() (bool, error) {
			obj, err := dynamicClient.Resource(schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource,
			}).Namespace(namespace).Get(context.TODO(), r.GetName(), k8smetav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if obj != nil {
				return true, nil
			}
			return false, nil
		})
		kvtutil.PanicOnError(err)
		workloadsNode = &nodes.Items[0]
		fmt.Fprintf(GinkgoWriter, "Found Workloads Node. Node name: %s; node labels:\n", workloadsNode.Name)
		w := json.NewEncoder(GinkgoWriter)
		w.SetIndent("", "  ")
		_ = w.Encode(workloadsNode.Labels)
	})

	AfterAll(func() {
		// remove the HCO CR created in BeforeAll step
		err = dynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: resource,
		}).Namespace(namespace).Delete(context.TODO(), hco.GetName(), k8smetav1.DeleteOptions{})
		kvtutil.PanicOnError(err)

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
