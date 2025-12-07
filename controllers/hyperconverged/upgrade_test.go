package hyperconverged

import (
	"cmp"
	"context"
	"fmt"
	"os"

	"github.com/blang/semver/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	imagev1 "github.com/openshift/api/image/v1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/reqresolver"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/metrics"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/version"
)

var _ = Describe("Upgrade Mode", func() {
	var (
		oldVersion          string // to be sure to cover v2v CRDs removal during upgrades
		newHCOVersion       string
		oldComponentVersion string
		newComponentVersion string
		expected            *BasicExpected
		origConditions      []metav1.Condition
		okConds             []metav1.Condition
	)

	BeforeEach(func() {
		getClusterInfo := hcoutil.GetClusterInfo
		fakeownresources.OLMV0OwnResourcesMock()

		origOperatorCondVarName := os.Getenv(hcoutil.OperatorConditionNameEnvVar)
		origVirtIOWinContainer := os.Getenv("VIRTIOWIN_CONTAINER")
		origOperatorNS := os.Getenv("OPERATOR_NAMESPACE")
		origVersion := os.Getenv(hcoutil.HcoKvIoVersionName)

		hcoutil.GetClusterInfo = func() hcoutil.ClusterInfo {
			return commontestutils.ClusterInfoMock{}
		}
		Expect(os.Setenv(hcoutil.OperatorConditionNameEnvVar, "OPERATOR_CONDITION")).To(Succeed())
		Expect(os.Setenv("VIRTIOWIN_CONTAINER", commontestutils.VirtioWinImage)).To(Succeed())
		Expect(os.Setenv("OPERATOR_NAMESPACE", namespace)).To(Succeed())
		Expect(os.Setenv(hcoutil.HcoKvIoVersionName, version.Version)).To(Succeed())

		reqresolver.GeneratePlaceHolders()

		newHCOVersion = version.Version
		oldComponentVersion = version.Version

		verComp := semver.MustParse(version.Version)
		verComp.Patch += 3
		newComponentVersion = verComp.String()

		verComp = semver.MustParse(version.Version)
		verComp.Minor--
		oldVersion = verComp.String()

		// this is used for version label and the tests below
		// assumes there is no change in labels. Therefore, it should be
		// set before getBasicDeployment so that the existing resource can
		// have the correct labels
		_ = os.Setenv(hcoutil.HcoKvIoVersionName, newHCOVersion)

		expected = getBasicDeployment()
		origConditions = expected.hco.Status.Conditions
		okConds = expected.hco.Status.Conditions

		expected.kv.Status.ObservedKubeVirtVersion = newComponentVersion
		_ = os.Setenv(hcoutil.KubevirtVersionEnvV, newComponentVersion)

		expected.cdi.Status.ObservedVersion = newComponentVersion
		_ = os.Setenv(hcoutil.CdiVersionEnvV, newComponentVersion)

		expected.cna.Status.ObservedVersion = newComponentVersion
		_ = os.Setenv(hcoutil.CnaoVersionEnvV, newComponentVersion)

		_ = os.Setenv(hcoutil.SspVersionEnvV, newComponentVersion)
		expected.ssp.Status.ObservedVersion = newComponentVersion

		_ = os.Setenv(hcoutil.AaqVersionEnvV, newComponentVersion)

		expected.hco.Status.Conditions = origConditions

		DeferCleanup(func() {
			hcoutil.GetClusterInfo = getClusterInfo
			fakeownresources.ResetOwnResources()

			Expect(os.Setenv(hcoutil.OperatorConditionNameEnvVar, origOperatorCondVarName)).To(Succeed())
			Expect(os.Setenv("VIRTIOWIN_CONTAINER", origVirtIOWinContainer)).To(Succeed())
			Expect(os.Setenv("OPERATOR_NAMESPACE", origOperatorNS)).To(Succeed())
			Expect(os.Setenv(hcoutil.HcoKvIoVersionName, origVersion)).To(Succeed())
		})
	})

	It("Should update OperatorCondition Upgradeable to False", func() {
		_ = commontestutils.GetScheme() // ensure the scheme is loaded so this test can be focused

		// old HCO Version is set
		UpdateVersion(&expected.hco.Status, hcoVersionName, oldVersion)

		cl := expected.initClient()
		r := initReconciler(cl, nil)

		r.ownVersion = cmp.Or(os.Getenv(hcoutil.HcoKvIoVersionName), version.Version)

		_, err := r.Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())

		validateOperatorCondition(r, metav1.ConditionFalse, hcoutil.UpgradeableUpgradingReason, hcoutil.UpgradeableUpgradingMessage)
	})

	It("Should update HCO Version Id in the CR on init", func() {

		expected.hco.Status.Conditions = nil

		cl := expected.initClient()
		foundResource, _, requeue := doReconcile(cl, expected.hco, nil)
		Expect(requeue).To(BeTrue())
		checkAvailability(foundResource, metav1.ConditionFalse)

		for _, cond := range foundResource.Status.Conditions {
			if cond.Type == hcov1beta1.ConditionAvailable {
				Expect(cond.Reason).To(Equal("Init"))
				break
			}
		}
		ver, ok := GetVersion(&foundResource.Status, hcoVersionName)
		Expect(ok).To(BeTrue())
		Expect(ver).To(Equal(newHCOVersion))

		expected.hco.Status.Conditions = okConds
	})

	It("detect upgrade existing HCO Version", func() {
		// old HCO Version is set
		UpdateVersion(&expected.hco.Status, hcoVersionName, oldVersion)

		// CDI is not ready
		expected.cdi.Status.Conditions = getGenericProgressingConditions()

		cl := expected.initClient()
		foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
		Expect(requeue).To(BeTrue())
		checkAvailability(foundResource, metav1.ConditionFalse)
		// check that the HCO version is not set, because upgrade is not completed
		ver, ok := GetVersion(&foundResource.Status, hcoVersionName)
		Expect(ok).To(BeTrue())
		Expect(ver).To(Equal(oldVersion))

		// ensure we are not hot-looping setting the version
		_, reconciler, requeue = doReconcile(cl, expected.hco, reconciler)
		Expect(requeue).To(BeFalse())

		validateOperatorCondition(reconciler, metav1.ConditionFalse, hcoutil.UpgradeableUpgradingReason, hcoutil.UpgradeableUpgradingMessage)

		// now, complete the upgrade
		expected.cdi.Status.Conditions = getGenericCompletedConditions()
		cl = expected.initClient()
		foundResource, reconciler, requeue = doReconcile(cl, expected.hco, reconciler)
		Expect(requeue).To(BeTrue())
		checkAvailability(foundResource, metav1.ConditionTrue)

		ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
		Expect(ok).To(BeTrue())
		Expect(ver).To(Equal(oldVersion))
		cond := apimetav1.FindStatusCondition(foundResource.Status.Conditions, hcov1beta1.ConditionProgressing)
		Expect(cond.Status).To(BeEquivalentTo(metav1.ConditionTrue))

		// Call again, to start complete the upgrade
		// check that the image Id is set, now, when upgrade is completed
		foundResource, reconciler, requeue = doReconcile(cl, expected.hco, reconciler)
		Expect(requeue).To(BeFalse())
		checkAvailability(foundResource, metav1.ConditionTrue)

		ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
		Expect(ok).To(BeTrue())
		Expect(ver).To(Equal(newHCOVersion))
		cond = apimetav1.FindStatusCondition(foundResource.Status.Conditions, hcov1beta1.ConditionProgressing)
		Expect(cond.Status).To(BeEquivalentTo(metav1.ConditionFalse))
		validateOperatorCondition(reconciler, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)

		// Call again, to start complete the upgrade
		// check that the image Id is set, now, when upgrade is completed
		_, _, requeue = doReconcile(cl, expected.hco, reconciler)
		Expect(requeue).To(BeFalse())
		validateOperatorCondition(reconciler, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
	})

	It("don't increase the overwrittenModifications metric during upgrade", func() {
		// old HCO Version is set
		UpdateVersion(&expected.hco.Status, hcoVersionName, oldVersion)

		// CDI is not ready
		expected.cdi.Status.Conditions = getGenericProgressingConditions()
		expected.cdi.Spec.Config.FeatureGates = []string{"fake_feature_gate"}

		cl := expected.initClient()
		r := initReconciler(cl, nil)

		rq := reqresolver.GetSecondaryCRRequest()

		counterValueBefore, err := metrics.GetOverwrittenModificationsCount(expected.cdi.Kind, expected.cdi.Name)
		Expect(err).ToNot(HaveOccurred())

		result, err := r.Reconcile(context.Background(), rq)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(requeueAfter))

		foundHC := &hcov1beta1.HyperConverged{}
		Expect(
			cl.Get(context.TODO(),
				types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
				foundHC),
		).ToNot(HaveOccurred())

		// check that the HCO version is not set, because upgrade is not completed
		ver, ok := GetVersion(&foundHC.Status, hcoVersionName)
		Expect(ok).To(BeTrue())
		Expect(ver).To(Equal(oldVersion))

		counterValueAfter, err := metrics.GetOverwrittenModificationsCount(expected.cdi.Kind, expected.cdi.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(counterValueAfter).To(Equal(counterValueBefore))
	})

	DescribeTable(
		"be tolerant parsing parse version",
		func(testHcoVersion string, acceptableVersion bool, errorMessage string) {
			foundResource := &hcov1beta1.HyperConverged{}
			UpdateVersion(&expected.hco.Status, hcoVersionName, testHcoVersion)

			cl := expected.initClient()

			r := initReconciler(cl, nil)
			r.firstLoop = false
			r.ownVersion = newHCOVersion

			res, err := r.Reconcile(context.TODO(), request)
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: request.Name, Namespace: request.Namespace},
					foundResource),
			).To(Succeed())
			ver, ok := GetVersion(&foundResource.Status, hcoVersionName)

			if acceptableVersion {
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(requeueAfter))
				Expect(ok).To(BeTrue())
				Expect(ver).To(Equal(testHcoVersion))
				// reconcile again to complete the upgrade
				res, err = r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(BeZero())
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: request.Name, Namespace: request.Namespace},
						foundResource),
				).To(Succeed())
				ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
				Expect(ok).To(BeTrue())
				Expect(ver).To(Equal(newHCOVersion))
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
				Expect(res.RequeueAfter).To(Equal(requeueAfter))
				Expect(ok).To(BeTrue())
				Expect(ver).To(Equal(testHcoVersion))
				// try a second time
				res, err = r.Reconcile(context.TODO(), request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
				Expect(res.RequeueAfter).To(Equal(requeueAfter))
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: request.Name, Namespace: request.Namespace},
						foundResource),
				).To(Succeed())
				ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
				Expect(ok).To(BeTrue())
				Expect(ver).To(Equal(testHcoVersion))
				// and a third
				res, err = r.Reconcile(context.TODO(), request)
				Expect(err).To(MatchError(ContainSubstring(errorMessage)))
				Expect(res.RequeueAfter).To(Equal(requeueAfter))
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: request.Name, Namespace: request.Namespace},
						foundResource),
				).To(Succeed())
				ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
				Expect(ok).To(BeTrue())
				Expect(ver).To(Equal(testHcoVersion))
			}
		},
		Entry(
			"semver",
			"1.11.0",
			true,
			"",
		),
		Entry(
			"semver with leading spaces",
			"  1.11.0",
			true,
			"",
		),
		Entry(
			"semver with trailing spaces",
			"1.11.0  ",
			true,
			"",
		),
		Entry(
			"semver with leading and trailing spaces",
			"  1.11.0  ",
			true,
			"",
		),
		Entry(
			"quasi semver with leading v",
			"  v1.11.0  ",
			true,
			"",
		),
		Entry(
			"quasi semver with leading v",
			"v1.11.0",
			true,
			"",
		),
		Entry(
			"only major and minor",
			"1.11",
			true,
			"",
		),
		Entry(
			"only major",
			"1",
			true,
			"",
		),
		Entry(
			"only major with leading v",
			"v1",
			true,
			"",
		),
		Entry(
			"additional zeros",
			"0000001.0000012.000000",
			true,
			"",
		),
		Entry(
			"negative numbers",
			"-1.7.0",
			false,
			"Invalid character(s) found in major number",
		),
		Entry(
			"additional dots",
			"1...12..0",
			false,
			"invalid syntax",
		),
		Entry(
			"x.y.z",
			"x.y.z",
			false,
			"Invalid character(s) found in",
		),
		Entry(
			"completely broken version",
			"completelyBrokenVersion",
			false,
			"Invalid character(s) found in major number",
		),
	)

	It("detect upgrade w/o HCO Version", func() {
		// CDI is not ready
		expected.cdi.Status.Conditions = getGenericProgressingConditions()
		expected.hco.Status.Versions = nil

		cl := expected.initClient()
		foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
		Expect(requeue).To(BeTrue())
		checkAvailability(foundResource, metav1.ConditionFalse)

		expected.hco = foundResource
		cl = expected.initClient()
		foundResource, reconciler, requeue = doReconcile(cl, expected.hco, reconciler)
		Expect(requeue).To(BeFalse())
		checkAvailability(foundResource, metav1.ConditionFalse)

		// check that the image Id is not set, because upgrade is not completed
		ver, ok := GetVersion(&foundResource.Status, hcoVersionName)
		_, _ = fmt.Fprintln(GinkgoWriter, "foundResource.Status.Versions", foundResource.Status.Versions)
		Expect(ok).To(BeFalse())
		Expect(ver).To(BeEmpty())

		// now, complete the upgrade
		expected.cdi.Status.Conditions = getGenericCompletedConditions()
		expected.hco = foundResource
		cl = expected.initClient()
		foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
		Expect(requeue).To(BeFalse())
		checkAvailability(foundResource, metav1.ConditionTrue)

		_, ok = GetVersion(&foundResource.Status, hcoVersionName)
		Expect(ok).To(BeTrue())
		cond := apimetav1.FindStatusCondition(foundResource.Status.Conditions, hcov1beta1.ConditionProgressing)
		Expect(cond.Status).To(BeEquivalentTo(metav1.ConditionFalse))

		ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
		Expect(ok).To(BeTrue())
		Expect(ver).To(Equal(newHCOVersion))

		cond = apimetav1.FindStatusCondition(foundResource.Status.Conditions, hcov1beta1.ConditionProgressing)
		Expect(cond.Status).To(BeEquivalentTo(metav1.ConditionFalse))
	})

	DescribeTable(
		"don't complete upgrade if a component version is not match to the component's version env ver",
		func(makeComponentNotReady, makeComponentReady, updateComponentVersion func()) {
			_ = os.Setenv(hcoutil.HcoKvIoVersionName, newHCOVersion)

			// old HCO Version is set
			UpdateVersion(&expected.hco.Status, hcoVersionName, oldVersion)

			makeComponentNotReady()

			cl := expected.initClient()
			foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionFalse)

			expected.hco = foundResource
			cl = expected.initClient()
			foundResource, reconciler, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionFalse)

			// check that the image Id is not set, because upgrade is not completed
			ver, ok := GetVersion(&foundResource.Status, hcoVersionName)
			Expect(ok).To(BeTrue())
			Expect(ver).To(Equal(oldVersion))
			cond := apimetav1.FindStatusCondition(foundResource.Status.Conditions, hcov1beta1.ConditionProgressing)
			Expect(cond.Status).To(BeEquivalentTo(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal("HCOUpgrading"))
			Expect(cond.Message).To(Equal("HCO is now upgrading to version " + newHCOVersion))

			// system health should remain healthy during upgrade progression
			verifySystemHealthStatusHealthy(foundResource)

			// check that the upgrade is not done if the not all the versions are match.
			// Conditions are valid
			makeComponentReady()

			expected.hco = foundResource
			cl = expected.initClient()
			foundResource, reconciler, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionTrue)

			// check that the image Id is set, now, when upgrade is completed
			ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
			Expect(ok).To(BeTrue())
			Expect(ver).To(Equal(oldVersion))
			cond = apimetav1.FindStatusCondition(foundResource.Status.Conditions, hcov1beta1.ConditionProgressing)
			Expect(cond.Status).To(BeEquivalentTo(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal("HCOUpgrading"))
			Expect(cond.Message).To(Equal("HCO is now upgrading to version " + newHCOVersion))

			// system health should remain healthy during upgrade progression
			verifySystemHealthStatusHealthy(foundResource)

			// now, complete the upgrade
			updateComponentVersion()

			expected.hco = foundResource
			cl = expected.initClient()
			foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionTrue)

			// check that the image Id is set, now, when upgrade is completed
			ver, ok = GetVersion(&foundResource.Status, hcoVersionName)
			Expect(ok).To(BeTrue())
			Expect(ver).To(Equal(newHCOVersion))
			cond = apimetav1.FindStatusCondition(foundResource.Status.Conditions, hcov1beta1.ConditionProgressing)
			Expect(cond.Status).To(BeEquivalentTo(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("ReconcileCompleted"))
		},
		Entry(
			"don't complete upgrade if kubevirt version is not match to the kubevirt version env ver",
			func() {
				expected.kv.Status.ObservedKubeVirtVersion = oldComponentVersion
				expected.kv.Status.Conditions[0].Status = "False"
			},
			func() {
				expected.kv.Status.Conditions[0].Status = "True"
			},
			func() {
				expected.kv.Status.ObservedKubeVirtVersion = newComponentVersion
			},
		),
		Entry(
			"don't complete upgrade if CDI version is not match to the CDI version env ver",
			func() {
				expected.cdi.Status.ObservedVersion = oldComponentVersion
				// CDI is not ready
				expected.cdi.Status.Conditions = getGenericProgressingConditions()
			},
			func() {
				// CDI is now ready
				expected.cdi.Status.Conditions = getGenericCompletedConditions()
			},
			func() {
				expected.cdi.Status.ObservedVersion = newComponentVersion
			},
		),
		Entry(
			"don't complete upgrade if CNA version is not match to the CNA version env ver",
			func() {
				expected.cna.Status.ObservedVersion = oldComponentVersion
				// CNA is not ready
				expected.cna.Status.Conditions = getGenericProgressingConditions()
			},
			func() {
				// CNA is now ready
				expected.cna.Status.Conditions = getGenericCompletedConditions()
			},
			func() {
				expected.cna.Status.ObservedVersion = newComponentVersion
			},
		),
	)

	Context("Amend bad defaults", func() {
		const (
			badBandwidthPerMigration    = "64Mi"
			customBandwidthPerMigration = "32Mi"
		)

		It("should drop spec.livemigrationconfig.bandwidthpermigration if == 64Mi when upgrading from < 1.5.0", func() {
			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.4.99")
			expected.hco.Spec.LiveMigrationConfig.BandwidthPerMigration = ptr.To(badBandwidthPerMigration)

			cl := expected.initClient()
			_, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			foundResource, _, requeue := doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeTrue())
			_, _, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			Expect(foundResource.Spec.LiveMigrationConfig.BandwidthPerMigration).To(BeNil())
		})

		It("should preserve spec.livemigrationconfig.bandwidthpermigration if != 64Mi when upgrading from < 1.5.0", func() {
			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.4.99")
			expected.hco.Spec.LiveMigrationConfig.BandwidthPerMigration = ptr.To(customBandwidthPerMigration)

			cl := expected.initClient()
			_, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			foundResource, _, requeue := doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			Expect(foundResource.Spec.LiveMigrationConfig.BandwidthPerMigration).To(HaveValue(Equal(customBandwidthPerMigration)))
		})

		It("should preserve spec.livemigrationconfig.bandwidthpermigration even if == 64Mi when upgrading from >= 1.5.1", func() {
			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.5.1")
			expected.hco.Spec.LiveMigrationConfig.BandwidthPerMigration = ptr.To(badBandwidthPerMigration)

			cl := expected.initClient()
			_, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			foundResource, _, requeue := doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			Expect(foundResource.Spec.LiveMigrationConfig.BandwidthPerMigration).To(HaveValue(Equal(badBandwidthPerMigration)))
		})
	})

	Context("remove old quickstart guides", func() {
		It("should drop old quickstart guide", func() {
			const oldQSName = "old-quickstart-guide"
			UpdateVersion(&expected.hco.Status, hcoVersionName, oldVersion)

			oldQs := &consolev1.ConsoleQuickStart{
				ObjectMeta: metav1.ObjectMeta{
					Name: oldQSName,
					Labels: map[string]string{
						hcoutil.AppLabel:          expected.hco.Name,
						hcoutil.AppLabelManagedBy: hcoutil.OperatorName,
					},
				},
			}

			kvRef, err := reference.GetReference(commontestutils.GetScheme(), expected.kv)
			Expect(err).ToNot(HaveOccurred())
			Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, *kvRef)).ToNot(HaveOccurred())

			oldQsRef, err := reference.GetReference(commontestutils.GetScheme(), oldQs)
			Expect(err).ToNot(HaveOccurred())
			Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, *oldQsRef)).ToNot(HaveOccurred())

			resources := append(expected.toArray(), oldQs)

			cl := commontestutils.InitClient(resources)
			foundResource, _, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundOldQs := &consolev1.ConsoleQuickStart{
				ObjectMeta: metav1.ObjectMeta{
					Name: "old-quickstart-guide",
				},
			}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(oldQs), foundOldQs)).To(HaveOccurred())

			Expect(searchInRelatedObjects(foundResource.Status.RelatedObjects, "ConsoleQuickStart", oldQSName)).To(BeFalse())
		})
	})

	Context("remove old ImageStream guides", func() {
		It("should drop old ImageStream guide", func() {
			const oldISName = "old-imagestream-guide"
			UpdateVersion(&expected.hco.Status, hcoVersionName, oldVersion)

			oldIs := &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      oldISName,
					Namespace: "some-namespace",
					Labels: map[string]string{
						hcoutil.AppLabel:          expected.hco.Name,
						hcoutil.AppLabelManagedBy: hcoutil.OperatorName,
					},
				},
			}

			kvRef, err := reference.GetReference(commontestutils.GetScheme(), expected.kv)
			Expect(err).ToNot(HaveOccurred())
			Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, *kvRef)).ToNot(HaveOccurred())

			oldQsRef, err := reference.GetReference(commontestutils.GetScheme(), oldIs)
			Expect(err).ToNot(HaveOccurred())
			Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, *oldQsRef)).ToNot(HaveOccurred())

			resources := append(expected.toArray(), oldIs)

			cl := commontestutils.InitClient(resources)
			foundResource, _, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundOldQs := &consolev1.ConsoleQuickStart{
				ObjectMeta: metav1.ObjectMeta{
					Name: "old-quickstart-guide",
				},
			}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(oldIs), foundOldQs)).To(HaveOccurred())

			Expect(searchInRelatedObjects(foundResource.Status.RelatedObjects, "ConsoleQuickStart", oldISName)).To(BeFalse())
		})
	})

	Context("remove leftovers on upgrades", func() {

		It("should remove ConfigMap v2v-vmware upgrading from <= 1.6.0", func() {

			cmToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "v2v-vmware",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			cmToBeRemoved2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vm-import-controller-config",
					Namespace: namespace,
				},
			}
			cmNotToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "v2v-vmware",
					Namespace: "different" + namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			cmNotToBeRemoved2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			toBeRemovedRelatedObjects := []corev1.ObjectReference{
				{
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            cmToBeRemoved1.Name,
					Namespace:       cmToBeRemoved1.Namespace,
					ResourceVersion: "999",
				},
			}
			otherRelatedObjects := []corev1.ObjectReference{
				{
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            cmNotToBeRemoved1.Name,
					Namespace:       cmNotToBeRemoved1.Namespace,
					ResourceVersion: "999",
				},
				{
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            cmNotToBeRemoved2.Name,
					Namespace:       cmNotToBeRemoved2.Namespace,
					ResourceVersion: "999",
				},
			}

			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.4.99")

			for _, objRef := range toBeRemovedRelatedObjects {
				Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, objRef)).ToNot(HaveOccurred())
			}
			for _, objRef := range otherRelatedObjects {
				Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, objRef)).ToNot(HaveOccurred())
			}

			resources := append(expected.toArray(), cmToBeRemoved1, cmToBeRemoved2, cmNotToBeRemoved1, cmNotToBeRemoved2)

			cl := commontestutils.InitClient(resources)
			foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundCM := &corev1.ConfigMap{}

			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(cmToBeRemoved1), foundCM)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))

			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(cmToBeRemoved2), foundCM)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))

			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved1), foundCM)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved2), foundCM)).To(Succeed())

			for _, objRef := range toBeRemovedRelatedObjects {
				Expect(foundResource.Status.RelatedObjects).ToNot(ContainElement(objRef))
			}
			for _, objRef := range otherRelatedObjects {
				Expect(foundResource.Status.RelatedObjects).To(ContainElement(objRef))
			}

		})

		It("should not remove ConfigMap v2v-vmware upgrading from >= 1.6.1", func() {

			cmToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "v2v-vmware",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			cmToBeRemoved2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vm-import-controller-config",
					Namespace: namespace,
				},
			}
			cmNotToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "v2v-vmware",
					Namespace: "different" + namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			cmNotToBeRemoved2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.6.1")

			resources := append(expected.toArray(), cmToBeRemoved1, cmToBeRemoved2, cmNotToBeRemoved1, cmNotToBeRemoved2)

			cl := commontestutils.InitClient(resources)
			foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundCM := &corev1.ConfigMap{}
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmToBeRemoved1), foundCM)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmToBeRemoved2), foundCM)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved1), foundCM)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved2), foundCM)).To(Succeed())
		})

		It("should remove ConfigMap kubevirt-storage-class-defaults upgrading from < 1.7.0", func() {
			cmToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubevirt-storage-class-defaults",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			roleToBeRemoved := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hco.kubevirt.io:config-reader",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			roleBindingToBeRemoved := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hco.kubevirt.io:config-reader",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			cmNotToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubevirt-storage-class-defaults",
					Namespace: "different" + namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			cmNotToBeRemoved2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			toBeRemovedRelatedObjects := []corev1.ObjectReference{
				{
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            cmToBeRemoved1.Name,
					Namespace:       cmToBeRemoved1.Namespace,
					ResourceVersion: "999",
				},
				{
					APIVersion:      "rbac.authorization.k8s.io/v1",
					Kind:            "Role",
					Name:            roleToBeRemoved.Name,
					Namespace:       roleToBeRemoved.Namespace,
					ResourceVersion: "999",
				},
				{
					APIVersion:      "rbac.authorization.k8s.io/v1",
					Kind:            "RoleBinding",
					Name:            roleBindingToBeRemoved.Name,
					Namespace:       roleBindingToBeRemoved.Namespace,
					ResourceVersion: "999",
				},
			}
			otherRelatedObjects := []corev1.ObjectReference{
				{
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            cmNotToBeRemoved1.Name,
					Namespace:       cmNotToBeRemoved1.Namespace,
					ResourceVersion: "999",
				},
				{
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            cmNotToBeRemoved2.Name,
					Namespace:       cmNotToBeRemoved2.Namespace,
					ResourceVersion: "999",
				},
			}

			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.6.9")

			for _, objRef := range toBeRemovedRelatedObjects {
				Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, objRef)).ToNot(HaveOccurred())
			}
			for _, objRef := range otherRelatedObjects {
				Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, objRef)).ToNot(HaveOccurred())
			}

			resources := append(expected.toArray(), cmToBeRemoved1, roleToBeRemoved, roleBindingToBeRemoved, cmNotToBeRemoved1, cmNotToBeRemoved2)

			cl := commontestutils.InitClient(resources)
			foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundCM := &corev1.ConfigMap{}
			foundRole := &rbacv1.Role{}
			foundRoleBinding := &rbacv1.RoleBinding{}

			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(cmToBeRemoved1), foundCM)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))

			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(roleToBeRemoved), foundRole)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))

			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(roleBindingToBeRemoved), foundRoleBinding)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))

			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved1), foundCM)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved2), foundCM)).To(Succeed())

			for _, objRef := range toBeRemovedRelatedObjects {
				Expect(foundResource.Status.RelatedObjects).ToNot(ContainElement(objRef))
			}
			for _, objRef := range otherRelatedObjects {
				Expect(foundResource.Status.RelatedObjects).To(ContainElement(objRef))
			}

		})

		It("should not remove ConfigMap kubevirt-storage-class-defaults upgrading from > 1.7.0", func() {
			cmToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubevirt-storage-class-defaults",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			roleToBeRemoved := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hco.kubevirt.io:config-reader",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}
			roleBindingToBeRemoved := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hco.kubevirt.io:config-reader",
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
					Namespace: namespace,
				},
			}
			cmNotToBeRemoved1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubevirt-storage-class-defaults",
					Namespace: "different" + namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			cmNotToBeRemoved2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.7.1")

			resources := append(expected.toArray(), cmToBeRemoved1, roleToBeRemoved, roleBindingToBeRemoved, cmNotToBeRemoved1, cmNotToBeRemoved2)

			cl := commontestutils.InitClient(resources)
			foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundCM := &corev1.ConfigMap{}
			foundRole := &rbacv1.Role{}
			foundRoleBinding := &rbacv1.RoleBinding{}

			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmToBeRemoved1), foundCM)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(roleToBeRemoved), foundRole)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(roleBindingToBeRemoved), foundRoleBinding)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved1), foundCM)).To(Succeed())
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved2), foundCM)).To(Succeed())
		})

		It("should remove TTO CRD upgrading from < 1.10.0", func() {
			crdToBeRemoved := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tektontasks.tektontasks.kubevirt.io",
				},
			}

			cmNotToBeRemoved := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other",
					Namespace: namespace,
					Labels: map[string]string{
						hcoutil.AppLabel: expected.hco.Name,
					},
				},
			}

			toBeRemovedRelatedObjects := []corev1.ObjectReference{
				{
					APIVersion:      "tektontasks.kubevirt.io/v1alpha1",
					Kind:            "TektonTasks",
					Name:            "tto-kubevirt-hyperconverged",
					Namespace:       "kubevirt-hyperconverged",
					ResourceVersion: "999",
				},
			}
			otherRelatedObjects := []corev1.ObjectReference{
				{
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            cmNotToBeRemoved.Name,
					Namespace:       cmNotToBeRemoved.Namespace,
					ResourceVersion: "999",
				},
			}

			UpdateVersion(&expected.hco.Status, hcoVersionName, "1.9.0")

			for _, objRef := range toBeRemovedRelatedObjects {
				Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, objRef)).ToNot(HaveOccurred())
			}
			for _, objRef := range otherRelatedObjects {
				Expect(objectreferencesv1.SetObjectReference(&expected.hco.Status.RelatedObjects, objRef)).ToNot(HaveOccurred())
			}

			resources := append(expected.toArray(), crdToBeRemoved, cmNotToBeRemoved)

			cl := commontestutils.InitClient(resources)
			restMapper := cl.RESTMapper()
			Expect(restMapper).ToNot(BeNil())
			dRestMapper := restMapper.(*apimetav1.DefaultRESTMapper)
			dRestMapper.AddSpecific(
				schema.GroupVersionKind{Group: "tektontasks.kubevirt.io", Version: "v1alpha1", Kind: "TektonTasks"},
				schema.GroupVersionResource{Group: "tektontasks.kubevirt.io", Version: "v1alpha1", Resource: "tektontasks"},
				schema.GroupVersionResource{Group: "tektontasks.kubevirt.io", Version: "v1alpha1", Resource: "tektontask"},
				apimetav1.RESTScopeNamespace)

			foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
			Expect(requeue).To(BeTrue())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
			Expect(requeue).To(BeFalse())
			checkAvailability(foundResource, metav1.ConditionTrue)

			foundCRD := &apiextensionsv1.CustomResourceDefinition{}

			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(crdToBeRemoved), foundCRD)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))

			foundCM := &corev1.ConfigMap{}
			Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cmNotToBeRemoved), foundCM)).To(Succeed())

			for _, objRef := range toBeRemovedRelatedObjects {
				Expect(foundResource.Status.RelatedObjects).ToNot(ContainElement(objRef))
			}
			for _, objRef := range otherRelatedObjects {
				Expect(foundResource.Status.RelatedObjects).To(ContainElement(objRef))
			}

		})

	})

	Context("remove old NetworkPolicies", func() {
		It("should drop old NetworkPolicies guide", func(ctx context.Context) {
			upToDateNP1 := &v1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "up-to-date-1",
					Namespace: namespace,
					Labels: map[string]string{
						npVersionLabel: version.Version,
					},
				},
			}

			upToDateNP2 := &v1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "up-to-date-2",
					Namespace: namespace,
					Labels: map[string]string{
						npVersionLabel: version.Version,
					},
				},
			}

			nonOLMNP := &v1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-olm-np",
					Namespace: namespace,
				},
			}

			oldNP := &v1.NetworkPolicy{ // only this one should be removed
				ObjectMeta: metav1.ObjectMeta{
					Name:      "old-should-be-removed",
					Namespace: namespace,
					Labels: map[string]string{
						npVersionLabel: oldVersion,
					},
				},
			}

			oldNPOtherNamespace := &v1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "old-other-ns-should-not-be-removed",
					Namespace: "other-ns",
					Labels: map[string]string{
						npVersionLabel: version.Version,
					},
				},
			}

			UpdateVersion(&expected.hco.Status, hcoVersionName, oldVersion)

			resources := append(expected.toArray(), upToDateNP1, upToDateNP2, nonOLMNP, oldNP, oldNPOtherNamespace)

			cl := commontestutils.InitClient(resources)
			doReconcile(cl, expected.hco, nil)

			foundNPs := &v1.NetworkPolicyList{}
			Expect(cl.List(ctx, foundNPs)).To(Succeed())

			Expect(foundNPs.Items).To(HaveLen(4))
			Expect(foundNPs.Items).To(ContainElements(*upToDateNP1, *upToDateNP2, *nonOLMNP, *oldNPOtherNamespace))
			Expect(foundNPs.Items).ToNot(ContainElements(*oldNP))
		})
	})
})
