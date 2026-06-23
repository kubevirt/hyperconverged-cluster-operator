package tests

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	fgs "github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	HCPatchTimeout = 2 * time.Minute
	HCPatchPolling = 5 * time.Second
)

// GetHCO reads the HCO CR from the APIServer with a DynamicClient
func GetHCO(ctx context.Context, cli client.Client) (*hcov1.HyperConverged, error) {
	hco := HCOWithNameOnly()

	err := cli.Get(ctx, client.ObjectKeyFromObject(hco), hco)
	if err != nil {
		return nil, err
	}

	return hco, nil
}

// GetHCOv1beta1 reads the HCO CR from the APIServer with a DynamicClient
func GetHCOv1beta1(ctx context.Context, cli client.Client) (*hcov1beta1.HyperConverged, error) {
	hco := &hcov1beta1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcoutil.HyperConvergedName,
			Namespace: InstallNamespace,
		},
	}

	err := cli.Get(ctx, client.ObjectKeyFromObject(hco), hco)
	if err != nil {
		return nil, err
	}

	return hco, nil
}

// UpdateHCORetry updates the HCO CR in a safe way internally calling UpdateHCO_old
// UpdateHCORetry internally uses an async Eventually block refreshing the in-memory
// object if needed and setting there Spec, Annotations, Finalizers and Labels from the
// input object.
// UpdateHCORetry should be preferred over UpdateHCO_old to reduce test flakiness due to
// inevitable concurrency conflicts
func UpdateHCORetry(ctx context.Context, cli client.Client, input *hcov1.HyperConverged) *hcov1.HyperConverged {
	ginkgo.GinkgoHelper()
	var output *hcov1.HyperConverged

	Eventually(func(ctx context.Context) error {
		hco, err := GetHCO(ctx, cli)
		if err != nil {
			return err
		}

		input.Spec.DeepCopyInto(&hco.Spec)
		hco.Annotations = maps.Clone(input.Annotations)
		hco.Finalizers = slices.Clone(input.Finalizers)
		hco.Labels = maps.Clone(input.Labels)

		output, err = UpdateHCO(ctx, cli, hco)
		return err
	}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())

	return output
}

// UpdateHCO updates the HCO CR using a DynamicClient, it can return errors on failures
func UpdateHCO(ctx context.Context, cli client.Client, input *hcov1.HyperConverged) (*hcov1.HyperConverged, error) {
	hco, err := GetHCO(ctx, cli)
	if err != nil {
		return nil, err
	}

	input.Spec.DeepCopyInto(&hco.Spec)
	hco.Annotations = input.Annotations
	hco.Finalizers = input.Finalizers
	hco.Labels = input.Labels
	hco.Status = hcov1.HyperConvergedStatus{} // to silence warning about unknown fields.

	err = cli.Update(ctx, hco)
	if err != nil {
		return nil, err
	}

	return GetHCO(ctx, cli)
}

func doPatch(ctx context.Context, cli client.Client, patch client.Patch) {
	ginkgo.GinkgoHelper()

	hco := HCOWithNameOnly()

	Eventually(cli.Patch).
		WithContext(ctx).
		WithArguments(hco, patch).
		WithTimeout(HCPatchTimeout).
		WithPolling(HCPatchPolling).
		Should(Succeed(), func() string {
			b := strings.Builder{}
			// looking at the patch implementation, it never returns error and never uses the object argument
			data, _ := patch.Data(nil)
			b.WriteString("patch: ")
			b.Write(data)
			b.WriteByte('\n')

			hc, err := GetHCO(ctx, cli)
			if err != nil {
				b.WriteString("Can't get the HyperConverged CR; ")
				b.WriteString(err.Error())
			} else {
				b.WriteString("Current HyperConverged CR:\n")
				b.WriteString(marshalHyperConverged(hc))
			}

			b.WriteByte('\n')

			return b.String()
		})

}

// PatchHCO updates the HCO CR using jsonpatch
func PatchHCO(ctx context.Context, cli client.Client, patchBytes []byte) {
	ginkgo.GinkgoHelper()

	patch := client.RawPatch(types.JSONPatchType, patchBytes)

	doPatch(ctx, cli, patch)
}

// PatchMergeHCO patches the HCO CR using merge strategy
func PatchMergeHCO(ctx context.Context, cli client.Client, patchBytes []byte) {
	ginkgo.GinkgoHelper()

	patch := client.RawPatch(types.MergePatchType, patchBytes)

	doPatch(ctx, cli, patch)
}

func RestoreDefaults(ctx context.Context, cli client.Client) {
	ginkgo.GinkgoHelper()

	PatchHCO(ctx, cli, []byte(`[{"op": "replace", "path": "/spec", "value": {}}]`))
}

func EnableFG(ctx context.Context, cli client.Client, fgName string) error {
	hc, err := GetHCO(ctx, cli)
	if err != nil {
		return err
	}

	if hc.Spec.FeatureGates.IsEnabled(fgName) {
		// already enabled
		return nil
	}

	idx := slices.IndexFunc(hc.Spec.FeatureGates, func(fg featuregates.FeatureGate) bool {
		return fg.Name == fgName
	})

	var patch []byte
	switch phase, _ := featuregatedetails.GetFeatureGatePhase(fgName); phase {
	case fgs.PhaseUnknown:
		return fmt.Errorf("unknown feature gate %q", fgName)

	case fgs.PhaseBeta:
		// beta FG are enabled by default. We just need to drop them in order to enable them.
		if idx == -1 {
			// Should never happen
			return fmt.Errorf("unknown issue. the beta feature gate %q is disabled, but is not in spec.featureGates", fgName)
		}

		patch = fmt.Appendf(nil, `[{ "op": "remove", "path": "/spec/featureGates/%d"}]`, idx)

	default:
		// alpha/deprecated FG are false by default. We need to explicitly enable them.
		if idx == -1 {
			if hc.Spec.FeatureGates == nil {
				patch = fmt.Appendf(nil, `[{"op": "add", "path": "/spec/featureGates", "value": [{"name": %q}]}]`, fgName)
			} else {
				patch = fmt.Appendf(nil, `[{"op": "add", "path": "/spec/featureGates/-", "value": {"name": %q}}]`, fgName)
			}
		} else {
			patch = fmt.Appendf(nil, `[{"op": "add", "path": "/spec/featureGates/%d", "value": {"name": %q, "state": "%s"}}]`, idx, fgName, featuregates.Enabled)
		}
	}

	PatchHCO(ctx, cli, patch)
	return nil
}

func DisableFG(ctx context.Context, cli client.Client, fgName string) error {
	hc, err := GetHCO(ctx, cli)
	if err != nil {
		return err
	}

	if !hc.Spec.FeatureGates.IsEnabled(fgName) {
		// already disabled
		return nil
	}

	idx := slices.IndexFunc(hc.Spec.FeatureGates, func(fg featuregates.FeatureGate) bool {
		return fg.Name == fgName
	})

	var patch []byte
	switch phase, _ := featuregatedetails.GetFeatureGatePhase(fgName); phase {
	case fgs.PhaseUnknown:
		return fmt.Errorf("unknown feature gate %q", fgName)

	case fgs.PhaseBeta:
		// beta FG are enabled by default. We need to explicitly disable them.
		if idx == -1 {
			if hc.Spec.FeatureGates == nil {
				patch = fmt.Appendf(nil, `[{"op": "add", "path": "/spec/featureGates", "value": [{"name": %q, "state": "%s"}]}]`, fgName, featuregates.Disabled)
			} else {
				patch = fmt.Appendf(nil, `[{"op": "add", "path": "/spec/featureGates/-", "value": {"name": %q, "state": "%s"}}]`, fgName, featuregates.Disabled)
			}

		} else {
			patch = fmt.Appendf(nil, `[{"op": "add", "path": "/spec/featureGates/%d", "value": {"name": %q, "state": "%s"}}]`, idx, fgName, featuregates.Disabled)
		}

	default:
		// alpha/deprecated FG are false by default. We just need to drop them in order to disable them
		if idx == -1 {
			// Should never happen
			return fmt.Errorf("unknown issue. the alpha feature gate %q is enabled, but is not in spec.featureGates", fgName)
		}

		patch = fmt.Appendf(nil, `[{ "op": "remove", "path": "/spec/featureGates/%d"}]`, idx)
	}

	PatchHCO(ctx, cli, patch)
	return nil
}

func HCOWithNameOnly() *hcov1.HyperConverged {
	return &hcov1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcoutil.HyperConvergedName,
			Namespace: InstallNamespace,
		},
	}
}
