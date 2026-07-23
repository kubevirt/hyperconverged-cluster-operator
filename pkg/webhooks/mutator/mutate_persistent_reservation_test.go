package mutator

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1fg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("test HyperConverged v1 PersistentReservation mutator", func() {
	var (
		cr      *hcov1.HyperConverged
		mutator *HyperConvergedMutator
	)

	BeforeEach(func() {
		cr = commontestutils.NewHco()

		cli := commontestutils.InitClient(nil)
		mutator = initHCMutator(mutatorScheme, cli)
	})

	Context("PersistentReservation mutation on creation", func() {
		var (
			ksmPatch = jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/virtualization/ksmConfiguration",
				Value:     kubevirtcorev1.KSMConfiguration{},
			}
		)

		DescribeTable("migrate persistentReservation FG to persistentReservationConfiguration.enabled on create",
			func(ctx context.Context, featureGates hcov1fg.HyperConvergedFeatureGates, storage *hcov1.StorageConfig, allowed bool, warning bool, extraPatches []jsonpatch.JsonPatchOperation) {
				cr.Spec.FeatureGates = featureGates
				cr.Spec.Storage = storage

				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(Equal(allowed))

				if warning {
					Expect(res.Warnings).To(HaveLen(1))
					Expect(res.Warnings).To(ContainElement(fmt.Sprintf(prFGDeprecationMsg, persistentReservationFGName)))
				} else {
					Expect(res.Warnings).To(BeEmpty())
				}

				var expectedPatches []jsonpatch.JsonPatchOperation
				if allowed {
					expectedPatches = append([]jsonpatch.JsonPatchOperation{ksmPatch}, extraPatches...)
				}
				Expect(res.Patches).To(Equal(expectedPatches))
			},
			Entry("should set enabled=true when the FG is enabled and storage is nil",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName}},
				nil,
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1HyperConvergedStoragePath,
					Value:     map[string]any{"persistentReservationConfiguration": map[string]any{"enabled": true}},
				}},
			),
			Entry("should set enabled=true when the FG is enabled and PRC is nil",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName}},
				&hcov1.StorageConfig{},
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1HyperConvergedPRConfigPath,
					Value:     map[string]any{"enabled": true},
				}},
			),
			Entry("should set enabled=true when the FG is enabled and enabled is unset",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName}},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{},
				},
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1PRConfigEnabledPath,
					Value:     true,
				}},
			),
			Entry("should set enabled=false when the FG is explicitly disabled",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName, State: ptr.To(hcov1fg.Disabled)}},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{},
				},
				true,
				true,
				[]jsonpatch.JsonPatchOperation{{
					Operation: "add",
					Path:      v1PRConfigEnabledPath,
					Value:     false,
				}},
			),
			Entry("should do nothing when the FG is not set",
				hcov1fg.HyperConvergedFeatureGates{},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{},
				},
				true,
				false,
				nil,
			),
			Entry("should do nothing when the field is enabled and the FG is not set",
				hcov1fg.HyperConvergedFeatureGates{},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
						Enabled: ptr.To(true),
					},
				},
				true,
				false,
				nil,
			),
			Entry("should do nothing when the field is disabled and the FG is not set",
				hcov1fg.HyperConvergedFeatureGates{},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
						Enabled: ptr.To(false),
					},
				},
				true,
				false,
				nil,
			),
			Entry("should warn if the FG is enabled and the enabled field is true (agree)",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName}},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
						Enabled: ptr.To(true),
					},
				},
				true,
				true,
				nil,
			),
			Entry("should reject if the FG is enabled and the enabled field is false (contradict)",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName}},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
						Enabled: ptr.To(false),
					},
				},
				false,
				false,
				nil,
			),
			Entry("should reject if the FG is disabled and the enabled field is true (contradict)",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName, State: ptr.To(hcov1fg.Disabled)}},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
						Enabled: ptr.To(true),
					},
				},
				false,
				false,
				nil,
			),
			Entry("should warn if the FG is disabled and the enabled field is false (agree)",
				hcov1fg.HyperConvergedFeatureGates{{Name: persistentReservationFGName, State: ptr.To(hcov1fg.Disabled)}},
				&hcov1.StorageConfig{
					PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
						Enabled: ptr.To(false),
					},
				},
				true,
				true,
				nil,
			),
		)
	})

	Context("sync the persistentReservation FG and persistentReservationConfiguration.enabled field on update", func() {
		/*
			Same 81-case matrix as MDev, but with same-direction semantics:
			- Agreement: FG Enabled + field=true, or FG Disabled + field=false
			- Contradiction: FG Enabled + field=false, or FG Disabled + field=true
		*/

		nilFG := hcov1fg.HyperConvergedFeatureGates{}
		enabledFG := hcov1fg.HyperConvergedFeatureGates{
			{Name: persistentReservationFGName},
		}
		disabledFG := hcov1fg.HyperConvergedFeatureGates{
			{Name: persistentReservationFGName, State: ptr.To(hcov1fg.Disabled)},
		}

		nilField := &hcov1.StorageConfig{
			PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{},
		}
		enabledField := &hcov1.StorageConfig{
			PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
				Enabled: ptr.To(true),
			},
		}
		disabledField := &hcov1.StorageConfig{
			PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{
				Enabled: ptr.To(false),
			},
		}

		testSyncPREnabledAndFG := func(ctx context.Context, origCR, newCR *hcov1.HyperConverged, expectedRes *prExpectedResponse) {
			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, newCR, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(expectedRes.checkAllowed)
			Expect(res.Patches).To(Equal(expectedRes.patches))
			Expect(res.Warnings).To(expectedRes.checkWarning)
		}

		DescribeTable("1st table row: when Enabled field is nil (no change)", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedSetEnabledTrue().WithPRWarning(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedSetEnabledFalse().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should set field = True, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedSetEnabledTrue(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedSetEnabledFalse().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedSetEnabledTrue().WithPRWarning(),
			),
			Entry("should set field = False, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedSetEnabledFalse(),
			),
		)

		DescribeTable("2nd table row: when Enabled field is changed: nil -> True", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should reject, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedRemoveFG(),
			),
			Entry("should reject, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedRemoveFG(),
			),
		)

		DescribeTable("3rd table row: when Enabled field is changed: nil -> False", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedRemoveFG(),
			),
		)

		DescribeTable("4th table row: when Enabled field is changed: True -> nil", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedRemoveFG(),
			),
		)

		DescribeTable("5th table row: when Enabled field is True (no change)", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedSetEnabledFalse().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should do nothing, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = False, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedSetEnabledFalse().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedRemoveFG(),
			),
		)

		DescribeTable("6th table row: when Enabled field is changed: True -> False", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedRemoveFG(),
			),
		)

		DescribeTable("7th table row: when Enabled field is changed: False -> nil", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedReject(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedRemoveFG(),
			),
			Entry("should trigger a warning, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      nilField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should reject, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      nilField,
					},
				},
				prExpectedReject(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      nilField,
					},
				},
				prExpectedRemoveFG(),
			),
		)

		DescribeTable("8th table row: when Enabled field is changed: False -> True", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should reject, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedRemoveFG(),
			),
			Entry("should reject, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedReject(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning, if FG changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should remove FG, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      enabledField,
					},
				},
				prExpectedRemoveFG(),
			),
		)

		DescribeTable("9th table row: when Enabled field is False (no change)", testSyncPREnabledAndFG,
			Entry("should do nothing, if FG is nil (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed nil -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedSetEnabledTrue().WithPRWarning(),
			),
			Entry("should trigger a warning, if FG is changed nil -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Enabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should remove FG, if FG is Enabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedRemoveFG(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed Enabled -> Disabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing().WithPRWarning(),
			),
			Entry("should do nothing, if FG is changed Disabled -> nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: nilFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
			Entry("should trigger a warning + set field = True, if FG is changed Disabled -> Enabled",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedSetEnabledTrue().WithPRWarning(),
			),
			Entry("should do nothing, if FG is Disabled (no change)",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: disabledFG,
						Storage:      disabledField,
					},
				},
				prExpectedDoNothing(),
			),
		)

		DescribeTable("drop FG: check special jsonpatch paths", testSyncPREnabledAndFG,
			Entry("should remove the whole FG array, if only the persistentReservation FG is set",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: persistentReservationFGName},
						},
						Storage: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: persistentReservationFGName},
						},
						Storage: enabledField,
					},
				},
				&prExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove only the persistentReservation FG, if it's the first FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: persistentReservationFGName},
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: ptr.To(hcov1fg.Disabled)},
						},
						Storage: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: persistentReservationFGName},
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: ptr.To(hcov1fg.Disabled)},
						},
						Storage: enabledField,
					},
				},
				&prExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/0",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove only the persistentReservation FG, if it's not the first FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: persistentReservationFGName},
							{Name: "someDisabledFG", State: ptr.To(hcov1fg.Disabled)},
						},
						Storage: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: persistentReservationFGName},
							{Name: "someDisabledFG", State: ptr.To(hcov1fg.Disabled)},
						},
						Storage: enabledField,
					},
				},
				&prExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/1",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),

			Entry("should remove only the persistentReservation FG, if it's the last FG",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: ptr.To(hcov1fg.Disabled)},
							{Name: persistentReservationFGName},
						},
						Storage: nilField,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcov1fg.HyperConvergedFeatureGates{
							{Name: "someEnabledFG"},
							{Name: "someDisabledFG", State: ptr.To(hcov1fg.Disabled)},
							{Name: persistentReservationFGName},
						},
						Storage: enabledField,
					},
				},
				&prExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "remove",
						Path:      "/spec/featureGates/2",
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
		)

		DescribeTable("set Enabled = true: check special jsonpatch paths", testSyncPREnabledAndFG,
			Entry("should set only the Enabled field, if the PersistentReservationConfiguration is not nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage: &hcov1.StorageConfig{
							PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{},
						},
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage: &hcov1.StorageConfig{
							PersistentReservationConfiguration: &hcov1.PersistentReservationConfiguration{},
						},
					},
				},
				&prExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "add",
						Path:      v1PRConfigEnabledPath,
						Value:     true,
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should set the PersistentReservationConfiguration, if it's nil but Storage exists",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      &hcov1.StorageConfig{},
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
						Storage:      &hcov1.StorageConfig{},
					},
				},
				&prExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "add",
						Path:      v1HyperConvergedPRConfigPath,
						Value:     map[string]any{"enabled": true},
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
			Entry("should set the Storage, if it's nil",
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
					},
				},
				&hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: enabledFG,
					},
				},
				&prExpectedResponse{
					patches: []jsonpatch.JsonPatchOperation{{
						Operation: "add",
						Path:      v1HyperConvergedStoragePath,
						Value:     map[string]any{"persistentReservationConfiguration": map[string]any{"enabled": true}},
					}},
					checkAllowed: BeTrue(),
					checkWarning: BeEmpty(),
				},
			),
		)
	})
})

type prExpectedResponse struct {
	checkAllowed gomegatypes.GomegaMatcher
	checkWarning gomegatypes.GomegaMatcher
	patches      []jsonpatch.JsonPatchOperation
}

func (response *prExpectedResponse) WithPRWarning() *prExpectedResponse {
	response.checkWarning = And(
		Not(BeEmpty()),
		ContainElement(fmt.Sprintf(prFGDeprecationMsg, persistentReservationFGName)),
	)
	return response
}

func prExpectedSetEnabledTrue() *prExpectedResponse {
	return &prExpectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      v1PRConfigEnabledPath,
			Value:     true,
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func prExpectedSetEnabledFalse() *prExpectedResponse {
	return &prExpectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      v1PRConfigEnabledPath,
			Value:     false,
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func prExpectedRemoveFG() *prExpectedResponse {
	return &prExpectedResponse{
		patches: []jsonpatch.JsonPatchOperation{{
			Operation: "remove",
			Path:      "/spec/featureGates",
		}},
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
	}
}

func prExpectedDoNothing() *prExpectedResponse {
	return &prExpectedResponse{
		checkAllowed: BeTrue(),
		checkWarning: BeEmpty(),
		patches:      noPatches,
	}
}

func prExpectedReject() *prExpectedResponse {
	return &prExpectedResponse{
		checkAllowed: BeFalse(),
		checkWarning: BeEmpty(),
		patches:      noPatches,
	}
}
