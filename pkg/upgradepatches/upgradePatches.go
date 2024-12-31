package upgradepatches

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/blang/semver/v4"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

//go:generate go run ../../tools/crwriter -out=hco.cr.go --format=go --header=../../hack/boilerplate.go.txt

const (
	upgradeChangesFileLocation = "./upgradePatches.json"
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
		logger.Info("applying upgrade patch", "knownHcoSV", knownHcoSV, "affectedRange", p.SemverRange, "patches", p.JSONPatch, "applyOptions", p.JSONPatchApplyOptions)
		var (
			patchedBytes []byte
			err          error
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

	SemverRangeFunc semver.Range

	// GroupVersionKind unambiguously identifies the kind of the object to be removed
	GroupVersionKind schema.GroupVersionKind `json:"groupVersionKind"`
	// objectKey contains name and namespace of the object to be removed.
	ObjectKey types.NamespacedName `json:"objectKey"`
}

func (o ObjectToBeRemoved) IsAffectedRange(ver semver.Version) bool {
	return o.SemverRange.isAffectedRange(ver)
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
}

func (up UpgradePatches) applyUpgradePatch(logger logr.Logger, hcoJSON []byte, knownHcoSV semver.Version) ([]byte, error) {
	var err error
	for _, patch := range up.HCOCRPatchList {
		hcoJSON, err = patch.applyUpgradePatch(logger, hcoJSON, knownHcoSV)
		if err != nil {
			return nil, err
		}
	}
	return hcoJSON, nil
}

var (
	hcoUpgradeChanges UpgradePatches
)

func ApplyUpgradePatch(logger logr.Logger, hcoJSON []byte, knownHcoSV semver.Version) ([]byte, error) {
	return hcoUpgradeChanges.applyUpgradePatch(logger, hcoJSON, knownHcoSV)
}

func GetObjectsToBeRemoved() []ObjectToBeRemoved {
	return hcoUpgradeChanges.ObjectsToBeRemoved
}

var getUpgradeChangesFileLocation = func() string {
	return upgradeChangesFileLocation
}

func readUpgradePatchesFromFile(logger logr.Logger) error {
	hcoUpgradeChanges = UpgradePatches{}
	fileLocation := getUpgradeChangesFileLocation()

	file, err := os.Open(fileLocation)
	if err != nil {
		logger.Error(err, "Can't open the upgradeChanges yaml file", "file name", fileLocation)
		return err
	}

	jDec := json.NewDecoder(file)
	err = jDec.Decode(&hcoUpgradeChanges)

	return err
}

var (
	once    = &sync.Once{}
	onceErr error
)

func Init(logger logr.Logger) error {
	once.Do(func() {
		onceErr = readUpgradePatchesFromFile(logger)
	})

	if onceErr != nil {
		return onceErr
	}

	var err error
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
		_, err = p.JSONPatch.ApplyWithOptions(hyperConvergedCRDefault, p.JSONPatchApplyOptions)
	} else {
		_, err = p.JSONPatch.Apply(hyperConvergedCRDefault)
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
