package upgradepatch

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"

	"github.com/blang/semver/v4"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1fg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
)

//go:generate go run ../../tools/crwriter/ --format=json --api-version=v1 --out=./hc.cr.json
//go:embed hc.cr.json
var hcCRBytes []byte

const (
	UpgradeChangesFileLocation = "upgradePatches.json"
)

type hcoCRPatch struct {
	// SemverRange is a set of conditions which specify which versions satisfy the range
	// (see https://github.com/blang/semver#ranges as a reference).
	SemverRange semverRange `json:"semverRange"`
	// JSONPatch contains a sequence of operations to apply to the HCO CR during upgrades
	// (see: https://datatracker.ietf.org/doc/html/rfc6902 as the format reference).
	JSONPatch jsonpatch.Patch `json:"jsonPatch"`

	// jsonPatchApplyOptions specifies options for calls to ApplyWithOptions.
	// jsonpatch.NewApplyOptions defaults are applied if empty.
	JSONPatchApplyOptions *jsonpatch.ApplyOptions `json:"jsonPatchApplyOptions,omitempty"`
}

func (p hcoCRPatch) applyUpgradePatch(logger logr.Logger, hcoJSON []byte, knownHcoSV semver.Version) ([]byte, error) {
	if p.IsAffectedRange(knownHcoSV) {
		buff := &bytes.Buffer{}
		err := json.NewEncoder(buff).Encode(p.JSONPatch)
		if err != nil {
			buff = bytes.NewBuffer([]byte("<unknown>"))
		}

		logger.Info("applying upgrade patch", "knownHcoSV", knownHcoSV, "affectedRange", p.SemverRange.ver, "patches", buff.String(), "applyOptions", p.JSONPatchApplyOptions)
		var (
			patchedBytes []byte
		)
		if p.JSONPatchApplyOptions != nil {
			patchedBytes, err = p.JSONPatch.ApplyWithOptions(hcoJSON, p.JSONPatchApplyOptions)
		} else {
			patchedBytes, err = p.JSONPatch.Apply(hcoJSON)
		}
		if err != nil {
			// tolerate jsonpatch test failures
			if errors.Is(err, jsonpatch.ErrTestFailed) {
				return hcoJSON, nil
			}

			return hcoJSON, err
		}
		return patchedBytes, nil
	}
	return hcoJSON, nil
}

func (p hcoCRPatch) IsAffectedRange(ver semver.Version) bool {
	return p.SemverRange.isAffectedRange(ver)
}

type ObjectToBeRemoved struct {
	// SemverRange is a set of conditions which specify which versions satisfy the range
	// (see https://github.com/blang/semver#ranges as a reference).
	SemverRange semverRange `json:"semverRange"`
	// GroupVersionKind unambiguously identifies the kind of the object to be removed
	GroupVersionKind schema.GroupVersionKind `json:"groupVersionKind"`
	// objectKey contains name and namespace of the object to be removed.
	ObjectKey types.NamespacedName `json:"objectKey"`
}

func (o ObjectToBeRemoved) IsAffectedRange(ver semver.Version) bool {
	return o.SemverRange.isAffectedRange(ver)
}

// DefaultFeatureGate holds a feature gate that its default value should be changed on upgrade
type DefaultFeatureGate struct {
	// SemverRange is a set of conditions which specify which versions satisfy the range
	// (see https://github.com/blang/semver#ranges as a reference).
	SemverRange semverRange `json:"semverRange"`

	// Name holds the FeatureGate name
	Name string `json:"name"`

	// State is the new default state; optional; assuming most cases are moving from alpha to beta
	State *hcov1fg.State `json:"state,omitempty"`
}

func (dfg DefaultFeatureGate) IsAffectedRange(ver semver.Version) bool {
	return dfg.SemverRange.isAffectedRange(ver)
}

func (dfg DefaultFeatureGate) applyDefault(ver semver.Version, hc *hcov1.HyperConverged) {
	if !dfg.IsAffectedRange(ver) {
		return
	}

	idx := hc.Spec.FeatureGates.Index(dfg.Name)
	if idx < 0 { // not explicitly set
		return
	}

	newDefaultState := hcov1fg.Enabled
	if dfg.State != nil {
		newDefaultState = *dfg.State
	}

	if hc.Spec.FeatureGates[idx].State == nil {
		if newDefaultState == hcov1fg.Disabled {
			hc.Spec.FeatureGates[idx].State = new(hcov1fg.Disabled)
		}

		return
	}

	if *hc.Spec.FeatureGates[idx].State != newDefaultState {
		hc.Spec.FeatureGates[idx].State = &newDefaultState
	}
}

func (dfg *DefaultFeatureGate) UnmarshalJSON(dfgBytes []byte) error {
	type _dfg DefaultFeatureGate

	var d = _dfg{}

	err := json.Unmarshal(dfgBytes, &d)
	if err != nil {
		return err
	}

	if d.Name == "" {
		return fmt.Errorf("no feature gate name specified")
	}

	if d.State != nil {
		switch *d.State {
		case hcov1fg.Enabled, hcov1fg.Disabled:
		default:
			return fmt.Errorf(`invalid feature gate state "%v" specified`, *d.State)
		}
	}

	*dfg = DefaultFeatureGate(d)

	return nil
}

type semverRange struct {
	ver string
	fn  semver.Range
}

func newSemverRange(ver string) (semverRange, error) {
	if len(ver) > 0 {
		fn, err := semver.ParseRange(ver)
		if err != nil {
			return semverRange{}, err
		}
		return semverRange{ver: ver, fn: fn}, nil
	}
	return semverRange{}, nil
}

func (o *semverRange) isAffectedRange(ver semver.Version) bool {
	if o.fn == nil {
		return false
	}
	return o.fn(ver)
}

func (o *semverRange) UnmarshalJSON(data []byte) error {
	var ver string
	err := json.Unmarshal(data, &ver)
	if err != nil {
		return err
	}

	vr, err := newSemverRange(ver)
	if err != nil {
		return err
	}

	*o = vr

	return nil
}

func (o *semverRange) MarshalJSON() ([]byte, error) {
	if o.fn == nil {
		return nil, nil
	}

	return json.Marshal(o.ver)
}

type UpgradePatches struct {
	// hcoCRPatchList is a list of upgrade patches.
	// Each hcoCRPatch consists in a semver range of affected source versions and a json patch to be applied during the upgrade if relevant.
	HCOCRPatchList []hcoCRPatch `json:"hcoCRPatchList"`
	// ObjectsToBeRemoved is a list of objects to be removed on upgrades.
	// Each objectToBeRemoved consists in a semver range of affected source versions and schema.GroupVersionKind and types.NamespacedName of the object to be eventually removed during the upgrade.
	ObjectsToBeRemoved []ObjectToBeRemoved `json:"objectsToBeRemoved"`

	// DefaultFeatureGates - holds a list of feature gates that their default value should be changed on upgrade
	DefaultFeatureGates []DefaultFeatureGate `json:"defaultFeatureGates,omitempty"`
}

func (up UpgradePatches) applyUpgradePatch(logger logr.Logger, hc *hcov1.HyperConverged, knownHcoSV semver.Version) (*hcov1.HyperConverged, error) {
	hcoJSON, err := json.Marshal(hc)
	if err != nil {
		return nil, err
	}

	for _, patch := range up.HCOCRPatchList {
		hcoJSON, err = patch.applyUpgradePatch(logger, hcoJSON, knownHcoSV)
		if err != nil {
			return nil, err
		}
	}

	tmpInstance := &hcov1.HyperConverged{}
	err = json.Unmarshal(hcoJSON, tmpInstance)
	if err != nil {
		return nil, err
	}

	return tmpInstance, nil
}

func (up UpgradePatches) applyDefaultFeatureGates(hc *hcov1.HyperConverged, knownHcoSV semver.Version) {
	for _, dfg := range up.DefaultFeatureGates {
		dfg.applyDefault(knownHcoSV, hc)
	}
}

var (
	hcoUpgradeChanges UpgradePatches
	once              = &sync.Once{}
	onceErr           error
)

func ApplyUpgradePatch(logger logr.Logger, hc *hcov1.HyperConverged, knownHcoSV semver.Version) (*hcov1.HyperConverged, error) {
	modified, err := hcoUpgradeChanges.applyUpgradePatch(logger, hc, knownHcoSV)
	if err != nil {
		return nil, err
	}

	hcoUpgradeChanges.applyDefaultFeatureGates(modified, knownHcoSV)
	return modified, nil
}

func GetObjectsToBeRemoved() []ObjectToBeRemoved {
	return hcoUpgradeChanges.ObjectsToBeRemoved
}

func readUpgradePatchesFromFile(pwdFS fs.FS, logger logr.Logger) error {
	file, err := pwdFS.Open(UpgradeChangesFileLocation)
	if err != nil {
		logger.Error(err, "Can't open the upgradeChanges yaml file", "file name", UpgradeChangesFileLocation)
		return err
	}

	defer file.Close()

	return readJsonFromReader(file)
}

func readJsonFromReader(file io.Reader) error {
	hcoUpgradeChanges = UpgradePatches{}

	jDec := json.NewDecoder(file)
	err := jDec.Decode(&hcoUpgradeChanges)
	if err != nil {
		return err
	}

	for _, p := range hcoUpgradeChanges.HCOCRPatchList {
		if err = validateUpgradePatch(p); err != nil {
			return err
		}
	}
	for _, r := range hcoUpgradeChanges.ObjectsToBeRemoved {
		if err = validateUpgradeLeftover(r); err != nil {
			return err
		}
	}

	return nil
}

func Init(pwdFS fs.FS, logger logr.Logger) error {
	once.Do(func() {
		onceErr = readUpgradePatchesFromFile(pwdFS, logger)
	})

	if onceErr != nil {
		return onceErr
	}
	return nil
}

func validateUpgradePatch(p hcoCRPatch) error {
	for _, patch := range p.JSONPatch {
		path, err := patch.Path()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(path, "/spec/") {
			return errors.New("can only modify spec fields")
		}
	}

	var err error
	if p.JSONPatchApplyOptions != nil {
		_, err = p.JSONPatch.ApplyWithOptions(hcCRBytes, p.JSONPatchApplyOptions)
	} else {
		_, err = p.JSONPatch.Apply(hcCRBytes)
	}
	// tolerate jsonpatch test failures
	if err != nil && !errors.Is(errors.Unwrap(err), jsonpatch.ErrTestFailed) {
		return err
	}
	return nil
}

func validateUpgradeLeftover(r ObjectToBeRemoved) error {
	if r.GroupVersionKind.Kind == "" {
		return errors.New("missing object kind")
	}
	if r.GroupVersionKind.Version == "" {
		return errors.New("missing object API version")
	}
	if r.ObjectKey.Name == "" {
		return errors.New("missing object name")
	}
	return nil
}
