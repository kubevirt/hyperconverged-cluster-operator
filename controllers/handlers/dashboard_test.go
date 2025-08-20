package handlers

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"strings"
	"testing/fstest"

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
		testLogger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("dashboard_test")
		hco        = commontestutils.NewHco()
	)

	Context("test dashboardHandlers", func() {
		It("should create dashboard handlers", func() {
			// create temp folder name for the test
			dir := fstest.MapFS{}

			By("folder not exists")
			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(BeEmpty())

			By("folder is empty")
			dir[DashboardManifestLocationDefault] = &fstest.MapFile{
				Mode: fs.ModeDir,
			}
			cli = commontestutils.InitClient([]client.Object{})
			handlers, err = GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(BeEmpty())

			By("no yaml files")
			dir[path.Join(DashboardManifestLocationDefault, "for_test.txt")] = &fstest.MapFile{
				Data: []byte("some text"),
			}
			cli = commontestutils.InitClient([]client.Object{})
			handlers, err = GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(BeEmpty())

			By("yaml file exists")
			cli = commontestutils.InitClient([]client.Object{})
			dir[path.Join(DashboardManifestLocationDefault, "kubevirt-top-consumers.yaml")] = &fstest.MapFile{
				Data: kubevirtTopConsumersFileContent,
			}
			handlers, err = GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))
		})
	})

	Context("test dashboardHandler", func() {
		var dir fs.FS

		BeforeEach(func() {
			dir = fstest.MapFS{
				DashboardManifestLocationDefault: &fstest.MapFile{
					Mode: fs.ModeDir,
				},
				path.Join(DashboardManifestLocationDefault, "kubevirt-top-consumers.yaml"): &fstest.MapFile{
					Data: kubevirtTopConsumersFileContent,
				},
			}
		})

		It("should create the Dashboard Configmap resource if not exists", func() {
			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)
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
			exists, err := getCMsFromTestData(dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).ToNot(BeNil())

			exists.Data = map[string]string{"fakeKey": "fakeValue"}

			cli := commontestutils.InitClient([]client.Object{exists})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)
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

			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)
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

			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetDashboardHandlers(testLogger, cli, schemeForTest, hco, dir)
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

func getCMsFromTestData(dir fs.FS) (*corev1.ConfigMap, error) {
	dirEntries, err := fs.ReadDir(dir, DashboardManifestLocationDefault)
	if err != nil {
		return nil, err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		filePath := path.Join(DashboardManifestLocationDefault, entry.Name())
		return getCMFromTestData(dir, filePath)
	}

	return nil, nil
}

func getCMFromTestData(dir fs.FS, filePath string) (*corev1.ConfigMap, error) {
	file, err := dir.Open(filePath)

	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	return cmFromFile(file)
}
