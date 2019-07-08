package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	framework "github.com/operator-framework/operator-sdk/pkg/test"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis"
	opv1alpha1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1alpha1"
	// . "github.com/kubevirt/hyperconverged-cluster-operator/test/check"
	// . "github.com/kubevirt/hyperconverged-cluster-operator/test/operations"
	// . "github.com/kubevirt/hyperconverged-cluster-operator/test/releases"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Lifecycle Test Suite")
}

var _ = BeforeSuite(func() {
	By("Adding custom resource scheme to framework")
	err := framework.AddToFrameworkScheme(apis.AddToScheme, &opv1alpha1.HyperConvergedList{})
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	By("Performing cleanup")
	// if GetConfig() != nil {
	// 	DeleteConfig()
	// }
	// CheckComponentsRemoval(AllComponents)
	// for _, release := range Releases() {
	// 	UninstallRelease(release)
	// }
})
