package handlers

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/dirtest"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("QuickStart tests", func() {

	schemeForTest := commontestutils.GetScheme()

	var (
		testLogger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("quickstart_test")
		hco        = commontestutils.NewHco()
	)

	Context("test GetQuickStartHandlers", func() {
		It("should not create handlers if the folder does not exists", func() {
			cli := commontestutils.InitClient([]client.Object{})
			dir := dirtest.New()
			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(BeEmpty())
		})

		It("should not create handlers if the folder is empty", func() {
			cli := commontestutils.InitClient([]client.Object{})
			dir := dirtest.New(dirtest.WithDir(QuickStartDefaultManifestLocation))
			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(BeEmpty())

		})

		It("should not create handlers if the folder contains no yaml files", func() {
			cli := commontestutils.InitClient([]client.Object{})
			fileName := path.Join(QuickStartDefaultManifestLocation, "for_test.txt")
			dir := dirtest.New(dirtest.WithFile(fileName, []byte("some text")))
			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(BeEmpty())

		})

		It("should create handler if the folder contains a valid yaml file", func() {
			cli := commontestutils.InitClient([]client.Object{})
			fileName := path.Join(QuickStartDefaultManifestLocation, "quickStart.yaml")
			dir := dirtest.New(dirtest.WithFile(fileName, quickstartFileContent))

			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))
			Expect(quickstartNames).To(ContainElements("test-quick-start"))
		})

		It("should return error if quickstart path is not a directory", func() {
			cli := commontestutils.InitClient([]client.Object{})
			dir := dirtest.New(dirtest.WithFile(QuickStartDefaultManifestLocation, quickstartFileContent))

			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)

			Expect(err).To(HaveOccurred())
			Expect(handlers).To(BeEmpty())
		})
	})

	Context("test quickStartHandler", func() {
		var dir fs.FS

		BeforeEach(func() {
			fileName := path.Join(QuickStartDefaultManifestLocation, "test-quick-start.yaml")
			dir = dirtest.New(dirtest.WithFile(fileName, quickstartFileContent))
		})

		It("should create the ConsoleQuickStart resource if not exists", func() {
			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))
			Expect(quickstartNames).To(ContainElement("test-quick-start"))

			hco := commontestutils.NewHco()
			By("apply the quickstart CRs", func() {
				req := commontestutils.NewReq(hco)
				res := handlers[0].Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				quickstartObjects := &consolev1.ConsoleQuickStartList{}
				Expect(cli.List(context.TODO(), quickstartObjects)).To(Succeed())
				Expect(quickstartObjects.Items).To(HaveLen(1))
				Expect(quickstartObjects.Items[0].Name).To(Equal("test-quick-start"))
			})
		})

		It("should update the ConsoleQuickStart resource if not not equal to the expected one", func() {
			exists, err := getQSsFromTestData(dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).ToNot(BeNil())
			exists.Spec.DurationMinutes = exists.Spec.DurationMinutes * 2

			cli := commontestutils.InitClient([]client.Object{exists})
			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))
			Expect(quickstartNames).To(ContainElement("test-quick-start"))

			hco := commontestutils.NewHco()
			By("apply the quickstart CRs", func() {
				req := commontestutils.NewReq(hco)
				res := handlers[0].Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())

				quickstartObjects := &consolev1.ConsoleQuickStartList{}
				Expect(cli.List(context.TODO(), quickstartObjects)).To(Succeed())
				Expect(quickstartObjects.Items).To(HaveLen(1))
				Expect(quickstartObjects.Items[0].Name).To(Equal("test-quick-start"))
				// check that the existing object was reconciled
				Expect(quickstartObjects.Items[0].Spec.DurationMinutes).To(Equal(20))

				// ObjectReference should have been updated
				Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
				objectRefOutdated, err := reference.GetReference(schemeForTest, exists)
				Expect(err).ToNot(HaveOccurred())
				objectRefFound, err := reference.GetReference(schemeForTest, &quickstartObjects.Items[0])
				Expect(err).ToNot(HaveOccurred())
				Expect(hco.Status.RelatedObjects).To(Not(ContainElement(*objectRefOutdated)))
				Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRefFound))
			})
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"

			cli := commontestutils.InitClient([]client.Object{})
			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			quickstartObjects := &consolev1.ConsoleQuickStartList{}

			req := commontestutils.NewReq(hco)

			By("apply the quickstart CRs", func() {
				res := handlers[0].Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				Expect(cli.List(context.TODO(), quickstartObjects)).To(Succeed())
				Expect(quickstartObjects.Items).To(HaveLen(1))
				Expect(quickstartObjects.Items[0].Name).To(Equal("test-quick-start"))
			})

			expectedLabels := make(map[string]map[string]string)

			By("getting opinionated labels", func() {
				for _, quickstart := range quickstartObjects.Items {
					expectedLabels[quickstart.Name] = maps.Clone(quickstart.Labels)
				}
			})

			By("altering the quickstart objects", func() {
				for _, foundResource := range quickstartObjects.Items {
					for k, v := range expectedLabels[foundResource.Name] {
						foundResource.Labels[k] = "wrong_" + v
					}
					foundResource.Labels[userLabelKey] = userLabelValue
					err = cli.Update(context.TODO(), &foundResource)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			By("reconciling quickstart objects", func() {
				for _, handler := range handlers {
					res := handler.Ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Err).ToNot(HaveOccurred())
				}
			})

			foundResourcesList := &consolev1.ConsoleQuickStartList{}
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
			handlers, err := GetQuickStartHandlers(testLogger, cli, schemeForTest, hco, dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(handlers).To(HaveLen(1))

			quickstartObjects := &consolev1.ConsoleQuickStartList{}

			req := commontestutils.NewReq(hco)

			By("apply the quickstart CRs", func() {
				res := handlers[0].Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				Expect(cli.List(context.TODO(), quickstartObjects)).To(Succeed())
				Expect(quickstartObjects.Items).To(HaveLen(1))
				Expect(quickstartObjects.Items[0].Name).To(Equal("test-quick-start"))
			})

			expectedLabels := make(map[string]map[string]string)

			By("getting opinionated labels", func() {
				for _, quickstart := range quickstartObjects.Items {
					expectedLabels[quickstart.Name] = maps.Clone(quickstart.Labels)
				}
			})

			By("altering the quickstart objects", func() {
				for _, foundResource := range quickstartObjects.Items {
					delete(foundResource.Labels, hcoutil.AppLabelVersion)
					foundResource.Labels[userLabelKey] = userLabelValue
					err = cli.Update(context.TODO(), &foundResource)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			By("reconciling quickstart objects", func() {
				for _, handler := range handlers {
					res := handler.Ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Err).ToNot(HaveOccurred())
				}
			})

			foundResourcesList := &consolev1.ConsoleQuickStartList{}
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

func getQSsFromTestData(dir fs.FS) (*consolev1.ConsoleQuickStart, error) {
	dirEntries, err := fs.ReadDir(dir, QuickStartDefaultManifestLocation)
	if err != nil {
		return nil, err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		filePath := path.Join(QuickStartDefaultManifestLocation, entry.Name())
		return getQSFromTestData(dir, filePath)
	}

	return nil, nil
}

func getQSFromTestData(dir fs.FS, filePath string) (*consolev1.ConsoleQuickStart, error) {
	file, err := dir.Open(filePath)

	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	return quickStartFromFile(file)
}
