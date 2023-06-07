package operands

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("Dashboard tests", func() {

	schemeForTest := commontestutils.GetScheme()

	var (
		logger            = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("dashboard_test")
		testFilesLocation = getTestFilesLocation() + "/dashboards"
		hco               = commontestutils.NewHco()
	)

	Context("test dashboardHandlers", func() {
		It("should use env var to override the yaml locations", func() {
			// create temp folder for the test
			dir := path.Join(os.TempDir(), fmt.Sprint(time.Now().UTC().Unix()))
			_ = os.Setenv(dashboardManifestLocationVarName, dir)

			By("folder not exists", func() {
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := getDashboardHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			Expect(os.Mkdir(dir, 0744)).To(Succeed())
			defer os.RemoveAll(dir)

			By("folder is empty", func() {
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := getDashboardHandlers(logger, cli, schemeForTest, hco)

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
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := getDashboardHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			Expect(
				commontestutils.CopyFile(path.Join(dir, "dashboard.yaml"), path.Join(testFilesLocation, "kubevirt-top-consumers.yaml")),
			).To(Succeed())

			By("yaml file exists", func() {
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := getDashboardHandlers(logger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(HaveLen(1))
			})
		})

		It("should return error if dashboard path is not a directory", func() {
			filePath := "/testFiles/dashboards/kubevirt-top-consumers.yaml"
			const currentDir = "/controllers/operands"
			wd, _ := os.Getwd()
			if !strings.HasSuffix(wd, currentDir) {
				filePath = wd + currentDir + filePath
			} else {
				filePath = wd + filePath
			}

			_ = os.Setenv(dashboardManifestLocationVarName, filePath)
			By("check that getDashboardHandlers returns error", func() {
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := getDashboardHandlers(logger, cli, schemeForTest, hco)

				Expect(err).Should(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})
		})
	})

	Context("test dashboardHandler", func() {

		var exists *corev1.ConfigMap = nil
		It("should create the Dashboard Configmap resource if not exists", func() {
			_ = os.Setenv(dashboardManifestLocationVarName, testFilesLocation)

			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := getDashboardHandlers(logger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			{ // for the next test
				handler, ok := handlers[0].(*genericOperand)
				Expect(ok).To(BeTrue())
				hooks, ok := handler.hooks.(*cmHooks)
				Expect(ok).To(BeTrue())
				exists = hooks.required.DeepCopy()
			}

			hco := commontestutils.NewHco()
			By("apply the configmap", func() {
				req := commontestutils.NewReq(hco)
				res := handlers[0].ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				cms := &corev1.ConfigMapList{}
				Expect(cli.List(context.TODO(), cms)).To(Succeed())
				Expect(cms.Items).To(HaveLen(1))
				Expect(cms.Items[0].Name).Should(Equal("grafana-dashboard-kubevirt-top-consumers"))
			})
		})

		It("should update the ConfigMap resource if not not equal to the expected one", func() {

			Expect(exists).ToNot(BeNil(), "Must run the previous test first")
			exists.Data = map[string]string{"fakeKey": "fakeValue"}

			_ = os.Setenv(dashboardManifestLocationVarName, testFilesLocation)

			cli := commontestutils.InitClient([]client.Object{exists})
			handlers, err := getDashboardHandlers(logger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			hco := commontestutils.NewHco()
			By("apply the confimap", func() {
				req := commontestutils.NewReq(hco)
				res := handlers[0].ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())

				cmList := &corev1.ConfigMapList{}
				Expect(cli.List(context.TODO(), cmList)).To(Succeed())
				Expect(cmList.Items).To(HaveLen(1))
				Expect(cmList.Items[0].Name).Should(Equal("grafana-dashboard-kubevirt-top-consumers"))

				// check that data is reconciled
				_, ok := cmList.Items[0].Data["kubevirt-top-consumers.json"]
				Expect(ok).Should(BeTrue())
			})
		})
	})
})
