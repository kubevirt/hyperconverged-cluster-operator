package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	log "github.com/go-logr/logr"
	imagev1 "github.com/openshift/api/image/v1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// TODO: this is very similar to the quickstart file; on golang 1.18, check if it possible to use type parameters instead.

// ImageStream resources are a short user guids
const (
	ImageStreamManifestLocationVarName = "IMAGE_STREAM_FILES_LOCATION"
	imageStreamDefaultManifestLocation = "./imageStreams"
)

var (
	imageStreamNames []string
)

func GetImageStreamNames() []string {
	return imageStreamNames
}

type imageStreamOperand struct {
	operand *operands.GenericOperand
	hooks   *isHooks
}

func (iso imageStreamOperand) Ensure(req *common.HcoRequest) *operands.EnsureResult {
	// if the EnableCommonBootImageImport field is set, make sure the imageStream is in place and up-to-date
	if req.Instance.Spec.EnableCommonBootImageImport != nil && *req.Instance.Spec.EnableCommonBootImageImport {
		if result := iso.checkCustomNamespace(req); result != nil {
			return result
		}

		return iso.operand.Ensure(req)
	}

	// if the FG is not set, make sure the imageStream is not exist
	cr := iso.hooks.GetEmptyCr()
	res := operands.NewEnsureResult(cr)
	res.SetName(cr.GetName())
	deleted, err := util.EnsureDeleted(req.Ctx, iso.operand.Client, cr, req.Instance.Name, req.Logger, false, false, true)
	if err != nil {
		return res.Error(err)
	}

	if deleted {
		res.SetDeleted()
		objectRef, err := reference.GetReference(iso.operand.Scheme, cr)
		if err != nil {
			return res.Error(err)
		}

		if err = objectreferencesv1.RemoveObjectReference(&req.Instance.Status.RelatedObjects, *objectRef); err != nil {
			return res.Error(err)
		}
		req.StatusDirty = true
	}

	return res.SetUpgradeDone(req.ComponentUpgradeInProgress)
}

func (iso imageStreamOperand) checkCustomNamespace(req *common.HcoRequest) *operands.EnsureResult {
	if ns := req.Instance.Spec.CommonBootImageNamespace; ns != nil && len(*ns) > 0 && iso.hooks.required.Namespace != *ns {
		if result := iso.deleteImageStream(req); result != nil {
			return result
		}

		iso.hooks.required.Namespace = *ns
	} else if (ns == nil || len(*ns) == 0) && iso.hooks.required.Namespace != iso.hooks.originalNS {
		if result := iso.deleteImageStream(req); result != nil {
			return result
		}

		iso.hooks.required.Namespace = iso.hooks.originalNS
	}
	return nil
}

func (iso imageStreamOperand) deleteImageStream(req *common.HcoRequest) *operands.EnsureResult {
	_, err := util.EnsureDeleted(req.Ctx, iso.operand.Client, iso.hooks.required, req.Instance.Name, req.Logger, false, true, false)
	if err != nil {
		return operands.NewEnsureResult(iso.hooks.required).Error(fmt.Errorf("failed to delete imagestream %s/%s; %w", iso.hooks.required.Namespace, iso.hooks.required.Name, err))
	}

	objectRef, err := reference.GetReference(iso.operand.Scheme, iso.hooks.required)
	if err != nil {
		return operands.NewEnsureResult(req.Instance).Error(err)
	}

	if err = objectreferencesv1.RemoveObjectReference(&req.Instance.Status.RelatedObjects, *objectRef); err != nil {
		return operands.NewEnsureResult(req.Instance).Error(err)
	}
	req.StatusDirty = true

	return nil
}

func (iso imageStreamOperand) Reset() {
	iso.operand.Reset()
}

func (iso imageStreamOperand) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return iso.operand.GetFullCr(hc)
}

func newImageStreamHandler(Client client.Client, Scheme *runtime.Scheme, required *imagev1.ImageStream, origNS string) operands.Operand {
	hooks := newIsHook(required, origNS)
	return &imageStreamOperand{
		operand: operands.NewGenericOperand(Client, Scheme, "ImageStream", hooks, false),
		hooks:   hooks,
	}
}

type isHooks struct {
	required   *imagev1.ImageStream
	originalNS string
	tags       map[string]imagev1.TagReference
}

func newIsHook(required *imagev1.ImageStream, origNS string) *isHooks {
	tags := make(map[string]imagev1.TagReference)
	for _, tag := range required.Spec.Tags {
		tags[tag.Name] = tag
	}
	return &isHooks{required: required, tags: tags, originalNS: origNS}
}

func (h isHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h isHooks) GetEmptyCr() client.Object {
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.required.Name,
			Namespace: h.required.Namespace,
		},
	}
}

func (h isHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	found, ok := exists.(*imagev1.ImageStream)

	if !ok {
		return false, false, errors.New("can't convert to ImageStream")
	}

	if label, ok := found.Labels[util.AppLabelManagedBy]; !ok || util.OperatorName != label {
		// not our imageStream. we won't reconcile it.
		return false, false, nil
	}

	if !h.compareAndUpgradeImageStream(found) {
		return false, false, nil
	}

	if req.HCOTriggered {
		req.Logger.Info("Updating existing ImageStream's Spec to new opinionated values", "name", h.required.Name)
	} else {
		req.Logger.Info("Reconciling an externally updated ImageStream's Spec to its opinionated values", "name", h.required.Name)
	}

	err := Client.Update(req.Ctx, found)
	if err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}

func (isHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (h isHooks) compareAndUpgradeImageStream(found *imagev1.ImageStream) bool {
	modified := false
	if !util.CompareLabels(h.required, found) {
		util.MergeLabels(&h.required.ObjectMeta, &found.ObjectMeta)
		modified = true
	}

	newTags := make([]imagev1.TagReference, 0)

	for _, foundTag := range found.Spec.Tags {
		reqTag, ok := h.tags[foundTag.Name]
		if !ok {
			modified = true
			continue
		}

		if compareOneTag(&foundTag, &reqTag) {
			modified = true
		}

		newTags = append(newTags, foundTag)
	}

	// find and add missing tags
	newTags, modified = h.addMissingTags(found, newTags, modified)

	if modified {
		found.Spec.Tags = newTags
	}

	return modified
}

func (h isHooks) addMissingTags(found *imagev1.ImageStream, newTags []imagev1.TagReference, modified bool) ([]imagev1.TagReference, bool) {
	for reqTagName, reqTag := range h.tags {
		tagExist := false
		for _, foundTag := range found.Spec.Tags {
			if reqTagName == foundTag.Name {
				tagExist = true
			}
		}

		if !tagExist {
			newTags = append(newTags, reqTag)
			modified = true
		}
	}
	return newTags, modified
}

func GetImageStreamHandlers(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) ([]operands.Operand, error) {
	filesLocation := util.GetManifestDirPath(ImageStreamManifestLocationVarName, imageStreamDefaultManifestLocation)

	err := util.ValidateManifestDir(filesLocation)
	if err != nil {
		logger.Error(err, "can't get manifest directory for imageStreams", "imageStream files location", filesLocation)
		return nil, errors.Unwrap(err) // if not wrapped, then it's not an error that stops processing, and it return nil
	}

	return createImageStreamHandlersFromFiles(logger, Client, Scheme, hc, filesLocation)
}

func createImageStreamHandlersFromFiles(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, filesLocation string) ([]operands.Operand, error) {
	var handlers []operands.Operand
	imageStreamNames = []string{}

	logger.Info("walking over the files in " + filesLocation + ", to find imageStream files.")

	err := filepath.Walk(filesLocation, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		logger.Info("processing imageStream file", "fileName", path, "fileInfo", info)
		is, err := processImageStreamFile(path, info, logger, hc, Client, Scheme)
		if err != nil {
			return err
		}

		if is != nil {
			handlers = append(handlers, is)
		}

		return nil
	})

	return handlers, err
}

func compareOneTag(foundTag, reqTag *imagev1.TagReference) bool {
	modified := false
	if reqTag.From.Name != foundTag.From.Name || reqTag.From.Kind != foundTag.From.Kind {
		foundTag.From = reqTag.From.DeepCopy()
		modified = true
	}

	if !reflect.DeepEqual(reqTag.ImportPolicy, foundTag.ImportPolicy) {
		foundTag.ImportPolicy = *reqTag.ImportPolicy.DeepCopy()
		modified = true
	}

	return modified
}

func processImageStreamFile(path string, info os.FileInfo, logger log.Logger, hc *hcov1beta1.HyperConverged, Client client.Client, Scheme *runtime.Scheme) (operands.Operand, error) {
	if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
		file, err := os.Open(path)
		if err != nil {
			logger.Error(err, "Can't open the ImageStream yaml file", "file name", path)
			return nil, err
		}

		is := &imagev1.ImageStream{}
		err = util.UnmarshalYamlFileToObject(file, is)
		if err != nil {
			return nil, err
		}

		origNS := is.Namespace
		if ns := hc.Spec.CommonBootImageNamespace; ns != nil && len(*ns) > 0 {
			is.Namespace = *ns
		}

		is.Labels = operands.GetLabels(hc, util.AppComponentCompute)
		imageStreamNames = append(imageStreamNames, is.Name)
		return newImageStreamHandler(Client, Scheme, is, origNS), nil
	}

	return nil, nil
}
