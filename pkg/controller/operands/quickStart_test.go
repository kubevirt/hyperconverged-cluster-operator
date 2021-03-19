package operands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/commonTestUtils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("QuickStart tests", func() {

	schemeForTest := commonTestUtils.GetScheme()

	var (
		logger            = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("quickstart_test")
		testFilesLocation = getTestFilesLocation() + "/quickstarts"
		hco               = commonTestUtils.NewHco()
	)

	Context("test checkCrdExists", func() {
		It("should return not-stop-processing if not exists", func() {
			cli := commonTestUtils.InitClient([]runtime.Object{})

			err := checkCrdExists(context.TODO(), cli, logger)
			Expect(err).Should(HaveOccurred())
			Expect(errors.Unwrap(err)).To(BeNil())
		})

		It("should return true if CRD exists, with no error", func() {
			cli := commonTestUtils.InitClient([]runtime.Object{qsCrd})

			err := checkCrdExists(context.TODO(), cli, logger)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("test getQuickStartHandlers", func() {
		It("should use env var to override the yaml locations", func() {
			// create temp folder for the test
			dir := path.Join(os.TempDir(), fmt.Sprint(time.Now().UTC().Unix()))
			_ = os.Setenv(quickStartManifestLocationVarName, dir)
			By("CRD is not deployed", func() {
				cli := commonTestUtils.InitClient([]runtime.Object{})
				handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			By("folder not exists", func() {
				cli := commonTestUtils.InitClient([]runtime.Object{qsCrd})
				handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			err := os.Mkdir(dir, 0744)
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(dir)

			By("folder is empty", func() {
				cli := commonTestUtils.InitClient([]runtime.Object{qsCrd})
				handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			nonYaml, err := os.OpenFile(path.Join(dir, "for_test.txt"), os.O_CREATE|os.O_WRONLY, 0644)
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(nonYaml.Name())

			_, err = fmt.Fprintln(nonYaml, `some text`)
			Expect(err).ToNot(HaveOccurred())
			_ = nonYaml.Close()

			By("no yaml files", func() {
				cli := commonTestUtils.InitClient([]runtime.Object{qsCrd})
				handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			err = copyFile(path.Join(dir, "quickStart.yaml"), path.Join(testFilesLocation, "quickstart.yaml"))
			Expect(err).ToNot(HaveOccurred())

			By("yaml file exists", func() {
				cli := commonTestUtils.InitClient([]runtime.Object{qsCrd})
				handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(HaveLen(1))
			})
		})

		It("should return error if quickstart path is not a directory", func() {
			filePath := "/testFiles/quickstarts/quickstart.yaml"
			const currentDir = "/pkg/controller/operands"
			wd, _ := os.Getwd()
			if !strings.HasSuffix(wd, currentDir) {
				filePath = wd + currentDir + filePath
			} else {
				filePath = wd + filePath
			}

			By("check that validateQuickstartDir return wrapped error", func() {
				err := validateQuickstartDir(filePath)
				Expect(err).Should(HaveOccurred())
				Expect(errors.Unwrap(err)).ShouldNot(BeNil())
			})

			// quickstart directory path of a file
			_ = os.Setenv(quickStartManifestLocationVarName, filePath)
			By("check that getQuickStartHandlers returns error", func() {
				cli := commonTestUtils.InitClient([]runtime.Object{qsCrd})
				handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)

				Expect(err).Should(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})
		})
	})

	Context("test quickStartHandler", func() {

		var exists *consolev1.ConsoleQuickStart = nil
		It("should create the ConsoleQuickStart resource if not exists", func() {
			_ = os.Setenv(quickStartManifestLocationVarName, testFilesLocation)

			cli := commonTestUtils.InitClient([]runtime.Object{qsCrd})
			handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			{ // for the next test
				handler, ok := handlers[0].(*genericOperand)
				Expect(ok).To(BeTrue())
				hooks, ok := handler.hooks.(*qsHooks)
				Expect(ok).To(BeTrue())
				exists = hooks.required.DeepCopy()
			}

			hco := commonTestUtils.NewHco()
			By("apply the quickstart CRs", func() {
				req := commonTestUtils.NewReq(hco)
				res := handlers[0].ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				quickstartObjects := &consolev1.ConsoleQuickStartList{}
				err := cli.List(context.TODO(), quickstartObjects)
				Expect(err).ToNot(HaveOccurred())
				Expect(quickstartObjects.Items).To(HaveLen(1))
				Expect(quickstartObjects.Items[0].Name).Should(Equal("test-quick-start"))
			})
		})

		It("should update the ConsoleQuickStart resource if not not equal to the expected one", func() {

			Expect(exists).ToNot(BeNil(), "Must run the previous test first")
			exists.Spec.DurationMinutes = exists.Spec.DurationMinutes * 2

			_ = os.Setenv(quickStartManifestLocationVarName, testFilesLocation)

			cli := commonTestUtils.InitClient([]runtime.Object{qsCrd, exists})
			handlers, err := getQuickStartHandlers(logger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			hco := commonTestUtils.NewHco()
			By("apply the quickstart CRs", func() {
				req := commonTestUtils.NewReq(hco)
				res := handlers[0].ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())

				quickstartObjects := &consolev1.ConsoleQuickStartList{}
				err := cli.List(context.TODO(), quickstartObjects)
				Expect(err).ToNot(HaveOccurred())
				Expect(quickstartObjects.Items).To(HaveLen(1))
				Expect(quickstartObjects.Items[0].Name).Should(Equal("test-quick-start"))
				// check that the existing object was reconciled
				Expect(quickstartObjects.Items[0].Spec.DurationMinutes).Should(Equal(20))
			})
		})
	})
})

func copyFile(dest, src string) error {
	fin, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fin.Close()

	fout, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer fout.Close()

	_, err = io.Copy(fout, fin)

	if err != nil {
		return err
	}
	return nil
}
