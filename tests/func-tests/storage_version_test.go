package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	k8sEtcdNamespace = "kube-system"
	k8sEtcdSelector  = "component=etcd"
	k8sEtcdPrefix    = "/registry"

	ocpEtcdNamespace     = "openshift-etcd"
	ocpEtcdSelector      = "app=etcd"
	ocpEtcdPrefix        = "/kubernetes.io"
	etcdctlContainerName = "etcdctl"
)

var _ = Describe("HyperConverged API v1 storage version", Label("v1storage"), func() {
	var (
		cli    client.Client
		cliSet *kubernetes.Clientset
	)

	BeforeEach(func() {
		cli = tests.GetControllerRuntimeClient()
		cliSet = tests.GetK8sClientSet()
	})

	It("CRD status.storedVersions must contain only v1", func(ctx context.Context) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		Expect(cli.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, crd)).To(Succeed())

		Expect(crd.Status.StoredVersions).To(Equal([]string{hcov1.APIVersionV1}),
			`expected storedVersions to be ["v1"], but got %v`, crd.Status.StoredVersions)
	})

	It("HyperConverged CR must be stored in v1 API version in etcd", func(ctx context.Context) {
		isOpenShift, err := tests.IsOpenShift(ctx, cli)
		Expect(err).ToNot(HaveOccurred())

		var (
			etcdNamespace, etcdSelector, etcdPrefix, container string
		)
		if isOpenShift {
			etcdNamespace = ocpEtcdNamespace
			etcdSelector = ocpEtcdSelector
			etcdPrefix = ocpEtcdPrefix
			container = etcdctlContainerName
		} else {
			etcdNamespace = k8sEtcdNamespace
			etcdSelector = k8sEtcdSelector
			etcdPrefix = k8sEtcdPrefix
		}

		etcdKey := fmt.Sprintf("%s/hco.kubevirt.io/hyperconvergeds/%s/%s", etcdPrefix, tests.InstallNamespace, hcoutil.HyperConvergedName)

		var etcdCmd []string
		if isOpenShift {
			etcdCmd = []string{"etcdctl", "get", etcdKey, "--print-value-only"}
		} else {
			etcdCmd = []string{"etcdctl", "get", etcdKey,
				"--cert=/etc/kubernetes/pki/etcd/server.crt",
				"--key=/etc/kubernetes/pki/etcd/server.key",
				"--endpoints=https://127.0.0.1:2379",
				"--print-value-only",
				"--insecure-skip-tls-verify",
			}
		}

		etcdPod, err := getEtcdPod(ctx, cliSet, etcdNamespace, etcdSelector)
		Expect(err).ToNot(HaveOccurred())

		stdout, stderr, err := tests.ExecuteUnwrappedCommandOnPod(ctx, cliSet, etcdPod, etcdCmd, container)
		Expect(err).ToNot(HaveOccurred(), "failed to exec etcdctl in pod %s/%s; stderr: %s", etcdPod.Namespace, etcdPod.Name, stderr)

		stdout = strings.TrimSpace(stdout)
		Expect(stdout).ToNot(BeEmpty(), "etcdctl returned empty value for key %s", etcdKey)

		type versionExtract struct {
			APIVersion string `json:"apiVersion"`
		}

		var obj versionExtract
		Expect(json.Unmarshal([]byte(stdout), &obj)).To(Succeed(),
			"failed to parse etcd value as JSON; raw value:\n%s", stdout)
		Expect(obj.APIVersion).To(Equal("hco.kubevirt.io/v1"),
			"expected the HyperConverged CR in etcd to have apiVersion hco.kubevirt.io/v1, but got %s", obj.APIVersion)
	})
})

func getEtcdPod(ctx context.Context, cliSet *kubernetes.Clientset, namespace, labelSelector string) (*corev1.Pod, error) {
	GinkgoHelper()

	pods, err := cliSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list etcd pods in namespace %s with selector %s; %v", namespace, labelSelector, err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no etcd pods found in namespace %s with selector %s", namespace, labelSelector)
	}

	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
			continue
		}

		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return &pod, nil
			}
		}
	}

	return nil, fmt.Errorf("no etcd pod is ready")
}
