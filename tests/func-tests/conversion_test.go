package tests_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("check v1 <=> v1beta1 API conversion", Label("CONVERSION"), func() {
	var (
		hcKey client.ObjectKey
		cli   client.Client
	)

	BeforeEach(func(ctx context.Context) {
		hcKey = client.ObjectKey{Namespace: tests.InstallNamespace, Name: hcoutil.HyperConvergedName}
		cli = tests.GetControllerRuntimeClient()

		By("Make sure feature gates are with default values")
		restoreFGsToDefault(ctx, cli)
	})

	It("naively read HCO in v1beta1 format", func(ctx context.Context) {
		hcv1beta1 := &hcov1beta1.HyperConverged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hcoutil.HyperConvergedName,
				Namespace: tests.InstallNamespace,
			},
		}

		Expect(cli.Get(ctx, hcKey, hcv1beta1)).To(Succeed())

		hcv1, err := tests.GetHCO(ctx, cli)
		Expect(err).NotTo(HaveOccurred())

		converted := &hcov1.HyperConverged{}
		Expect(hcv1beta1.ConvertTo(converted)).To(Succeed())

		diff := cmp.Diff(hcv1.Spec, converted.Spec)
		if diff != "" {
			GinkgoWriter.Println(diff)
			Fail("v1 HyperConverged should be equal to the v1beta1 converted one")
		}

		By("check that v1 feature gates got the same logical value as v1beta1's")
		v1beta1FGStatus := getCurrentV1Beta1FGStatus(hcv1beta1.Spec.FeatureGates)
		for fgName, fgValue := range v1beta1FGStatus {
			matcher := BeFalseBecause("the %q feature gate is disabled in v1beta1, but enabled in v1", fgName)
			if fgValue {
				matcher = BeTrueBecause("the %q feature gate is enabled in v1beta1, but disabled in v1", fgName)
			}
			Expect(hcv1.Spec.FeatureGates.IsEnabled(fgName)).To(matcher)
		}
	})

	It("should allow set fields in HyperConverged v1beta1", func(ctx context.Context) {
		betaFG := getV1beta1FeatureGate(featuregatedetails.ListBetaFeatureGates)
		alphaFG := getV1beta1FeatureGate(featuregatedetails.ListAlphaFeatureGates)

		var patchFGs []string

		if betaFG != "" {
			GinkgoLogr.Info("found a beta feature gate with a field in v1beta1 API version", "name", betaFG)
			patchFGs = []string{fmt.Sprintf(`%q: false`, betaFG)}
		} else {
			GinkgoLogr.Info("no beta feature gate defined in v1beta1 API version")
		}

		if alphaFG != "" {
			GinkgoLogr.Info("found an alpha feature gate with a field in v1beta1 API version", "name", alphaFG)
			patchFGs = append(patchFGs, fmt.Sprintf(`%q: true`, alphaFG))
		} else {
			GinkgoLogr.Info("no alpha feature gate defined in v1beta1 API version")
		}

		if betaFG == "" && alphaFG == "" {
			// should not happen until dropping the v1beta API version, when we'll drop the whole file anyway
			Skip("no alpha or beta feature gates found in v1beta1 API version; skipping this test")
		}

		DeferCleanup(func(ctx context.Context) {
			By("restore the FGs")
			restoreFGsToDefault(ctx, cli)
		})

		By("patch the HyperConverged to modify two FGs, in v1beta1 format")
		patch := fmt.Appendf(nil, `{"spec":{"featureGates": {%s}}}`, strings.Join(patchFGs, ","))
		GinkgoLogr.Info("patching v1beta1 Feature gates", "path", string(patch))

		hcv1beta1 := &hcov1beta1.HyperConverged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hcoutil.HyperConvergedName,
				Namespace: tests.InstallNamespace,
			},
		}

		Eventually(func(ctx context.Context) error {

			return cli.Patch(ctx, hcv1beta1, client.RawPatch(types.MergePatchType, patch))
		}).WithTimeout(60 * time.Second).
			WithPolling(time.Second).
			WithContext(ctx).
			Should(Succeed())

		By("validate the feature gates in HyperConverged v1 format after the v1beta1 update")
		hcv1 := &hcov1.HyperConverged{}
		Expect(cli.Get(ctx, hcKey, hcv1)).To(Succeed())
		if betaFG != "" {
			Expect(hcv1.Spec.FeatureGates.IsEnabled(betaFG)).To(BeFalseBecause("the %q beta feature gate was disabled using v1beta1 API. it is expected to be 'false' in v1, but it's not", betaFG))
		}

		if alphaFG != "" {
			Expect(hcv1.Spec.FeatureGates.IsEnabled(alphaFG)).To(BeTrueBecause("the %q alpha feature gate was enabled using v1beta1 API. it is expected to be 'true' in v1, but it's not", alphaFG))
		}

		By("Check v1 <==> v1beta1 conversion, with non-empty feature gate list")
		Eventually(func(ctx context.Context) error {
			return cli.Get(ctx, hcKey, hcv1beta1)
		}).WithContext(ctx).
			WithTimeout(60 * time.Second).
			WithPolling(time.Second).
			Should(Succeed())

		converted := &hcov1.HyperConverged{}
		Expect(hcv1beta1.ConvertTo(converted)).To(Succeed())
		diff := cmp.Diff(converted.Spec, hcv1.Spec)
		if diff != "" {
			GinkgoWriter.Println(diff)
			Fail("v1 HyperConverged should be equal to the v1beta1 converted one")
		}
	})
})

func getCurrentV1Beta1FGStatus(fgs hcov1beta1.HyperConvergedFeatureGates) map[string]bool {
	fgMap := make(map[string]bool)

	fgVal := reflect.ValueOf(fgs)
	fgType := reflect.TypeOf(fgs)

	if fgVal.Kind() == reflect.Pointer {
		fgVal = fgVal.Elem()
		fgType = fgType.Elem()
	}

	for i := range fgType.NumField() {
		field := fgType.Field(i) // Type info (name, tags, etc.)
		value := fgVal.Field(i)  // Actual value

		fgName := strings.Split(field.Tag.Get("json"), ",")[0]
		if fgName == "" {
			continue
		}

		var fgValue bool
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				continue
			}

			fgValue = value.Elem().Bool()
		} else {
			fgValue = value.Bool()
		}

		fgMap[fgName] = fgValue
	}

	return fgMap
}

func restoreFGsToDefault(ctx context.Context, cl client.Client) {
	GinkgoHelper()

	hcv1beta1 := &hcov1beta1.HyperConverged{}
	patch := fmt.Appendf(nil, removePathPatchTmplt, "/spec/featureGates")

	Eventually(func(g Gomega, ctx context.Context) {
		hco, err := tests.GetHCO(ctx, cl)
		g.Expect(err).ToNot(HaveOccurred())

		if hco.Spec.FeatureGates == nil {
			return
		}

		g.Expect(tests.PatchHCO(ctx, cl, patch)).To(Succeed())
	}).WithTimeout(2 * time.Second).
		WithPolling(500 * time.Millisecond).
		WithContext(ctx).
		Should(Succeed())

	Eventually(func(g Gomega, ctx context.Context) {
		var err error
		hcv1beta1, err = tests.GetHCOv1beta1(ctx, cl)
		g.Expect(err).NotTo(HaveOccurred())

		v1beta1FGStatus := getCurrentV1Beta1FGStatus(hcv1beta1.Spec.FeatureGates)
		defaultFGs := featuregates.HyperConvergedFeatureGates{}

		for fgName, fgValue := range v1beta1FGStatus {
			fgDefault := defaultFGs.IsEnabled(fgName)
			matcher := BeFalseBecause("the %q feature gate should be disabled by default, but is enabled in v1beta1", fgName)
			if fgDefault {
				matcher = BeTrueBecause("the %q feature gate should be enabled by default, but is disabled in v1beta1", fgName)
			}

			g.Expect(fgValue).To(matcher)
		}
	}).WithTimeout(10 * time.Second).
		WithPolling(500 * time.Millisecond).
		WithContext(ctx).
		Should(Succeed())
}

func isFGExistInV1beta1(fgName string) bool {
	for fld := range reflect.TypeFor[hcov1beta1.HyperConvergedFeatureGates]().Fields() {
		if names := strings.Split(fld.Tag.Get("json"), ","); len(names) > 0 {
			if fgName == names[0] {
				return true
			}
		}
	}

	return false
}

// getV1beta1FeatureGate receives a (function that returns a) list of FG names
// it returns the first FG name that is also exists as a field in v1beta1 FG struct.
func getV1beta1FeatureGate(getter func() []string) string {
	for _, fg := range getter() {
		if isFGExistInV1beta1(fg) {
			return fg
		}
	}

	return ""
}
