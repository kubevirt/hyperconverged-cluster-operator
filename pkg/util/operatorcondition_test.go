package util

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("OperatorCondition", func() {
	noErrorFixtures := []struct {
		name        string
		clusterInfo ClusterInfo
	}{
		{
			name: "should no-op when not managed by OLM",
			clusterInfo: &ClusterInfoImp{
				managedByOLM:   false,
				runningLocally: false,
			},
		},
		{
			name: "should no-op when running locally",
			clusterInfo: &ClusterInfoImp{
				managedByOLM:   true,
				runningLocally: true,
			},
		},
	}

	for i := range noErrorFixtures {
		fix := noErrorFixtures[i]
		It(fix.name, func() {
			oc, err := NewOperatorCondition(
				fix.clusterInfo, nil, UpgradableCondition)
			Expect(err).To(BeNil())

			ctx := context.Background()
			err = oc.Set(ctx, metav1.ConditionTrue)
			Expect(err).To(BeNil())
		})
	}
})

func TestOperatorCondition(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OperatorCondition Suite")
}
