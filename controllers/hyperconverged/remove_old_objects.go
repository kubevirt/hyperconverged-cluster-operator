package hyperconverged

import (
	"slices"

	consolev1 "github.com/openshift/api/console/v1"
	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func removeOldQuickStartGuides(req *common.HcoRequest, cl client.Client, requiredQSList []string) {
	existingQSList := &consolev1.ConsoleQuickStartList{}
	req.Logger.Info("reading quickstart guides")
	err := cl.List(req.Ctx, existingQSList, client.MatchingLabels{hcoutil.AppLabelManagedBy: hcoutil.OperatorName})
	if err != nil {
		req.Logger.Error(err, "failed to read list of quickstart guides")
		return
	}

	for _, qs := range existingQSList.Items {
		if !slices.Contains(requiredQSList, qs.Name) {
			req.Logger.Info("deleting ConsoleQuickStart", "name", qs.Name)
			if _, err = hcoutil.EnsureDeleted(req.Ctx, cl, &qs, req.Instance.Name, req.Logger, false, false, true); err != nil {
				req.Logger.Error(err, "failed to delete ConsoleQuickStart", "name", qs.Name)
			}
		}
	}

	removeRelatedObjects(req, requiredQSList, "ConsoleQuickStart")
}

// removeRelatedObjects removes old reference from the related object list
// can't use the removeRelatedObject function because the status not get updated during each reconcile loop,
// but the old object already removed (above) so you loos track of it. That why we must re-check all the names
func removeRelatedObjects(req *common.HcoRequest, requiredNames []string, typeName string) {
	refs := make([]corev1.ObjectReference, 0, len(req.Instance.Status.RelatedObjects))
	foundOldQs := false

	for _, obj := range req.Instance.Status.RelatedObjects {
		if obj.Kind == typeName && !slices.Contains(requiredNames, obj.Name) {
			foundOldQs = true
			continue
		}
		refs = append(refs, obj)
	}

	if foundOldQs {
		req.Instance.Status.RelatedObjects = refs
		req.StatusDirty = true
	}
}

func removeOldImageStream(req *common.HcoRequest, cl client.Client, requiredISList []string) {
	existingISList := &imagev1.ImageStreamList{}
	req.Logger.Info("reading ImageStreams")
	err := cl.List(req.Ctx, existingISList, client.MatchingLabels{hcoutil.AppLabelManagedBy: hcoutil.OperatorName})
	if err != nil {
		req.Logger.Error(err, "failed to read list of ImageStreams")
		return
	}

	for _, is := range existingISList.Items {
		if !slices.Contains(requiredISList, is.Name) {
			req.Logger.Info("deleting ImageStream", "name", is.Name)
			if _, err = hcoutil.EnsureDeleted(req.Ctx, cl, &is, req.Instance.Name, req.Logger, false, false, true); err != nil {
				req.Logger.Error(err, "failed to delete ImageStream", "name", is.Name)
			}
		}
	}

	removeRelatedObjects(req, requiredISList, "ImageStream")
}
