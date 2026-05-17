package tests

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func Dump(ctx context.Context) {
	artifactDir := os.Getenv("ARTIFACT_DIR")
	if artifactDir == "" {
		ginkgo.GinkgoLogr.Info("the ARTIFACT_DIR env var is not defined. Skipping dump")
		return
	}

	endTime := time.Now()

	r := ginkgo.CurrentSpecReport()
	startTime := r.StartTime

	dir := path.Join(r.ContainerHierarchyTexts...)
	dir = path.Join(dir, r.LeafNodeText)
	dir = strings.ReplaceAll(dir, " ", "_")
	dir = path.Join(artifactDir, dir)

	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "can't create directory", "directory name", dir)
		return
	}

	dumpPods(ctx, dir, startTime, endTime)
	dumpHyperConverged(ctx, dir)
}

func dumpPods(ctx context.Context, dir string, _, _ time.Time) {
	cli := GetK8sClientSet()

	pods, err := cli.CoreV1().Pods(InstallNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "can't list pods")
		return
	}

	podsFileName := path.Join(dir, "pods.json")
	f, err := os.Create(podsFileName)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "can't create pods file")
		return
	}

	defer f.Close()
	dec := json.NewDecoder(f)

	err = dec.Decode(&pods.Items)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "can't write pods json to the file")
	}
}

func dumpHyperConverged(ctx context.Context, dir string) {
	cli := GetControllerRuntimeClient()
	hc := &hcov1.HyperConverged{}

	err := cli.Get(ctx, client.ObjectKey{Namespace: InstallNamespace, Name: hcoutil.HyperConvergedName}, hc)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "can't read the HyperConverged CR")
	}

	podsFileName := path.Join(dir, "hyperconverged.yaml")
	f, err := os.Create(podsFileName)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "can't create the hyperconverged file")
		return
	}
	defer f.Close()

	err = yaml.NewEncoder(f).Encode(hc)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "can't write the hyperconverged file")
	}
}
