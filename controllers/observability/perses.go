package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// persesAssetsDir is the directory (inside the repo image) containing Perses CR manifests (yaml).
// Each YAML should define a single resource: PersesDashboard or PersesDatasource.
const persesAssetsDir = "assets/dashboards/perses"

// ReconcilePersesResources applies all Perses resources shipped with the operator under assets/perses.
// It uses server-side apply to create/update resources idempotently without relying on cache reads.
func (r *Reconciler) ReconcilePersesResources(ctx context.Context) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working dir: %w", err)
	}

	d := os.DirFS(wd)
	// If the directory does not exist, nothing to do.
	if _, err := fs.Stat(d, persesAssetsDir); err != nil {
		return nil
	}

	return fs.WalkDir(d, persesAssetsDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if ext := filepath.Ext(entry.Name()); ext != ".yml" && ext != ".yaml" {
			return nil
		}

		file, err := d.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer file.Close()

		content, err := fs.ReadFile(d, path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Decode YAML → JSON → map
		jsonBytes, err := yaml.YAMLToJSON(content)
		if err != nil {
			return fmt.Errorf("failed to convert yaml to json for %s: %w", path, err)
		}

		var objMap map[string]any
		if err := json.Unmarshal(jsonBytes, &objMap); err != nil {
			return fmt.Errorf("failed to unmarshal json for %s: %w", path, err)
		}

		// Enforce namespace to operator's namespace to ensure ownership & GC
		// If the manifest specifies a namespace, it will be overridden.
		// We only support namespaced resources here.
		metadata, _ := objMap["metadata"].(map[string]any)
		if metadata == nil {
			metadata = map[string]any{}
			objMap["metadata"] = metadata
		}
		metadata["namespace"] = r.namespace

		// Build Unstructured with GVK populated
		apiVersion, _ := objMap["apiVersion"].(string)
		kind, _ := objMap["kind"].(string)
		if apiVersion == "" || kind == "" {
			return fmt.Errorf("missing apiVersion/kind in %s", path)
		}

		u := &unstructured.Unstructured{Object: objMap}
		u.SetGroupVersionKind(schema.FromAPIVersionAndKind(apiVersion, kind))

		// Apply with SSA so we don't need to read existing objects
		if err := r.Patch(ctx, u, client.Apply, client.FieldOwner("hyperconverged-operator"), client.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply %s: %w", path, err)
		}

		return nil
	})
}
