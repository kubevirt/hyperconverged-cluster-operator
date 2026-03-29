package annotate

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("test annotation - importsToKeep", func() {
	Context("importsToKeep should default to 1", func() {
		var assetsDir string
		var outputDir string
		var cfg *config.Config

		const tmplFile = "dataImportCronTemplates.yaml"

		BeforeEach(func() {
			assetsDir = GinkgoT().TempDir()
			outputDir = GinkgoT().TempDir()

			cfg = config.GetConfig()
			cfg.DictDir = assetsDir
			cfg.OutputFileName = fmt.Sprintf("%s/%s", outputDir, "output.yaml")

			input := []byte(`- metadata:
    annotations:
      cdi.kubevirt.io/storage.bind.immediate.requested: "true"
      ssp.kubevirt.io/dict.architectures: amd64,arm64,s390x
    name: centos-stream9-image-cron
  spec:
    garbageCollect: Outdated
    importsToKeep: 1
    managedDataSource: centos-stream9
    schedule: "0 */12 * * *"
    template:
      spec:
        source:
          registry:
            url: docker://quay.io/containerdisks/centos-stream:9
        storage:
          resources:
            requests:
              storage: 10Gi
`)
			Expect(os.WriteFile(fmt.Sprintf("%s/%s", assetsDir, tmplFile), input, 0600)).To(Succeed())
		})

		It("importsToKeep should get updated", func() {
			cfg.ImportsToKeep = 2
			entries, err := os.ReadDir(assetsDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			outputFile := createFile()
			defer outputFile.Close()

			AnnotateOneFile(context.TODO(), nil, entries[0], outputFile)

			outputData, err := os.ReadFile(cfg.OutputFileName)
			outputString := string(outputData)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputString).To(ContainSubstring("importsToKeep: 4"))
			Expect(outputString).To(ContainSubstring("name: centos-stream9-image-cron"))
			Expect(outputString).To(ContainSubstring("cdi.kubevirt.io/storage.bind.immediate.requested: \"true\""))
			Expect(outputString).To(ContainSubstring("ssp.kubevirt.io/dict.architectures: amd64,arm64,s390x"))
			Expect(outputString).To(ContainSubstring("schedule: \"0 */12 * * *\""))
		})
	})
})

func createFile() *os.File {
	targetFile, err := os.Create(config.OutputFileName())
	if err != nil {
		PrintErrorAndExit("error creating file %s: %v", config.OutputFileName(), err)
	}
	return targetFile
}

func TestAnnotation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "annotation suite")
}
