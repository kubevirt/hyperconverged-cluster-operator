package nodes

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("test the hyperconvergedPredicate predicate", func() {
	var predicate *hyperconvergedPredicate

	BeforeEach(func() {
		predicate = &hyperconvergedPredicate{}
	})

	DescribeTable("should return false", func(e event.TypedUpdateEvent[*hcov1.HyperConverged]) {
		Expect(predicate.Update(e)).To(BeFalse())
	},
		Entry("when no node placement is defined for workloads no nodePlacements", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{},
				},
			},
		}),
		Entry("when only new empty NP is defined", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{},
				},
			},
		}),
		Entry("when only old empty NP is defined", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
		}),
		Entry("when empty NP is defined in both", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
		}),
		Entry("when only new Infra NP is defined", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
		}),
		Entry("when only old Infra NP is defined", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
		}),
		Entry("when Infra NP is defined in both", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
		}),
		Entry("when same workload NP is defined in both", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
		}),
		Entry("when modified workload NP is defined in both, but the new is deleted", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewOtherNodePlacement(),
						},
					},
				},
			},
		}),
	)

	DescribeTable("should return true", func(e event.TypedUpdateEvent[*hcov1.HyperConverged]) {
		Expect(predicate.Update(e)).To(BeTrue())
	},
		Entry("when modified workload NP is defined in both", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewOtherNodePlacement(),
						},
					},
				},
			},
		}),
		Entry("when workload NP only defined in new", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
		}),
		Entry("when workload NP only defined in old", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
		}),
		Entry("when NP only defined in new", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{},
				},
			},
		}),
		Entry("when NP only defined in old", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
		}),
		Entry("when modified workload NP is defined in both, but the only old is deleted [not a real scenario]", event.TypedUpdateEvent[*hcov1.HyperConverged]{
			ObjectNew: &hcov1.HyperConverged{
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewNodePlacement(),
						},
					},
				},
			},
			ObjectOld: &hcov1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Workload: commontestutils.NewOtherNodePlacement(),
						},
					},
				},
			},
		}),
	)
})
