package v1alpha1

import (
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

const (
	testName       = "aName"
	testVersion    = "aVersion"
	testOldVersion = "anOldVersion"
)

func TestHyperConvergedStatus_UpdateVersion_noVersions(t *testing.T) {
	hcs := &HyperConvergedStatus{Conditions: []conditionsv1.Condition{}, RelatedObjects: []corev1.ObjectReference{}}

	hcs.UpdateVersion(testName, testVersion)

	if len(hcs.Versions) != 1 {
		t.Error("Should be able to add a new version to a nil version array")
	}

	if hcs.Versions[0].Name != testName {
		t.Errorf(`Version name should be "%s" but it's "%s"`, testName, hcs.Versions[0].Name)
	}

	if hcs.Versions[0].Version != testVersion {
		t.Errorf(`Version should be "%s" but it's "%s"`, testVersion, hcs.Versions[0].Version)
	}
}

func TestHyperConvergedStatus_UpdateVersion_emptyVersions(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions:       Versions{},
	}

	hcs.UpdateVersion(testName, testVersion)

	if len(hcs.Versions) != 1 {
		t.Error("Should be able to add a new version to an empty version array")
	}

	if hcs.Versions[0].Name != testName {
		t.Errorf(`Version name should be "%s" but it's "%s"`, testName, hcs.Versions[0].Name)
	}

	if hcs.Versions[0].Version != testVersion {
		t.Errorf(`Version should be "%s" but it's "%s"`, testVersion, hcs.Versions[0].Version)
	}
}

func TestHyperConvergedStatus_UpdateVersion_addNew(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: "aaa", Version: "1.2.3"},
			{Name: "bbb", Version: "4.5.6"},
			{Name: "ccc", Version: "7.8.9"},
		},
	}

	hcs.UpdateVersion(testName, testVersion)

	if len(hcs.Versions) != 4 {
		t.Errorf("Should be able to add a new version to a non-empty version array")
	}

	if hcs.Versions[3].Name != testName {
		t.Errorf(`Version name should be ""%s"" but it's "%s"`, testName, hcs.Versions[3].Name)
	}

	if hcs.Versions[3].Version != testVersion {
		t.Errorf(`Version should be ""%s"" but it's "%s"`, testVersion, hcs.Versions[3].Version)
	}
}

func TestHyperConvergedStatus_UpdateVersion_updateFirst(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: testName, Version: testOldVersion},
			{Name: "bbb", Version: "4.5.6"},
			{Name: "ccc", Version: "7.8.9"},
		},
	}

	hcs.UpdateVersion(testName, testVersion)

	if len(hcs.Versions) != 3 {
		t.Errorf("Should be able to update an existing version; array length should be 3, but it's %d", len(hcs.Versions))
	}

	if hcs.Versions[0].Name != testName {
		t.Errorf(`Version name should be "%s" but it's "%s"`, testName, hcs.Versions[0].Name)
	}

	if hcs.Versions[0].Version != testVersion {
		t.Errorf(`Version should be "%s" but it's "%s"`, testVersion, hcs.Versions[0].Version)
	}
}

func TestHyperConvergedStatus_UpdateVersion_updateMiddle(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: "aaa", Version: "1.2.3"},
			{Name: testName, Version: testOldVersion},
			{Name: "ccc", Version: "7.8.9"},
		},
	}

	hcs.UpdateVersion(testName, testVersion)

	if len(hcs.Versions) != 3 {
		t.Errorf("Should be able to update an existing version; array length should be 3, but it's %d", len(hcs.Versions))
	}

	if hcs.Versions[1].Name != testName {
		t.Errorf(`Version name should be "%s" but it's "%s"`, testName, hcs.Versions[1].Name)
	}

	if hcs.Versions[1].Version != testVersion {
		t.Errorf(`Version should be "%s" but it's "%s"`, testVersion, hcs.Versions[1].Version)
	}
}

func TestHyperConvergedStatus_UpdateVersion_updateLast(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: "aaa", Version: "1.2.3"},
			{Name: "bbb", Version: "4.5.6"},
			{Name: testName, Version: testOldVersion},
		},
	}

	hcs.UpdateVersion(testName, testVersion)

	if len(hcs.Versions) != 3 {
		t.Errorf("Should be able to update an existing version; array length should be 3, but it's %d", len(hcs.Versions))
	}

	if hcs.Versions[2].Name != testName {
		t.Errorf(`Version name should be "%s" but it's "%s"`, testName, hcs.Versions[2].Name)
	}

	if hcs.Versions[2].Version != testVersion {
		t.Errorf(`Version should be "%s" but it's "%s"`, testVersion, hcs.Versions[2].Version)
	}
}

func TestHyperConvergedStatus_GetVersion_nil(t *testing.T) {
	hcs := &HyperConvergedStatus{Conditions: []conditionsv1.Condition{}, RelatedObjects: []corev1.ObjectReference{}}
	ver, ok := hcs.GetVersion(testName)
	if ok || ver != "" {
		t.Error("Should not find the version in empty version array")
	}
}

func TestHyperConvergedStatus_GetVersion_empty(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions:       Versions{},
	}
	ver, ok := hcs.GetVersion(testName)
	if ok || ver != "" {
		t.Error("Should not find the version in empty version array")
	}
}

func TestHyperConvergedStatus_GetVersion_notFound(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: "aaa", Version: "1.2.3"},
			{Name: "bbb", Version: "4.5.6"},
			{Name: "ccc", Version: "7.8.9"},
		},
	}
	ver, ok := hcs.GetVersion(testName)
	if ok || ver != "" {
		t.Error("Should not find the version; it should be missing")
	}
}

func TestHyperConvergedStatus_GetVersion_findFirst(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: testName, Version: testVersion},
			{Name: "bbb", Version: "4.5.6"},
			{Name: "ccc", Version: "7.8.9"},
		},
	}
	ver, ok := hcs.GetVersion(testName)
	if !ok {
		t.Errorf(`version "%s"" should be found`, testName)
	}
	if ver != testVersion {
		t.Errorf(`version "%s"" should be "%s"; but it's %s`, testName, testVersion, ver)
	}
}

func TestHyperConvergedStatus_GetVersion_findMiddle(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: "aaa", Version: "1.2.3"},
			{Name: testName, Version: testVersion},
			{Name: "ccc", Version: "7.8.9"},
		},
	}
	ver, ok := hcs.GetVersion(testName)
	if !ok {
		t.Errorf(`version "%s"" should be found`, testName)
	}
	if ver != testVersion {
		t.Errorf(`version "%s"" should be "%s"; but it's %s`, testName, testVersion, ver)
	}
}

func TestHyperConvergedStatus_GetVersion_findLast(t *testing.T) {
	hcs := &HyperConvergedStatus{
		Conditions:     []conditionsv1.Condition{},
		RelatedObjects: []corev1.ObjectReference{},
		Versions: Versions{
			{Name: "aaa", Version: "1.2.3"},
			{Name: "bbb", Version: "4.5.6"},
			{Name: testName, Version: testVersion},
		},
	}
	ver, ok := hcs.GetVersion(testName)
	if !ok {
		t.Errorf(`version "%s"" should be found`, testName)
	}
	if ver != testVersion {
		t.Errorf(`version "%s"" should be "%s"; but it's %s`, testName, testVersion, ver)
	}
}
