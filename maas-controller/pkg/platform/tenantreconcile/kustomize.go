package tenantreconcile

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	maasv1alpha1 "github.com/opendatahub-io/models-as-a-service/maas-controller/api/maas/v1alpha1"
)

// overlayDefaultNamespace is the namespace hardcoded in the overlay's
// kustomization.yaml (namespace: opendatahub). postBuildTransform remaps
// it to the actual appNamespace from the Tenant CR.
const overlayDefaultNamespace = "opendatahub"

// RenderKustomize runs kustomize build for the ODH maas-api overlay and
// applies ODH-equivalent namespace remapping and component labels.
func RenderKustomize(manifestDir, appNamespace string) ([]unstructured.Unstructured, error) {
	kustomizationPath := manifestDir
	if !fileExists(filepath.Join(manifestDir, "kustomization.yaml")) {
		kustomizationPath = filepath.Join(manifestDir, "default")
	}

	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	fs := filesys.MakeFsOnDisk()
	resMap, err := k.Run(fs, kustomizationPath)
	if err != nil {
		return nil, fmt.Errorf("kustomize build %q: %w", kustomizationPath, err)
	}

	if err := postBuildTransform(resMap, appNamespace); err != nil {
		return nil, fmt.Errorf("post-build transform: %w", err)
	}

	rendered := resMap.Resources()
	out := make([]unstructured.Unstructured, 0, len(rendered))
	for i := range rendered {
		m, err := rendered[i].Map()
		if err != nil {
			return nil, fmt.Errorf("resource map: %w", err)
		}
		normalizeJSONTypes(m)
		out = append(out, unstructured.Unstructured{Object: m})
	}
	return out, nil
}

// postBuildTransform remaps the overlay's hardcoded default namespace to the
// actual appNamespace and merges ODH component labels into metadata. Unlike the
// blanket kustomize NamespaceTransformerPlugin + LabelTransformerPlugin, this:
//   - Leaves cluster-scoped resources (no namespace) untouched
//   - Preserves cross-namespace resources placed in a non-default namespace by
//     kustomize replacements (e.g., payload-processing in the gateway namespace)
//   - Preserves ClusterRoleBinding/RoleBinding subjects with non-default namespaces
//   - Merges labels into metadata only (not into Deployment selectors, which are
//     already correct from each base's own kustomization)
func postBuildTransform(resMap resmap.ResMap, appNamespace string) error {
	componentLabels := map[string]string{
		LabelODHAppPrefix + "/" + ComponentName: "true",
		LabelK8sPartOf:                          "models-as-a-service",
	}

	for _, res := range resMap.Resources() {
		// --- namespace remapping ---
		if appNamespace != "" {
			ns := res.GetNamespace()
			if ns == overlayDefaultNamespace {
				res.SetNamespace(appNamespace)
			}
		}

		m, err := res.Map()
		if err != nil {
			continue
		}

		// Remap CRB/RB subject namespaces
		if appNamespace != "" {
			kind := res.GetKind()
			if kind == "ClusterRoleBinding" || kind == "RoleBinding" {
				if subjects, ok := m["subjects"].([]interface{}); ok {
					for _, s := range subjects {
						if subj, ok := s.(map[string]interface{}); ok {
							if sns, ok := subj["namespace"].(string); ok && sns == overlayDefaultNamespace {
								subj["namespace"] = appNamespace
							}
						}
					}
				}
			}
		}

		// --- ODH component labels (metadata only) ---
		labels, _, _ := unstructured.NestedStringMap(m, "metadata", "labels")
		if labels == nil {
			labels = make(map[string]string)
		}
		for k, v := range componentLabels {
			labels[k] = v
		}
		if err := unstructured.SetNestedStringMap(m, labels, "metadata", "labels"); err != nil {
			return fmt.Errorf("set labels on %s %s: %w", res.GetKind(), res.GetName(), err)
		}
	}
	return nil
}

// normalizeJSONTypes converts Go int values to int64 in an unstructured map.
// Kustomize's resMap.Map() returns int for YAML integers, but
// k8s.io/apimachinery DeepCopyJSONValue only handles int64/float64.
func normalizeJSONTypes(obj map[string]any) {
	for k, v := range obj {
		obj[k] = normalizeValue(v)
	}
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case int:
		return int64(val)
	case map[string]any:
		normalizeJSONTypes(val)
		return val
	case []any:
		for i, item := range val {
			val[i] = normalizeValue(item)
		}
		return val
	default:
		return v
	}
}

func fileExists(p string) bool {
	fs := filesys.MakeFsOnDisk()
	return fs.Exists(p)
}

// DefaultManifestPath returns MAAS_PLATFORM_MANIFESTS or a dev default relative to cwd (models-as-a-service repo layout).
func DefaultManifestPath() string {
	if v := os.Getenv("MAAS_PLATFORM_MANIFESTS"); v != "" {
		return v
	}
	return "../maas-api/deploy/overlays/odh"
}

// EnsureTenantGatewayDefaults applies the same default gateway ref as ODH when unset.
func EnsureTenantGatewayDefaults(t *maasv1alpha1.Tenant) {
	if t.Spec.GatewayRef.Namespace == "" && t.Spec.GatewayRef.Name == "" {
		t.Spec.GatewayRef.Namespace = DefaultGatewayNamespace
		t.Spec.GatewayRef.Name = DefaultGatewayName
	}
}
