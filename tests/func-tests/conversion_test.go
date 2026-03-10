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
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("check v1 <=> v1beta1 API conversion", func() {
	var (
		hcKey client.ObjectKey
		cli   client.Client
	)

	BeforeEach(func() {
		hcKey = client.ObjectKey{Namespace: tests.InstallNamespace, Name: hcoutil.HyperConvergedName}
		cli = tests.GetControllerRuntimeClient()
	})

	It("naively read HCO in v1 format", func(ctx context.Context) {
		hcv1 := &hcov1.HyperConverged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hcoutil.HyperConvergedName,
				Namespace: tests.InstallNamespace,
			},
		}

		Expect(cli.Get(ctx, hcKey, hcv1)).To(Succeed())

		hcv1beta1 := tests.GetHCO(ctx, cli)
		converted := &hcov1.HyperConverged{}
		Expect(hcv1beta1.ConvertTo(converted)).To(Succeed())

		diff := cmp.Diff(hcv1, converted)
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

	It("should allow set fields in HyperConverged v1", func(ctx context.Context) {
		By("Make sure feature gates are with default values")
		restoreFGsToDefault(ctx, cli)

		By("read HyperConverged in v1 format, then update two FGs")
		Eventually(func(g Gomega, ctx context.Context) {
			hcv1 := &hcov1.HyperConverged{}
			g.Expect(cli.Get(ctx, hcKey, hcv1)).To(Succeed())

			hcv1.Spec.FeatureGates.Disable("videoConfig")
			hcv1.Spec.FeatureGates.Enable("downwardMetrics")

			g.Expect(cli.Update(ctx, hcv1)).To(Succeed())
		}).WithTimeout(60 * time.Second).
			WithPolling(time.Second).
			WithContext(ctx).
			Should(Succeed())

		By("read HyperConverged in v1beta1 format after the v1 update")
		hcv1beta1 := &hcov1beta1.HyperConverged{}
		Expect(cli.Get(ctx, hcKey, hcv1beta1)).To(Succeed())
		Expect(hcv1beta1.Spec.FeatureGates.DownwardMetrics).To(HaveValue(BeTrueBecause("downwardMetrics was enabled using v1 API. it is expected to be 'true' in v1beta1, but it's not'")))
		Expect(hcv1beta1.Spec.FeatureGates.VideoConfig).To(HaveValue(BeFalseBecause("videoConfig was disabled using v1 API. it is expected to be 'false' in v1beta1, but it's not'")))

		DeferCleanup(func(ctx context.Context) {
			By("restore the FGs")
			restoreFGsToDefault(ctx, cli)
		})
	})
})

func getCurrentV1Beta1FGStatus(fgs hcov1beta1.HyperConvergedFeatureGates) map[string]bool {
	fgMap := make(map[string]bool)

	fgVal := reflect.ValueOf(fgs)
	fgType := reflect.TypeOf(fgs)

	if fgVal.Kind() == reflect.Ptr {
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
		if value.Kind() == reflect.Ptr {
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
	patch := []byte(fmt.Sprintf(removePathPatchTmplt, "/spec/featureGates"))

	Eventually(func(g Gomega, ctx context.Context) {
		g.Expect(tests.PatchHCO(ctx, cl, patch)).To(Succeed())
	}).WithTimeout(2 * time.Second).
		WithPolling(500 * time.Millisecond).
		WithContext(ctx).
		Should(Succeed())

	Eventually(func(g Gomega, ctx context.Context) {
		hcv1beta1 = tests.GetHCO(ctx, cl)
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
