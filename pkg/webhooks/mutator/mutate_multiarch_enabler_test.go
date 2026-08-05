package mutator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1fg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("test HyperConverged v1 multiArch mutator", func() {
	var (
		cr      *hcov1.HyperConverged
		mutator *HyperConvergedMutator
	)

	BeforeEach(func() {
		cr = commontestutils.NewHco()

		cli := commontestutils.InitClient(nil)
		mutator = initHCMutator(mutatorScheme, cli)
	})

	Context("multiArch mutation on creation", func() {
		var (
			ksmPatch = jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/virtualization/ksmConfiguration",
				Value:     kubevirtcorev1.KSMConfiguration{},
			}
		)

		DescribeTable("migrate multiArch FG to enableMultiArchBootImageImport on create",
			func(ctx context.Context, featureGates hcov1fg.HyperConvergedFeatureGates, enabler *bool, allowed bool, warning bool, extraPatches []jsonpatch.JsonPatchOperation) {
				cr.Spec.FeatureGates = featureGates
				cr.Spec.WorkloadSources.EnableMultiArchBootImageImport = enabler

				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(Equal(allowed))

				if warning {
					Expect(res.Warnings).To(HaveLen(1))
					Expect(res.Warnings).To(ContainElement(multiArchFGDeprecationMsg))
				} else {
					Expect(res.Warnings).To(BeEmpty())
				}

				var expectedPatches []jsonpatch.JsonPatchOperation
				if allowed {
					expectedPatches = append([]jsonpatch.JsonPatchOperation{ksmPatch}, extraPatches...)
				}
				Expect(res.Patches).To(Equal(expectedPatches))
			},
			Entry("should set enabled=true when the FG is enabled and enabled is unset",
				hcov1fg.HyperConvergedFeatureGates{{Name: multiArchFGName}},
				nil,
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1MultiArchEnabledPath,
					Value:     true,
				}},
			),
			Entry("should set enabled=false when the FG is explicitly disabled and enabled is unset",
				hcov1fg.HyperConvergedFeatureGates{{Name: multiArchFGName, State: new(hcov1fg.Disabled)}},
				nil,
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1MultiArchEnabledPath,
					Value:     false,
				}},
			),
			Entry("should do nothing when the FG is not set and enabled is unset",
				hcov1fg.HyperConvergedFeatureGates{},
				nil,
				true,
				false,
				nil,
			),
			Entry("should do nothing when the field is enabled and the FG is not set",
				hcov1fg.HyperConvergedFeatureGates{},
				new(true),
				true,
				false,
				nil,
			),
			Entry("should do nothing when the field is disabled and the FG is not set",
				hcov1fg.HyperConvergedFeatureGates{},
				new(false),
				true,
				false,
				nil,
			),
			Entry("should warn if the FG is enabled and the enabled field is true (agree)",
				hcov1fg.HyperConvergedFeatureGates{{Name: multiArchFGName}},
				new(true),
				true,
				true,
				nil,
			),
			Entry("should reject if the FG is enabled and the enabled field is false (contradict)",
				hcov1fg.HyperConvergedFeatureGates{{Name: multiArchFGName}},
				new(false),
				false,
				false,
				nil,
			),
			Entry("should reject if the FG is disabled and the enabled field is true (contradict)",
				hcov1fg.HyperConvergedFeatureGates{{Name: multiArchFGName, State: new(hcov1fg.Disabled)}},
				new(true),
				false,
				false,
				nil,
			),
			Entry("should warn if the FG is disabled and the enabled field is false (agree)",
				hcov1fg.HyperConvergedFeatureGates{{Name: multiArchFGName, State: new(hcov1fg.Disabled)}},
				new(false),
				true,
				true,
				nil,
			),
		)
	})

	Context("sync the multiArch FG and enableMultiArchBootImageImport field on update", func() {
		/*
			Same 81-case matrix as MDev, but with same-direction semantics:
			- Agreement: FG Enabled + field=true, or FG Disabled + field=false
			- Contradiction: FG Enabled + field=false, or FG Disabled + field=true
		*/

		nilFG := hcov1fg.HyperConvergedFeatureGates{}
		enabledFG := hcov1fg.HyperConvergedFeatureGates{
			{Name: multiArchFGName},
		}
		disabledFG := hcov1fg.HyperConvergedFeatureGates{
			{Name: multiArchFGName, State: new(hcov1fg.Disabled)},
		}

		nilField := hcov1.WorkloadSourcesConfig{}
		enabledField := hcov1.WorkloadSourcesConfig{
			EnableMultiArchBootImageImport: new(true),
		}
		disabledField := hcov1.WorkloadSourcesConfig{
			EnableMultiArchBootImageImport: new(false),
		}

		testSyncMultiArchEnabledAndFG := func(ctx context.Context, origCR, newCR *hcov1.HyperConverged, expectedRes *multiArchExpectedResponse) {
			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, newCR, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(expectedRes.checkAllowed)
			Expect(res.Patches).To(Equal(expectedRes.patches))
			Expect(res.Warnings).To(expectedRes.checkWarning)
		}

		DescribeTable("1st table row: when Enabled field is nil (no change)", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedSetEnabledTrue().WithWarning(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedSetEnabledFalse().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should set field = True, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedSetEnabledTrue(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedSetEnabledFalse().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedSetEnabledTrue().WithWarning(),
			),
			Entry("should set field = False, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedSetEnabledFalse(),
			),
		)

		DescribeTable("2nd table row: when Enabled field is changed: nil -> True", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should reject, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
			Entry("should reject, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
		)

		DescribeTable("3rd table row: when Enabled field is changed: nil -> False", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
		)

		DescribeTable("4th table row: when Enabled field is changed: True -> nil", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
		)

		DescribeTable("5th table row: when Enabled field is True (no change)", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedSetEnabledFalse().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should do nothing, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedSetEnabledFalse().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
		)

		DescribeTable("6th table row: when Enabled field is changed: True -> False", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
		)

		DescribeTable("7th table row: when Enabled field is changed: False -> nil", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: nilField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
		)

		DescribeTable("8th table row: when Enabled field is changed: False -> True", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should reject, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
			Entry("should reject, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: enabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
		)

		DescribeTable("9th table row: when Enabled field is False (no change)", testSyncMultiArchEnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedSetEnabledTrue().WithWarning(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing().WithWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    nilFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    enabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedSetEnabledTrue().WithWarning(),
			),
			Entry("should do nothing, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates:    disabledFG,
						WorkloadSources: disabledField,
					},
				},
				multiArchExpectedDoNothing(),
			),
		)

		DescribeTable("drop FG: check special jsonpatch paths", testSyncMultiArchEnabledAndFG,
			Entry("should remove the whole FG array, if only the multiArch FG is set",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: multiArchFGName},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: multiArchFGName},
						},
						WorkloadSources: enabledField,
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove only the multiArch FG, if it's the first FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: multiArchFGName},
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: multiArchFGName},
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						WorkloadSources: enabledField,
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/0",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove only the multiArch FG, if it's not the first FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: multiArchFGName},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: multiArchFGName},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
						},
						WorkloadSources: enabledField,
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/1",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should remove only the multiArch FG, if it's the last FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: multiArchFGName},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: multiArchFGName},
						},
						WorkloadSources: enabledField,
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/2",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should remove several FGs",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: multiArchFGName},
							{Name: persistentReservationFGName},
							{Name: disableMDevConfigurationFGName},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: multiArchFGName},
							{Name: persistentReservationFGName},
							{Name: disableMDevConfigurationFGName},
						},
						WorkloadSources: enabledField,
						Storage: &hcov1.StorageConfig{
							PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
								Enabled: new(true),
							},
						},
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
								Enabled: new(true),
							},
						},
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "remove",
							Path:      "/spec/featureGates/4",
						},
						{
							Operation: "remove",
							Path:      "/spec/featureGates/3",
						},
						{
							Operation: "remove",
							Path:      "/spec/featureGates/2",
						},
					},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should remove several FGs - different order",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: disableMDevConfigurationFGName},
							{Name: persistentReservationFGName},
							{Name: multiArchFGName},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: new(hcov1fg.Disabled)},
							{Name: disableMDevConfigurationFGName},
							{Name: persistentReservationFGName},
							{Name: multiArchFGName},
						},
						WorkloadSources: enabledField,
						Storage: &hcov1.StorageConfig{
							PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
								Enabled: new(true),
							},
						},
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
								Enabled: new(true),
							},
						},
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "remove",
							Path:      "/spec/featureGates/4",
						},
						{
							Operation: "remove",
							Path:      "/spec/featureGates/3",
						},
						{
							Operation: "remove",
							Path:      "/spec/featureGates/2",
						},
					},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should sort patches by numeric index, not by string",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "0000"},
							{Name: "1111", State: new(hcov1fg.Disabled)},
							{Name: disableMDevConfigurationFGName},
							{Name: "3333"},
							{Name: "4444", State: new(hcov1fg.Disabled)},
							{Name: "5555"},
							{Name: "6666", State: new(hcov1fg.Disabled)},
							{Name: persistentReservationFGName},
							{Name: "8888"},
							{Name: "9999", State: new(hcov1fg.Disabled)},
							{Name: "1010"},
							{Name: multiArchFGName},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "0000"},
							{Name: "1111", State: new(hcov1fg.Disabled)},
							{Name: disableMDevConfigurationFGName},
							{Name: "3333"},
							{Name: "4444", State: new(hcov1fg.Disabled)},
							{Name: "5555"},
							{Name: "6666", State: new(hcov1fg.Disabled)},
							{Name: persistentReservationFGName},
							{Name: "8888"},
							{Name: "9999", State: new(hcov1fg.Disabled)},
							{Name: "1010"},
							{Name: multiArchFGName},
						},
						WorkloadSources: enabledField,
						Storage: &hcov1.StorageConfig{
							PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
								Enabled: new(true),
							},
						},
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
								Enabled: new(true),
							},
						},
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "remove",
							Path:      "/spec/featureGates/11",
						},
						{
							Operation: "remove",
							Path:      "/spec/featureGates/7",
						},
						{
							Operation: "remove",
							Path:      "/spec/featureGates/2",
						},
					},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should remove all FGs",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: disableMDevConfigurationFGName},
							{Name: persistentReservationFGName},
							{Name: multiArchFGName},
						},
						WorkloadSources: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: disableMDevConfigurationFGName},
							{Name: persistentReservationFGName},
							{Name: multiArchFGName},
						},
						WorkloadSources: enabledField,
						Storage: &hcov1.StorageConfig{
							PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
								Enabled: new(true),
							},
						},
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
								Enabled: new(true),
							},
						},
					},
				},
				&multiArchExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "remove",
							Path:      "/spec/featureGates",
						},
					},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
		)
	})
})

type multiArchExpectedResponse struct {
	checkAllowed gomegatypes.GomegaMatcher
	checkWarning gomegatypes.GomegaMatcher
	patches      []jsonpatch.JsonPatchOperation
}

func (response *multiArchExpectedResponse) WithWarning() *multiArchExpectedResponse {
	response.checkWarning = And(
		Not(BeEmpty()),
		ContainElement(multiArchFGDeprecationMsg),
	)
	return response
}

func multiArchExpectedSetEnabledTrue() *multiArchExpectedResponse {
	return &multiArchExpectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      v1MultiArchEnabledPath,
			Value:     true,
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func multiArchExpectedSetEnabledFalse() *multiArchExpectedResponse {
	return &multiArchExpectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      v1MultiArchEnabledPath,
			Value:     false,
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func multiArchExpectedRemoveFG() *multiArchExpectedResponse {
	return &multiArchExpectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "remove",
			Path:      "/spec/featureGates",
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func multiArchExpectedDoNothing() *multiArchExpectedResponse {
	return &multiArchExpectedResponse{
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
		patches:      noPatches,
	}
}

func multiArchExpectedReject() *multiArchExpectedResponse {
	return &multiArchExpectedResponse{
		checkAllowed: BeFalse(),
		checkWarning: BeEmpty(),
		patches:      noPatches,
	}
}
