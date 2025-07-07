package handlers

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Dashboard tests", func() {

	schemeForTest := commontestutils.GetScheme()

	var (
		testLogger        = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("dashboard_test")
		testFilesLocation = getTestFilesLocation() + "/dashboards"
		hco               = commontestutils.NewHco()
	)

	Context("test dashboardHandlers", func() {
		It("should use env var to override the yaml locations", func() {
			// create temp folder for the test
			dir := path.Join(os.TempDir(), fmt.Sprint(time.Now().UTC().Unix()))
			_ = os.Setenv(DashboardManifestLocationVarName, dir)

			By("folder not exists", func() {
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			Expect(os.Mkdir(dir, 0744)).To(Succeed())
			defer os.RemoveAll(dir)

			By("folder is empty", func() {
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)

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
				handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(BeEmpty())
			})

			Expect(
				commontestutils.CopyFile(path.Join(dir, "dashboard.yaml"), path.Join(testFilesLocation, "kubevirt-top-consumers.yaml")),
			).To(Succeed())

			By("yaml file exists", func() {
				cli := commontestutils.InitClient([]client.Object{})
				handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(handlers).To(HaveLen(1))
			})
		})

		It("should return error if dashboard path is not a directory", func() {
			filePath := "/testFiles/dashboards/kubevirt-top-consumers.yaml"
			const currentDir = "/controllers/handlers"
			wd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			if !strings.HasSuffix(wd, currentDir) {
				filePath = wd + currentDir + filePath
			} else {
				filePath = wd + filePath
			}

			Expect(os.Setenv(DashboardManifestLocationVarName, filePath)).To(Succeed())
			By("check that GetDashboardHandlers returns error")
			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)

			Expect(err).To(HaveOccurred())
			Expect(handlers).To(BeEmpty())
		})
	})

	Context("test dashboardHandler", func() {
		It("should create the Dashboard Configmap resource if not exists", func() {
			_ = os.Setenv(DashboardManifestLocationVarName, testFilesLocation)

			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			hco := commontestutils.NewHco()
			By("apply the configmap", func() {
				req := commontestutils.NewReq(hco)
				res := handlers[0].Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				cms := &corev1.ConfigMapList{}
				Expect(cli.List(context.TODO(), cms)).To(Succeed())
				Expect(cms.Items).To(HaveLen(1))
				Expect(cms.Items[0].Name).To(Equal("grafana-dashboard-kubevirt-top-consumers"))
			})
		})

		It("should update the ConfigMap resource if not not equal to the expected one", func() {
			Expect(os.Setenv(DashboardManifestLocationVarName, testFilesLocation)).To(Succeed())

			exists, err := getCMsFromTestData(testFilesLocation)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).ToNot(BeNil())

			exists.Data = map[string]string{"fakeKey": "fakeValue"}

			cli := commontestutils.InitClient([]client.Object{exists})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			hco := commontestutils.NewHco()

			By("reconcile the confimap")
			req := commontestutils.NewReq(hco)
			res := handlers[0].Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			cmList := &corev1.ConfigMapList{}
			Expect(cli.List(context.TODO(), cmList)).To(Succeed())
			Expect(cmList.Items).To(HaveLen(1))

			cm := cmList.Items[0]
			Expect(cm.Name).To(Equal("grafana-dashboard-kubevirt-top-consumers"))
			Expect(cm.Data).ToNot(HaveKey("fakeKey"))

			// check that data is reconciled
			_, ok := cm.Data["kubevirt-top-consumers.json"]
			Expect(ok).To(BeTrue())
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"

			_ = os.Setenv(DashboardManifestLocationVarName, testFilesLocation)
			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			cmList := &corev1.ConfigMapList{}

			req := commontestutils.NewReq(hco)

			By("apply the CMs", func() {
				res := handlers[0].Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				Expect(cli.List(context.TODO(), cmList)).To(Succeed())
				Expect(cmList.Items).To(HaveLen(1))
				Expect(cmList.Items[0].Name).To(Equal("grafana-dashboard-kubevirt-top-consumers"))
			})

			expectedLabels := make(map[string]map[string]string)

			By("getting opinionated labels", func() {
				for _, cm := range cmList.Items {
					expectedLabels[cm.Name] = maps.Clone(cm.Labels)
				}
			})

			By("altering the cm objects", func() {
				for _, foundResource := range cmList.Items {
					for k, v := range expectedLabels[foundResource.Name] {
						foundResource.Labels[k] = "wrong_" + v
					}
					foundResource.Labels[userLabelKey] = userLabelValue
					err = cli.Update(context.TODO(), &foundResource)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			By("reconciling cm objects", func() {
				for _, handler := range handlers {
					res := handler.Ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Err).ToNot(HaveOccurred())
				}
			})

			foundResourcesList := &corev1.ConfigMapList{}
			Expect(cli.List(context.TODO(), foundResourcesList)).To(Succeed())

			for _, foundResource := range foundResourcesList.Items {
				for k, v := range expectedLabels[foundResource.Name] {
					Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
				}
				Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
			}
		})

		It("should reconcile managed labels to default on label deletion without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"

			_ = os.Setenv(DashboardManifestLocationVarName, testFilesLocation)
			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			cmList := &corev1.ConfigMapList{}

			req := commontestutils.NewReq(hco)

			By("apply the CMs", func() {
				res := handlers[0].Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				Expect(cli.List(context.TODO(), cmList)).To(Succeed())
				Expect(cmList.Items).To(HaveLen(1))
				Expect(cmList.Items[0].Name).To(Equal("grafana-dashboard-kubevirt-top-consumers"))
			})

			expectedLabels := make(map[string]map[string]string)

			By("getting opinionated labels", func() {
				for _, cm := range cmList.Items {
					expectedLabels[cm.Name] = maps.Clone(cm.Labels)
				}
			})

			By("altering the cm objects", func() {
				for _, foundResource := range cmList.Items {
					foundResource.Labels[userLabelKey] = userLabelValue
					delete(foundResource.Labels, hcoutil.AppLabelVersion)
					err = cli.Update(context.TODO(), &foundResource)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			By("reconciling cm objects", func() {
				for _, handler := range handlers {
					res := handler.Ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Err).ToNot(HaveOccurred())
				}
			})

			foundResourcesList := &corev1.ConfigMapList{}
			Expect(cli.List(context.TODO(), foundResourcesList)).To(Succeed())

			for _, foundResource := range foundResourcesList.Items {
				for k, v := range expectedLabels[foundResource.Name] {
					Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
				}
				Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
			}
		})
	})
})

func getCMsFromTestData(testFilesLocation string) (*corev1.ConfigMap, error) {
	dirEntries, err := os.ReadDir(testFilesLocation)
	if err != nil {
		return nil, err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		filePath := path.Join(testFilesLocation, entry.Name())
		return getCMFromTestData(filePath)
	}

	return nil, nil
}

func getCMFromTestData(filePath string) (*corev1.ConfigMap, error) {
	file, err := os.Open(filePath)

	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	return cmFromFile(file)
}
