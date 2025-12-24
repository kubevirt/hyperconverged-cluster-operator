package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"

	assets "github.com/kubevirt/hyperconverged-cluster-operator/assets"
)

// no secret management needed; Perses handles console proxy secrets when client.tls is set

// persesAssetsDir is the directory (inside the repo image) containing Perses CR manifests (yaml).
// Each YAML should define a single resource: PersesDashboard or PersesDatasource.
const persesAssetsDir = "."

// ReconcilePersesResources applies all Perses resources shipped with the operator under assets/perses.
// It uses server-side apply to create/update resources idempotently without relying on cache reads.
func (r *PersesReconciler) ReconcilePersesResources(ctx context.Context) error {
	// Ensure Perses CRDs are present before attempting to apply resources.
	// Without CRDs, server-side apply will fail on every reconcile loop.
	if !hcoutil.IsPersesAvailable(ctx, r.Client) {
		return nil
	}

	// Load and cache dashboards once
	r.assetsOnce.Do(func() {
		ca, err := readPersesAssets(assets.GetPersesDashboardsFS())
		if err != nil {
			panic(fmt.Sprintf("failed to read/parse embedded Perses dashboards: %v", err))
		}
		r.cachedDashboards = ca
	})

	// Load and cache datasources once
	r.datasourcesOnce.Do(func() {
		ca, err := readPersesAssets(assets.GetPersesDatasourcesFS())
		if err != nil {
			panic(fmt.Sprintf("failed to read/parse embedded Perses datasources: %v", err))
		}
		r.cachedDatasources = ca
	})

	// Apply cached resources each reconcile
	if err := r.applyObjectList(ctx, r.cachedDatasources); err != nil {
		return err
	}
	if err := r.applyObjectList(ctx, r.cachedDashboards); err != nil {
		return err
	}
	return nil
}

// applyObjectList applies a list of generic objects (maps) after enforcing the operator namespace.
func (r *PersesReconciler) applyObjectList(ctx context.Context, objects []map[string]any) error {
	for _, raw := range objects {
		objMap := deepCopyMap(raw)
		enforceNamespace(objMap, r.namespace)

		u, err := buildUnstructured(objMap)
		if err != nil {
			return err
		}
		if err := r.applyIfChanged(ctx, u); err != nil {
			return err
		}
	}
	return nil
}

// applyIfChanged fetches the current object and applies via SSA only if the spec differs.
func (r *PersesReconciler) applyIfChanged(ctx context.Context, desired *unstructured.Unstructured) error {
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(desired.GroupVersionKind())
	err := r.Get(ctx, types.NamespacedName{Namespace: desired.GetNamespace(), Name: desired.GetName()}, current)
	if err == nil && reflect.DeepEqual(current.Object["spec"], desired.Object["spec"]) {
		return nil
	}
	if err := r.Patch(ctx, desired, client.Apply, client.FieldOwner("hyperconverged-operator"), client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply %s/%s: %w", desired.GetNamespace(), desired.GetName(), err)
	}
	return nil
}

// enforceNamespace ensures metadata.namespace is set on the object map.
func enforceNamespace(objMap map[string]any, namespace string) {
	metadata, _ := objMap["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
		objMap["metadata"] = metadata
	}
	metadata["namespace"] = namespace
}

// buildUnstructured builds an Unstructured object from a generic map and validates apiVersion/kind.
func buildUnstructured(objMap map[string]any) (*unstructured.Unstructured, error) {
	apiVersion, _ := objMap["apiVersion"].(string)
	kind, _ := objMap["kind"].(string)
	if apiVersion == "" || kind == "" {
		return nil, fmt.Errorf("missing apiVersion/kind in object")
	}
	u := &unstructured.Unstructured{Object: objMap}
	u.SetGroupVersionKind(schema.FromAPIVersionAndKind(apiVersion, kind))
	return u, nil
}

// readPersesAssets walks the assets dir and returns parsed objects as generic maps.
func readPersesAssets(d fs.FS) ([]map[string]any, error) {
	var out []map[string]any
	err := fs.WalkDir(d, persesAssetsDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if ext := filepath.Ext(entry.Name()); ext != ".yml" && ext != ".yaml" {
			return nil
		}
		content, err := fs.ReadFile(d, path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}
		jsonBytes, err := yaml.YAMLToJSON(content)
		if err != nil {
			return fmt.Errorf("failed to convert yaml to json for %s: %w", path, err)
		}
		var objMap map[string]any
		if err := json.Unmarshal(jsonBytes, &objMap); err != nil {
			return fmt.Errorf("failed to unmarshal json for %s: %w", path, err)
		}
		out = append(out, objMap)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func deepCopyMap(src map[string]any) map[string]any {
	b, _ := json.Marshal(src)
	var dst map[string]any
	_ = json.Unmarshal(b, &dst)
	return dst
}
