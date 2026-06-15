package tests

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
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

// PatchHCO updates the HCO CR using a DynamicClient, it can return errors on failures
func PatchHCO(ctx context.Context, cli client.Client, patchBytes []byte) error {
	patch := client.RawPatch(types.JSONPatchType, patchBytes)
	hco := HCOWithNameOnly()

	return cli.Patch(ctx, hco, patch)
}

// PatchMergeHCO patches the HCO CR using a DynamicClient, it can return errors on failures
func PatchMergeHCO(ctx context.Context, cli client.Client, patchBytes []byte) error {
	patch := client.RawPatch(types.MergePatchType, patchBytes)
	hco := HCOWithNameOnly()

	return cli.Patch(ctx, hco, patch)
}

func RestoreDefaults(ctx context.Context, cli client.Client) {
	Eventually(func(ctx context.Context) error {
		return PatchHCO(ctx, cli, []byte(`[{"op": "replace", "path": "/spec", "value": {}}]`))
	}).
		WithOffset(1).
		WithTimeout(time.Second * 5).
		WithPolling(time.Millisecond * 100).
		WithContext(ctx).
		Should(Succeed())
}

func EnableFG(ctx context.Context, cli client.Client, fgName string) error {
	hc, err := GetHCO(ctx, cli)
	if err != nil {
		return err
	}

	if hc.Spec.FeatureGates == nil {
		patch := fmt.Appendf(nil, `{"spec":{"featureGates":[{"name": %q, "state": "%v"}]}}`, fgName, featuregates.Enabled)
		return PatchMergeHCO(ctx, cli, patch)
	}

	if !hc.Spec.FeatureGates.IsEnabled(fgName) {
		var patch []byte
		idx := slices.IndexFunc(hc.Spec.FeatureGates, func(fg featuregates.FeatureGate) bool {
			return fg.Name == fgName
		})
		if idx == -1 {
			patch = fmt.Appendf(nil, `[{"op": "add", "path": "/spec/featureGates/-", "value": {"name": %q}}]`, fgName)
		} else {
			patch = fmt.Appendf(nil, `[{"op": "replace", "path": "/spec/featureGates/%d/state", "value": "%v"}]`, idx, featuregates.Enabled)
		}

		return PatchHCO(ctx, cli, patch)
	}

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
