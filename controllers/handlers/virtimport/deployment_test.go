package virtimport

import (
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

var _ = Describe("virt-import Deployment", func() {
	var hco *hcov1.HyperConverged

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		setupImageEnv()
	})

	It("should take both images from the HCO env vars", func() {
		dep := newDeployment(getBundle().Deployment, hco)

		Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
		manager := dep.Spec.Template.Spec.Containers[0]
		Expect(manager.Name).To(Equal(virtimport.ManagerContainerName))
		Expect(manager.Image).To(Equal(testControllerImage))
		Expect(manager.Env).To(ContainElement(corev1.EnvVar{Name: virtimport.FetcherImageEnvName, Value: testFetcherImage}))
	})

	It("should default the image pull policy like the apiserver for the HCO image", func() {
		dep := newDeployment(getBundle().Deployment, hco)
		Expect(dep.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

		setEnv(hcoutil.VirtImportControllerImageEnvV, "quay.io/kubevirt/virt-import-controller:latest")
		dep = newDeployment(getBundle().Deployment, hco)
		Expect(dep.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))
	})

	It("should keep the image pull policy of the bundle", func() {
		bundleDeployment := getBundle().Deployment
		bundleDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever

		dep := newDeployment(bundleDeployment, hco)
		Expect(dep.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullNever))
	})

	It("should be deployed to the operator namespace with a stable HCO selector", func() {
		dep := newDeployment(getBundle().Deployment, hco)

		Expect(dep.Namespace).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
		Expect(dep.Spec.Selector.MatchLabels).To(Equal(map[string]string{
			hcoutil.AppLabel:          hcoutil.HyperConvergedName,
			hcoutil.AppLabelComponent: string(appComponent),
		}))
		Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
		Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(appComponent)))
	})

	It("should keep the pod labels of the bundle", func() {
		bundleDeployment := getBundle().Deployment
		dep := newDeployment(bundleDeployment, hco)

		for k, v := range bundleDeployment.Spec.Template.Labels {
			if k == hcoutil.AppLabel || k == hcoutil.AppLabelComponent {
				continue
			}
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(k, v))
		}
	})

	It("should not modify the bundle Deployment", func() {
		bundleDeployment := getBundle().Deployment
		orig := bundleDeployment.DeepCopy()
		_ = newDeployment(bundleDeployment, hco)
		Expect(bundleDeployment).To(Equal(orig))
	})

	It("should only allow the controller DNS and API server egress", func() {
		bundleDeployment := getBundle().Deployment
		np := newNetworkPolicy(bundleDeployment)
		dep := newDeployment(bundleDeployment, hco)

		Expect(np.Namespace).To(Equal(dep.Namespace))
		Expect(np.Spec.PodSelector.MatchLabels).To(Equal(dep.Spec.Selector.MatchLabels))
		Expect(np.Spec.Ingress).To(BeEmpty())
		Expect(np.Spec.Egress).ToNot(BeEmpty())
		Expect(np.Spec.PolicyTypes).To(ContainElement(networkingv1.PolicyTypeEgress))
	})

	Context("node placement", func() {
		It("should keep the bundle placement without infra placement", func() {
			bundleDeployment := getBundle().Deployment
			dep := newDeployment(bundleDeployment, hco)

			Expect(dep.Spec.Template.Spec.NodeSelector).To(Equal(bundleDeployment.Spec.Template.Spec.NodeSelector))
			Expect(dep.Spec.Template.Spec.Affinity).To(Equal(bundleDeployment.Spec.Template.Spec.Affinity))
			Expect(dep.Spec.Template.Spec.Tolerations).To(Equal(bundleDeployment.Spec.Template.Spec.Tolerations))
		})

		It("should apply the infra placement", func() {
			hco.Spec.Deployment.NodePlacements = &hcov1.NodePlacements{
				Infra: commontestutils.NewNodePlacement(),
			}
			infra := hco.Spec.Deployment.NodePlacements.Infra

			bundleDeployment := getBundle().Deployment
			dep := newDeployment(bundleDeployment, hco)

			Expect(dep.Spec.Template.Spec.NodeSelector).To(Equal(infra.NodeSelector))
			Expect(dep.Spec.Template.Spec.Tolerations).To(Equal(infra.Tolerations))

			infraTerm := infra.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
			bundleTerm := bundleDeployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
			Expect(dep.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(Equal([]corev1.NodeSelectorTerm{{
				MatchExpressions: slices.Concat(infraTerm.MatchExpressions, bundleTerm.MatchExpressions),
				MatchFields:      infraTerm.MatchFields,
			}}))
		})
	})

	Context("mergeAffinity", func() {
		archRequirement := corev1.NodeSelectorRequirement{Key: "kubernetes.io/arch", Operator: corev1.NodeSelectorOpIn, Values: []string{"amd64"}}
		zoneRequirement := corev1.NodeSelectorRequirement{Key: "zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"a"}}
		infraRequirement := corev1.NodeSelectorRequirement{Key: "node-role.kubernetes.io/infra", Operator: corev1.NodeSelectorOpExists}
		otherRequirement := corev1.NodeSelectorRequirement{Key: "other", Operator: corev1.NodeSelectorOpExists}

		required := func(terms ...corev1.NodeSelectorTerm) *corev1.Affinity {
			return &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: terms},
			}}
		}
		term := func(reqs ...corev1.NodeSelectorRequirement) corev1.NodeSelectorTerm {
			return corev1.NodeSelectorTerm{MatchExpressions: reqs}
		}

		It("should use the infra affinity when the bundle has none", func() {
			infra := required(term(infraRequirement))
			Expect(mergeAffinity(nil, infra)).To(Equal(infra))
		})

		It("should keep the bundle requirement when infra has no required node affinity", func() {
			antiAffinity := &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 1}},
			}
			infra := &corev1.Affinity{PodAntiAffinity: antiAffinity}

			merged := mergeAffinity(required(term(archRequirement)), infra)

			Expect(merged.PodAntiAffinity).To(Equal(antiAffinity))
			Expect(merged.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(Equal([]corev1.NodeSelectorTerm{term(archRequirement)}))
		})

		It("should add the bundle requirements to every infra term", func() {
			merged := mergeAffinity(
				required(term(archRequirement), term(zoneRequirement)),
				required(term(infraRequirement), term(otherRequirement)),
			)

			Expect(merged.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(Equal([]corev1.NodeSelectorTerm{
				term(infraRequirement, archRequirement),
				term(infraRequirement, zoneRequirement),
				term(otherRequirement, archRequirement),
				term(otherRequirement, zoneRequirement),
			}))
		})

		It("should not modify its inputs", func() {
			bundle := required(term(archRequirement))
			infra := required(term(infraRequirement))
			origBundle, origInfra := bundle.DeepCopy(), infra.DeepCopy()

			_ = mergeAffinity(bundle, infra)

			Expect(bundle).To(Equal(origBundle))
			Expect(infra).To(Equal(origInfra))
		})
	})
})
