package mutator

import (
	"context"
	"fmt"

	"k8s.io/utils/pointer"

	v1 "kubevirt.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commonTestUtils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	hcoNamespace = HcoValidNamespace
	podNamespace = "fake-namespace"

	podName = "virt-launcher-vmi-test"
	vmiName = "test-vmi"
)

var _ = Describe("virt-launcher webhook mutator", func() {

	Describe("resource multiplier", func() {
		DescribeTable("produces correct results", func(inputQuantityStr, expectedOutputQuantityStr string, ratio float64) {
			inputQuantity := resource.MustParse(inputQuantityStr)
			expectedOutputQuantity := resource.MustParse(expectedOutputQuantityStr)

			mutator := getVirtLauncherMutator()
			actualOutput := mutator.multiplyResource(inputQuantity, ratio)
			Expect(actualOutput.Equal(expectedOutputQuantity)).To(BeTrue(), fmt.Sprintf("expected %s to equal %s", actualOutput.String(), expectedOutputQuantity.String()))
		},
			Entry("CPU: 100m with ratio 2", "100m", "200m", 2.0),
			Entry("CPU: 700m with ratio 2", "700m", "1400m", 2.0),
			Entry("CPU: 2.4 with ratio 2", "2.4", "4800m", 2.0),
			Entry("CPU: 0.4 with ratio 2", "0.4", "800m", 2.0),
			Entry("CPU: 200m with ratio 0.5", "200m", "100m", 0.5),
			Entry("CPU: 1 with ratio 0.5", "1", "500m", 0.5),

			Entry("Memory: 256 with ratio 3.0", "256", "768", 3.0),
			Entry("Memory: 256M with ratio 3.0", "256M", "768M", 3.0),
			Entry("Memory: 256Mi with ratio 3.0", "256Mi", "768Mi", 3.0),
			Entry("Memory: 256Gi with ratio 3.0", "256Gi", "768Gi", 3.0),
			Entry("Memory: 700M with ratio 3.0", "700M", "2100M", 3.0),
			Entry("Memory: 256M with ratio 3.0", "260M", "52M", 0.2),
		)

	})

	Context("resource ratio enforcement", func() {
		DescribeTable("set resource ratio", func(memRatio, cpuRatio string, podResources, expectedResources k8sv1.ResourceRequirements) {
			mutator := getVirtLauncherMutator()
			launcherPod := getFakeLauncherPod()
			hco := getHco()
			hco.Annotations = map[string]string{
				cpuLimitToRequestRatioAnnotation:    cpuRatio,
				memoryLimitToRequestRatioAnnotation: memRatio,
			}

			launcherPod.Spec.Containers[0].Resources = podResources
			creationConfig := virtLauncherCreationConfig{
				enforceCpuLimits:    true,
				cpuRatioStr:         cpuRatio,
				enforceMemoryLimits: true,
				memRatioStr:         memRatio,
			}
			err := mutator.handleVirtLauncherCreation(context.Background(), launcherPod, creationConfig)
			Expect(err).ToNot(HaveOccurred())

			resources := launcherPod.Spec.Containers[0].Resources
			Expect(resources.Limits[k8sv1.ResourceCPU].Equal(expectedResources.Limits[k8sv1.ResourceCPU])).To(BeTrue())
			Expect(resources.Requests[k8sv1.ResourceCPU].Equal(expectedResources.Requests[k8sv1.ResourceCPU])).To(BeTrue())
			Expect(resources.Limits[k8sv1.ResourceMemory].Equal(expectedResources.Limits[k8sv1.ResourceMemory])).To(BeTrue())
			Expect(resources.Requests[k8sv1.ResourceMemory].Equal(expectedResources.Requests[k8sv1.ResourceMemory])).To(BeTrue())
		},
			Entry("200m cpu with ratio 2", "1.0", "2.0",
				getResources(withCpuRequest("200m")),
				getResources(withCpuRequest("200m"), withCpuLimit("400m")),
			),
			Entry("100M memory with ratio 1.5", "1.5", "1.0",
				getResources(withMemRequest("100M")),
				getResources(withMemRequest("100M"), withMemLimit("150M")),
			),
			Entry("200m cpu with ratio 2, 100M memory with ratio 1.5", "1.5", "2.0",
				getResources(withCpuRequest("200m"), withMemRequest("100M")),
				getResources(withCpuRequest("200m"), withCpuLimit("400m"), withMemRequest("100M"), withMemLimit("150M")),
			),
			Entry("requests and limits are already set", "1.5", "2.0",
				getResources(withCpuRequest("200m"), withCpuLimit("400m"), withMemRequest("100M"), withMemLimit("150M")),
				getResources(withCpuRequest("200m"), withCpuLimit("400m"), withMemRequest("100M"), withMemLimit("150M")),
			),
			Entry("requests and limits aren't set - nothing should be done", "1.5", "2.0",
				getResources(),
				getResources(),
			),
		)

		Context("resources to enforce", func() {
			const (
				setRatio, dontSetRatio          = true, false
				setLimit, dontSetLimit          = true, false
				shouldEnforce, shouldNotEnforce = true, false
			)

			DescribeTable("should behave as expected", func(resourceName k8sv1.ResourceName, setRatio, setResourceQuotaLimit, shouldEnforce bool) {
				Expect(resourceName).To(Or(Equal(k8sv1.ResourceMemory), Equal(k8sv1.ResourceCPU)))

				hco := getHco()
				if setRatio {
					if resourceName == k8sv1.ResourceCPU {
						hco.Annotations[cpuLimitToRequestRatioAnnotation] = "1.2"
					} else {
						hco.Annotations[memoryLimitToRequestRatioAnnotation] = "3.4"
					}
				}

				mutator := getVirtLauncherMutatorWithoutResourceQuotas(true, true)
				if setResourceQuotaLimit {
					if resourceName == k8sv1.ResourceCPU {
						mutator = getVirtLauncherMutatorWithoutResourceQuotas(false, true)
					} else {
						mutator = getVirtLauncherMutatorWithoutResourceQuotas(true, false)
					}
				}

				creationConfig, err := mutator.getResourcesToEnforce(context.TODO(), podNamespace, hco, virtLauncherCreationConfig{})
				Expect(err).ToNot(HaveOccurred())

				if resourceName == k8sv1.ResourceCPU {
					Expect(creationConfig.enforceCpuLimits).To(Equal(shouldEnforce))
				} else {
					Expect(creationConfig.enforceMemoryLimits).To(Equal(shouldEnforce))
				}
			},
				Entry("memory: setRatio, setLimit - shouldEnforce", k8sv1.ResourceMemory, setRatio, setLimit, shouldEnforce),
				Entry("memory: setRatio, dontSetLimit - shouldNotEnforce", k8sv1.ResourceMemory, setRatio, dontSetLimit, shouldNotEnforce),
				Entry("memory: dontSetRatio, setLimit - shouldNotEnforce", k8sv1.ResourceMemory, dontSetRatio, setLimit, shouldNotEnforce),
				Entry("memory: dontSetRatio, dontSetLimit - shouldNotEnforce", k8sv1.ResourceMemory, dontSetRatio, dontSetLimit, shouldNotEnforce),

				Entry("cpu: setRatio, setLimit - shouldEnforce", k8sv1.ResourceCPU, setRatio, setLimit, shouldEnforce),
				Entry("cpu: setRatio, dontSetLimit - shouldNotEnforce", k8sv1.ResourceCPU, setRatio, dontSetLimit, shouldNotEnforce),
				Entry("cpu: dontSetRatio, setLimit - shouldNotEnforce", k8sv1.ResourceCPU, dontSetRatio, setLimit, shouldNotEnforce),
				Entry("cpu: dontSetRatio, dontSetLimit - shouldNotEnforce", k8sv1.ResourceCPU, dontSetRatio, dontSetLimit, shouldNotEnforce),
			)
		})

		Context("invalid requests", func() {
			const resourceAnnotationKey = "fake-key" // this is not important for this test

			DescribeTable("invalid ratio", func(ratio string, resourceName k8sv1.ResourceName) {
				launcherPod := getFakeLauncherPod()
				mutator := getVirtLauncherMutator()

				Expect(mutator.setResourceRatio(launcherPod, ratio, resourceAnnotationKey, resourceName)).ToNot(Succeed())
			},
				Entry("zero ratio", "0", k8sv1.ResourceCPU),
				Entry("negative ratio", "-1.2", k8sv1.ResourceMemory),
			)

			Context("objects do not exist", func() {
				newRequest := func(operation admissionv1.Operation, object runtime.Object, encoder runtime.Encoder) admissionv1.AdmissionRequest {
					return admissionv1.AdmissionRequest{
						Operation: operation,
						Object: runtime.RawExtension{
							Raw:    []byte(runtime.EncodeOrDie(encoder, object)),
							Object: object,
						},
					}
				}

				It("HCO CR object does not exist", func() {
					codecFactory := serializer.NewCodecFactory(scheme.Scheme)
					corev1Codec := codecFactory.LegacyCodec(k8sv1.SchemeGroupVersion)

					launcherPod := getFakeLauncherPod()
					mutator := getVirtLauncherMutatorWithoutHco()
					req := admission.Request{AdmissionRequest: newRequest(admissionv1.Create, launcherPod, corev1Codec)}

					res := mutator.Handle(context.TODO(), req)
					Expect(res.Allowed).To(BeFalse())
					Expect(res.Result.Message).To(ContainSubstring("not found"))
				})
			})

			It("should not apply if only limit is set", func() {
				launcherPod := getFakeLauncherPod()
				mutator := getVirtLauncherMutator()

				launcherPod.Spec.Containers[0].Resources = k8sv1.ResourceRequirements{
					Limits: map[k8sv1.ResourceName]resource.Quantity{
						k8sv1.ResourceCPU:    resource.MustParse("1"),
						k8sv1.ResourceMemory: resource.MustParse("1"),
					},
				}

				const ratio = "1.23"
				err := mutator.setResourceRatio(launcherPod, ratio, resourceAnnotationKey, k8sv1.ResourceCPU)
				Expect(err).ToNot(HaveOccurred())

				err = mutator.setResourceRatio(launcherPod, ratio, resourceAnnotationKey, k8sv1.ResourceMemory)
				Expect(err).ToNot(HaveOccurred())

				Expect(launcherPod.Spec.Containers[0].Resources.Requests).To(BeEmpty())
			})
		})
	})

	Context("system reserved memory", func() {

		var (
			enableReservedMemory = pointer.String("true")
			//disableReservedMemory = pointer.String("false")
		)

		getHco := func(enableReservedMemoryAnnotation, customReservedMemoryAnnotation *string) *v1beta1.HyperConverged {
			hco := getHco()
			if hco.Annotations == nil {
				hco.Annotations = map[string]string{}
			}

			if enableReservedMemoryAnnotation != nil {
				hco.Annotations[enableMemoryHeadroomAnnotation] = *enableReservedMemoryAnnotation
			}
			if customReservedMemoryAnnotation != nil {
				hco.Annotations[customMemoryHeadroomAnnotation] = *customReservedMemoryAnnotation
			}

			return hco
		}

		getVmi := func(guestMemory, memoryRequest, memoryLimit string) *v1.VirtualMachineInstance {
			vmi := &v1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vmiName,
					Namespace: podNamespace,
				},
				Spec: v1.VirtualMachineInstanceSpec{
					Domain: v1.DomainSpec{
						Memory: &v1.Memory{},
						Resources: v1.ResourceRequirements{
							Limits:   map[k8sv1.ResourceName]resource.Quantity{},
							Requests: map[k8sv1.ResourceName]resource.Quantity{},
						},
					},
				},
			}

			if guestMemory != "" {
				guestMemoryQuantity := resource.MustParse(guestMemory)
				vmi.Spec.Domain.Memory.Guest = &guestMemoryQuantity
			}
			if memoryRequest != "" {
				vmi.Spec.Domain.Resources.Requests[k8sv1.ResourceMemory] = resource.MustParse(memoryRequest)
			}
			if memoryLimit != "" {
				vmi.Spec.Domain.Resources.Limits[k8sv1.ResourceMemory] = resource.MustParse(memoryLimit)
			}

			return vmi
		}

		getMutator := func(enableReservedMemoryAnnotation, customReservedMemoryAnnotation *string, vmi *v1.VirtualMachineInstance) *VirtLauncherMutator {
			hco := getHco(enableReservedMemoryAnnotation, customReservedMemoryAnnotation)

			var clusterObjects []runtime.Object
			if vmi != nil {
				clusterObjects = append(clusterObjects, vmi)
			}

			return getVirtLauncherMutatorHelperWithHco(hco, clusterObjects...)
		}

		DescribeTable("generateCreationConfig", func(enableReservedMemoryAnnotation, customReservedMemoryAnnotation *string, expectedConfig virtLauncherCreationConfig) {
			mutator := getMutator(enableReservedMemoryAnnotation, customReservedMemoryAnnotation, nil)
			hco := getHco(enableReservedMemoryAnnotation, customReservedMemoryAnnotation)

			config, err := mutator.generateCreationConfig(context.Background(), podNamespace, hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(Equal(expectedConfig))
		},
			Entry("enable annotation is nil, without custom memory reservation", nil, nil, virtLauncherCreationConfig{}),
			Entry("enable annotation is false, without custom memory reservation", pointer.String("false"), nil, virtLauncherCreationConfig{}),
			Entry("enable annotation is nil, with custom memory reservation", nil, pointer.String("123G"), virtLauncherCreationConfig{}),
			Entry("enable annotation is false, with custom memory reservation", pointer.String("false"), pointer.String("123G"), virtLauncherCreationConfig{}),

			Entry("enable annotation is true, without custom memory reservation", pointer.String("true"), nil,
				virtLauncherCreationConfig{
					addMemoryHeadroom: true,
					memoryHeadroom:    "2G",
				}),
			Entry("enable annotation is true, with custom memory reservation", pointer.String("true"), pointer.String("123G"),
				virtLauncherCreationConfig{
					addMemoryHeadroom: true,
					memoryHeadroom:    "123G",
				}),
		)

		Context("handleVirtLauncherMemoryHeadroom()", func() {

			getFakeLauncherPod := func(memRequest, memLimit string) *k8sv1.Pod {
				pod := getFakeLauncherPod()
				if memRequest != "" {
					pod.Spec.Containers[0].Resources.Requests[k8sv1.ResourceMemory] = resource.MustParse(memRequest)
				}
				if memLimit != "" {
					pod.Spec.Containers[0].Resources.Limits[k8sv1.ResourceMemory] = resource.MustParse(memLimit)
				}

				return pod
			}

			getDefaultCreationConfig := func() virtLauncherCreationConfig {
				return virtLauncherCreationConfig{
					addMemoryHeadroom: true,
					memoryHeadroom:    "2G",
				}
			}

			It("should skip if limits are set", func() {
				vmi := getVmi("1G", "2G", "3G")
				mutator := getMutator(enableReservedMemory, nil, vmi)
				launcherPod := getFakeLauncherPod("2G", "3G")
				originalLauncherPod := launcherPod.DeepCopy()

				err := mutator.handleVirtLauncherMemoryHeadroom(context.Background(), launcherPod, getDefaultCreationConfig())
				Expect(err).ToNot(HaveOccurred())
				Expect(originalLauncherPod).To(Equal(launcherPod), "expect nothing to be changed")
			})

			It("should provide error if VMI does not exist", func() {
				mutator := getMutator(enableReservedMemory, nil, nil)
				launcherPod := getFakeLauncherPod("2G", "3G")
				originalLauncherPod := launcherPod.DeepCopy()

				err := mutator.handleVirtLauncherMemoryHeadroom(context.Background(), launcherPod, getDefaultCreationConfig())
				Expect(err).To(HaveOccurred())
				Expect(originalLauncherPod).To(Equal(launcherPod), "expect nothing to be changed")
			})

			It("should provide error if VMI name label doesn't exist", func() {
				vmi := getVmi("1G", "2G", "3G")
				mutator := getMutator(enableReservedMemory, nil, vmi)
				launcherPod := getFakeLauncherPod("2G", "3G")
				launcherPod.Labels = map[string]string{}
				originalLauncherPod := launcherPod.DeepCopy()

				err := mutator.handleVirtLauncherMemoryHeadroom(context.Background(), launcherPod, getDefaultCreationConfig())
				Expect(err).To(HaveOccurred())
				Expect(originalLauncherPod).To(Equal(launcherPod), "expect nothing to be changed")
			})

			It("should provide error if compute container doesn't exist", func() {
				vmi := getVmi("1G", "2G", "")
				mutator := getMutator(enableReservedMemory, nil, vmi)
				launcherPod := getFakeLauncherPod("2G", "")
				launcherPod.Spec.Containers[0].Name = "something-that-is-not-compute"
				originalLauncherPod := launcherPod.DeepCopy()

				err := mutator.handleVirtLauncherMemoryHeadroom(context.Background(), launcherPod, getDefaultCreationConfig())
				Expect(err).To(HaveOccurred())
				Expect(originalLauncherPod).To(Equal(launcherPod), "expect nothing to be changed")
			})

			DescribeTable("should add system reserved memory to requests", func(guestMemory string) {
				const memRequest = "2G"

				vmi := getVmi(guestMemory, memRequest, "")
				mutator := getMutator(enableReservedMemory, nil, vmi)
				launcherPod := getFakeLauncherPod(memRequest, "")

				originalLauncherPod := launcherPod.DeepCopy()
				creationConfig := getDefaultCreationConfig()

				err := mutator.handleVirtLauncherMemoryHeadroom(context.Background(), launcherPod, creationConfig)
				Expect(err).ToNot(HaveOccurred())

				expectedMemRequest := originalLauncherPod.Spec.Containers[0].Resources.Requests[k8sv1.ResourceMemory]
				expectedMemRequest.Add(resource.MustParse(creationConfig.memoryHeadroom))
				Expect(expectedMemRequest).To(Equal(launcherPod.Spec.Containers[0].Resources.Requests[k8sv1.ResourceMemory]), "system reserved memory should be added")

				updatedVmi := &v1.VirtualMachineInstance{}
				err = mutator.cli.Get(context.Background(), client.ObjectKeyFromObject(vmi), updatedVmi)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedVmi.Spec.Domain.Memory).To(Equal(vmi.Spec.Domain.Memory), "memory guest should not be changed")
			},
				Entry("with guest memory defined", "1G"),
				Entry("without guest memory defined", ""),
			)
		})

	})

	DescribeTable("any operation other than CREATE should be allowed", func(operation admissionv1.Operation) {
		mutator := getVirtLauncherMutator()
		codecFactory := serializer.NewCodecFactory(scheme.Scheme)
		corev1Codec := codecFactory.LegacyCodec(k8sv1.SchemeGroupVersion)
		launcherPod := getFakeLauncherPod()

		req := admission.Request{AdmissionRequest: newRequest(admissionv1.Create, launcherPod, corev1Codec)}

		res := mutator.Handle(context.TODO(), req)
		Expect(res.Allowed).To(BeFalse())
	},
		Entry("update", admissionv1.Update),
		Entry("delete", admissionv1.Delete),
		Entry("connect", admissionv1.Connect),
	)

})

func getVirtLauncherMutator() *VirtLauncherMutator {
	return getVirtLauncherMutatorWithResourceQuotas(true, true, true)
}

func getVirtLauncherMutatorWithoutHco() *VirtLauncherMutator {
	return getVirtLauncherMutatorWithResourceQuotas(false, true, true)
}

func getVirtLauncherMutatorWithoutResourceQuotas(avoidCpuLimit, avoidMemoryLimit bool) *VirtLauncherMutator {
	return getVirtLauncherMutatorWithResourceQuotas(true, !avoidCpuLimit, !avoidMemoryLimit)
}

func getVirtLauncherMutatorWithResourceQuotas(hcoExists, resourceQuotaCpuExists, resourceQuotaMemoryExists bool) *VirtLauncherMutator {
	var hco *v1beta1.HyperConverged
	if hcoExists {
		hco = getHco()
	}

	var clusterObjects []runtime.Object
	if resourceQuotaCpuExists {
		clusterObjects = append(clusterObjects, getResourceQuota(true, false))
	}
	if resourceQuotaMemoryExists {
		clusterObjects = append(clusterObjects, getResourceQuota(false, true))
	}

	return getVirtLauncherMutatorHelperWithHco(hco, clusterObjects...)
}

func getVirtLauncherMutatorHelperWithHco(hco *v1beta1.HyperConverged, objects ...runtime.Object) *VirtLauncherMutator {
	var cli *commonTestUtils.HcoTestClient
	var clusterObjects []runtime.Object

	if hco != nil {
		clusterObjects = append(clusterObjects, hco)
	}
	clusterObjects = append(clusterObjects, objects...)

	cli = commonTestUtils.InitClient(clusterObjects)
	mutator := NewVirtLauncherMutator(cli, hcoNamespace)

	decoder, err := admission.NewDecoder(scheme.Scheme)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	ExpectWithOffset(1, mutator.InjectDecoder(decoder)).Should(Succeed())

	return mutator
}

func getHco() *v1beta1.HyperConverged {
	return &v1beta1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:        util.HyperConvergedName,
			Namespace:   HcoValidNamespace,
			Annotations: map[string]string{},
		},
		Spec: v1beta1.HyperConvergedSpec{},
	}
}

func getFakeLauncherPod() *k8sv1.Pod {
	return &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
			Labels:    map[string]string{virtLauncherVmiNameLabel: vmiName},
		},
		Spec: k8sv1.PodSpec{
			Containers: []k8sv1.Container{{
				Name: "compute",
				Resources: k8sv1.ResourceRequirements{
					Requests: map[k8sv1.ResourceName]resource.Quantity{},
					Limits:   map[k8sv1.ResourceName]resource.Quantity{},
				},
			}},
		},
	}
}

func getResourceQuota(toLimitCPU, toLimitMemory bool) *k8sv1.ResourceQuota {
	rq := &k8sv1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource-quota" + rand.String(5),
			Namespace: podNamespace,
		},
		Spec: k8sv1.ResourceQuotaSpec{
			Hard: map[k8sv1.ResourceName]resource.Quantity{},
		},
	}

	if toLimitCPU {
		rq.Spec.Hard["limits.cpu"] = resource.MustParse("3000")
	}
	if toLimitMemory {
		rq.Spec.Hard["limits.memory"] = resource.MustParse("3000G")
	}

	return rq
}

type resourceOption func(*k8sv1.ResourceRequirements)

func getResources(options ...resourceOption) k8sv1.ResourceRequirements {
	r := k8sv1.ResourceRequirements{
		Limits:   map[k8sv1.ResourceName]resource.Quantity{},
		Requests: map[k8sv1.ResourceName]resource.Quantity{},
	}

	for _, option := range options {
		option(&r)
	}

	return r
}

func withCpuRequest(quantityStr string) resourceOption {
	return func(r *k8sv1.ResourceRequirements) {
		r.Requests[k8sv1.ResourceCPU] = resource.MustParse(quantityStr)
	}
}

func withCpuLimit(quantityStr string) resourceOption {
	return func(r *k8sv1.ResourceRequirements) {
		r.Limits[k8sv1.ResourceCPU] = resource.MustParse(quantityStr)
	}
}

func withMemRequest(quantityStr string) resourceOption {
	return func(r *k8sv1.ResourceRequirements) {
		r.Requests[k8sv1.ResourceMemory] = resource.MustParse(quantityStr)
	}
}

func withMemLimit(quantityStr string) resourceOption {
	return func(r *k8sv1.ResourceRequirements) {
		r.Limits[k8sv1.ResourceMemory] = resource.MustParse(quantityStr)
	}
}
