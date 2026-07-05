package mutator

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1fg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
)

var _ = Describe("test HyperConverged v1 mutator", func() {
	var (
		cr      *hcov1.HyperConverged
		cli     client.Client
		mutator *HyperConvergedMutator
	)

	mutatorScheme = scheme.Scheme
	Expect(hcov1.AddToScheme(mutatorScheme)).To(Succeed())
	BeforeEach(func() {
		Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
		cr = &hcov1.HyperConverged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      util.HyperConvergedName,
				Namespace: HcoValidNamespace,
			},
			Spec: hcov1.HyperConvergedSpec{
				Virtualization: hcov1.VirtualizationConfig{
					EvictionStrategy: ptr.To(kubevirtcorev1.EvictionStrategyLiveMigrate),
				},
			},
		}

		cli = commontestutils.InitClient(nil)
		mutator = initHCMutator(mutatorScheme, cli)
	})

	Context("MDev mutation on creation", func() {
		var (
			ksmPatch = jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/virtualization/ksmConfiguration",
				Value:     kubevirtcorev1.KSMConfiguration{},
			}
		)

		DescribeTable("migrate disableMDevConfiguration to mediatedDevicesConfiguration.enabled on create",
			func(ctx context.Context, featureGates hcov1fg.HyperConvergedFeatureGates, mdc *hcov1.MediatedDevicesConfiguration, allowed bool, warning bool, extraPatches []jsonpatch.JsonPatchOperation) {
				cr.Spec.FeatureGates = featureGates
				cr.Spec.Virtualization.MediatedDevicesConfiguration = mdc

				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(Equal(allowed))

				if warning {
					Expect(res.Warnings).To(HaveLen(1))
					Expect(res.Warnings).To(ContainElement(fmt.Sprintf(fgDeprecationMsg, disableMDevConfigurationFGName)))
				} else {
					Expect(res.Warnings).To(BeEmpty())
				}

				var expectedPatches []jsonpatch.JsonPatchOperation
				if allowed {
					expectedPatches = append([]jsonpatch.JsonPatchOperation{ksmPatch}, extraPatches...)
				}
				Expect(res.Patches).To(Equal(expectedPatches))
			},
			Entry("should set enabled=false when the FG is enabled and mediatedDevicesConfiguration is unset",
				hcov1fg.HyperConvergedFeatureGates{{Name: disableMDevConfigurationFGName}},
				nil,
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1HyperConvergedMdevConfigPath,
					Value:     map[string]any{"enabled": false},
				}},
			),
			Entry("should set enabled=false when the FG is enabled and enabled is unset",
				hcov1fg.HyperConvergedFeatureGates{{Name: disableMDevConfigurationFGName}},
				&hcov1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222", "nvidia-230"},
				},
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1MDevEnabledPath,
					Value:     false,
				}},
			),
			Entry("should set the enabled to true, when the FG is explicitly disabled",
				hcov1fg.HyperConvergedFeatureGates{{Name: disableMDevConfigurationFGName, State: new(hcov1fg.Disabled)}},
				&hcov1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222", "nvidia-230"},
				},
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1MDevEnabledPath,
					Value:     true,
				}},
			),
			Entry("should do nothing when the field and the FG are not set",
				hcov1fg.HyperConvergedFeatureGates{},
				&hcov1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222", "nvidia-230"},
				},
				true,
				false,
				nil,
			),
			Entry("should do nothing when the field is enabled and the FG are not set",
				hcov1fg.HyperConvergedFeatureGates{},
				&hcov1.MediatedDevicesConfiguration{
					Enabled: new(true),
				},
				true,
				false,
				nil,
			),
			Entry("should do nothing when the field is disabled and the FG are not set",
				hcov1fg.HyperConvergedFeatureGates{},
				&hcov1.MediatedDevicesConfiguration{
					Enabled: new(false),
				},
				true,
				false,
				nil,
			),
			Entry("should reject if the FG is enabled and the enabled field is true",
				hcov1fg.HyperConvergedFeatureGates{{Name: disableMDevConfigurationFGName}},
				&hcov1.MediatedDevicesConfiguration{
					Enabled:             new(true),
					MediatedDeviceTypes: []string{"nvidia-222", "nvidia-230"},
				},
				false,
				false,
				nil,
			),
			Entry("should reject if the FG is disabled and the enabled field is false",
				hcov1fg.HyperConvergedFeatureGates{{Name: disableMDevConfigurationFGName, State: new(hcov1fg.Disabled)}},
				&hcov1.MediatedDevicesConfiguration{
					Enabled:             new(false),
					MediatedDeviceTypes: []string{"nvidia-222", "nvidia-230"},
				},
				false,
				false,
				nil,
			),
		)
	})

	Context("sync the disableMDevConfiguration FG and mediatedDevicesConfiguration.enabled field on update", func() {
		/*
			There are 81 cases:
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			FG →      | Old: nil | Old: nil	  | Old: nil   | Old: E    | Old: E     | Old: E     | Old: D     | Old: D     | Old: D
			Enabled ↓ | New: nil | New: E     | New: D     | New: nil  | New: E     | New: D     | New: nil   | New: E     | New: D
			============================================================================================================================
			Old: nil |           | warning +  | warning +  |           |            | warning +  |            | warning +  |
			New: nil | No patches| Enabled = F|	Enabled = T| No patches| Enabled = F| Enabled = T| No patches |	Enabled = F| Enabled = T
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: nil |           |            | warning +  |           |            | warning +  |            |            |
			New: T   | No patches| Reject     | No patches | No patches| Remove FG  | No patches | No patches | Reject     | Remove FG
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: nil |           | warning +  | warning +  |           |            |            |            | warning +  |
			New: F   | No patches| No patches | Reject     | No patches| Remove FG  | Reject     | No patches | No patches | Remove FG
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: T   |           |            | warning +  |           |            | warning +  |            |            |
			New: nil | No patches| Reject     | No patches | No patches| Remove FG  | No patches | No patches | Reject     | Remove FG
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: T   |           | warning +  | warning +  |           |            | warning +  |            | warning +  |
			New: T   | No patches| Enabled = F| No patches | No patches| Remove FG  | No patches | No patches | Enabled = F| No patches
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: T   |           | warning +  |            |           |            |            |            | warning +  |
			New: F   | No patches| No patches | Reject     | No patches| Remove FG  | Reject     | No patches | No patches | Remove FG
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: F   |           |            | warning +  |           |            | warning +  |            |            |
			New: nil | No patches| Reject     | No patches | No patches| Remove FG  | No patches | No patches | Reject     | Remove FG
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: F   |           |            | warning +  |           |            | warning +  |            |            |
			New: T   | No patches| Reject     | No patches | No patches| Remove FG  | No patches | No patches | Reject     | Remove FG
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
			Old: F   |           | warning +  | warning +  |           |            | warning +  |            | warning +  |
			New: F   | No patches| No patches | Enabled = T| No patches| No patches | Enabled = T| No patches | No patches | Remove FG
			---------+-----------+------------+------------+-----------+------------+------------+------------+------------+------------
		*/

		nilFG := hcov1fg.HyperConvergedFeatureGates{}
		enabledFG := hcov1fg.HyperConvergedFeatureGates{
			{Name: disableMDevConfigurationFGName},
		}
		disabledFG := hcov1fg.HyperConvergedFeatureGates{
			{Name: disableMDevConfigurationFGName, State: new(hcov1fg.Disabled)},
		}

		nilField := hcov1.VirtualizationConfig{
			MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{},
		}
		enabledField := hcov1.VirtualizationConfig{
			MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
				Enabled: new(true),
			},
		}
		disabledField := hcov1.VirtualizationConfig{
			MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
				Enabled: new(false),
			},
		}

		testSyncMDevEnabledAndFG := func(ctx context.Context, origCR, newCR *hcov1.HyperConverged, expectedRes *expectedResponse) {
			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, newCR, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(expectedRes.checkAllowed)
			Expect(res.Patches).To(Equal(expectedRes.patches))
			Expect(res.Warnings).To(expectedRes.checkWarning)
		}

		DescribeTable("1st table row: when Enabled field is nil (no change)", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedSetEnabledFalse().WithWarning(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedSetEnabledTrue().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should set field = False, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedSetEnabledFalse(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedSetEnabledTrue().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedSetEnabledFalse().WithWarning(),
			),
			Entry("should set field = True, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedSetEnabledTrue(),
			),
		)

		DescribeTable("2nd table row: when Enabled field is changed: nil -> True", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedRemoveFG(),
			),
		)
		DescribeTable("3rd table row: when Enabled field is changed: nil -> False", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should reject, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedReject(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedRemoveFG(),
			),
			Entry("should reject, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedReject(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedRemoveFG(),
			),
		)

		DescribeTable("4th table row: when Enabled field is changed: True -> nil", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedRemoveFG(),
			),
		)

		DescribeTable("5th table row: when Enabled field is True (no change)", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedSetEnabledFalse().WithWarning(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning + set field = False: changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedSetEnabledFalse().WithWarning(),
			),
			Entry("should do nothing, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
		)
		DescribeTable("6th table row: when Enabled field is changed: True -> False", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should reject, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedReject(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedRemoveFG(),
			),
			Entry("should reject, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedReject(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedRemoveFG(),
			),
		)

		DescribeTable("7th table row: when Enabled field is changed: False -> nil", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: nilField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: nilField,
					},
				},
				expectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: nilField,
					},
				},
				expectedRemoveFG(),
			),
		)

		DescribeTable("8th table row: when Enabled field is changed: False -> True", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: enabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should reject, if FG changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: enabledField,
					},
				},
				expectedReject(),
			),
			Entry("should removeFG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: enabledField,
					},
				},
				expectedRemoveFG(),
			),
		)

		DescribeTable("9th table row: when Enabled field is False (no change)", testSyncMDevEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should trigger warning + set the Enabled field to true, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedSetEnabledTrue().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should do nothing, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger warning + set the Enabled field to true, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedSetEnabledTrue().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   nilFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: disabledField,
					},
				},
				expectedDoNothing().WithWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: disabledField,
					},
				},
				expectedRemoveFG(),
			),
		)

		DescribeTable("drop FG: check special jsonpatch paths", testSyncMDevEnabledAndFG,
			Entry("should remove the whole FG array, if only the disableMDevConfiguration FG is set",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "disableMDevConfiguration"},
						},
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "disableMDevConfiguration"},
						},
						Virtualization: enabledField,
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove the only the disableMDevConfiguration FG is set, if it's the first FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "disableMDevConfiguration"},
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "disableMDevConfiguration"},
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						Virtualization: enabledField,
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/0",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove the only the disableMDevConfiguration FG is set, if it's not the first FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "disableMDevConfiguration"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "disableMDevConfiguration"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						Virtualization: enabledField,
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/1",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove the only the disableMDevConfiguration FG is set, if it's the last FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: "disableMDevConfiguration"},
						},
						Virtualization: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: "disableMDevConfiguration"},
						},
						Virtualization: enabledField,
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/2",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
		)

		DescribeTable("set Enabled = true: check special jsonpatch paths", testSyncMDevEnabledAndFG,
			Entry("should set only the Enabled filed, if the MediatedDevicesConfiguration is not nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{},
						},
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{},
						},
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "add",
						Path:      v1MDevEnabledPath,
						Value:     true,
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should set the MediatedDevicesConfiguration filed, if it's nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: hcov1.VirtualizationConfig{},
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   disabledFG,
						Virtualization: hcov1.VirtualizationConfig{},
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "add",
						Path:      v1HyperConvergedMdevConfigPath,
						Value:     map[string]any{"enabled": true},
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
		)

		DescribeTable("set Enabled = false: check special jsonpatch paths", testSyncMDevEnabledAndFG,
			Entry("should set only the Enabled filed, if the MediatedDevicesConfiguration is not nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{},
						},
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{},
						},
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "add",
						Path:      v1MDevEnabledPath,
						Value:     false,
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should set the MediatedDevicesConfiguration filed, if it's nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: hcov1.VirtualizationConfig{},
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:   enabledFG,
						Virtualization: hcov1.VirtualizationConfig{},
					},
				},
				&expectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "add",
						Path:      v1HyperConvergedMdevConfigPath,
						Value:     map[string]any{"enabled": false},
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
		)
	})
})

type expectedResponse struct {
	checkAllowed gomegatypes.GomegaMatcher
	checkWarning gomegatypes.GomegaMatcher
	patches      []jsonpatch.JsonPatchOperation
}

func (response *expectedResponse) WithWarning() *expectedResponse {
	response.checkWarning = And(
		Not(BeEmpty()),
		ContainElement(fmt.Sprintf(fgDeprecationMsg, disableMDevConfigurationFGName)),
	)
	return response
}

var noPatches []jsonpatch.JsonPatchOperation

func expectedSetEnabledTrue() *expectedResponse {
	return &expectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      v1MDevEnabledPath,
			Value:     true,
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func expectedSetEnabledFalse() *expectedResponse {
	return &expectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      v1MDevEnabledPath,
			Value:     false,
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func expectedRemoveFG() *expectedResponse {
	return &expectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "remove",
			Path:      "/spec/featureGates",
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func expectedDoNothing() *expectedResponse {
	return &expectedResponse{
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
		patches:      noPatches,
	}
}

func expectedReject() *expectedResponse {
	return &expectedResponse{
		checkAllowed: BeFalse(),
		checkWarning: BeEmpty(),
		patches:      noPatches,
	}
}
