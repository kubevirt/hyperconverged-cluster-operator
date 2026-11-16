package virtimport

import (
	"bufio"
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"sync"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	ManagerContainerName = "manager"
	FetcherImageEnvName  = "FETCHER_IMAGE"
)

// bundleYAML is the install-hco.yaml release asset of kubevirt/virt-import, synced by
// hack/update-virt-import-bundle.sh. Downstream builds may replace it before building.
//
//go:embed data/install-hco.yaml
var bundleYAML []byte

// Bundle holds the objects of the virt-import bundle, sorted by kind.
type Bundle struct {
	CRDs                              []*apiextensionsv1.CustomResourceDefinition
	ServiceAccounts                   []*corev1.ServiceAccount
	ClusterRoles                      []*rbacv1.ClusterRole
	ClusterRoleBindings               []*rbacv1.ClusterRoleBinding
	Roles                             []*rbacv1.Role
	RoleBindings                      []*rbacv1.RoleBinding
	Deployment                        *appsv1.Deployment
	ValidatingAdmissionPolicies       []*admissionregistrationv1.ValidatingAdmissionPolicy
	ValidatingAdmissionPolicyBindings []*admissionregistrationv1.ValidatingAdmissionPolicyBinding
}

var parseEmbedded = sync.OnceValues(func() (*Bundle, error) {
	return Parse(bundleYAML)
})

func newDecoder() runtime.Decoder {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	return serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

// GetBundle returns a deep copy of the embedded bundle. The bundle is parsed once.
func GetBundle() (*Bundle, error) {
	b, err := parseEmbedded()
	if err != nil {
		return nil, err
	}
	return b.DeepCopy(), nil
}

// Parse decodes a multi-document YAML bundle, sorts its objects by kind and checks the contract
// with HCO.
func Parse(data []byte) (*Bundle, error) {
	b := &Bundle{}
	decoder := newDecoder()
	reader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read the virt-import bundle: %w", err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj, _, err := decoder.Decode(doc, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to decode an object of the virt-import bundle: %w", err)
		}

		if err = b.add(obj); err != nil {
			return nil, err
		}
	}

	if err := b.validate(); err != nil {
		return nil, err
	}

	if err := normalizeDeployment(b.Deployment); err != nil {
		return nil, fmt.Errorf("failed to normalize the virt-import Deployment: %w", err)
	}

	return b, nil
}

func (b *Bundle) add(obj runtime.Object) error {
	switch o := obj.(type) {
	case *apiextensionsv1.CustomResourceDefinition:
		b.CRDs = append(b.CRDs, o)
	case *corev1.ServiceAccount:
		b.ServiceAccounts = append(b.ServiceAccounts, o)
	case *rbacv1.ClusterRole:
		b.ClusterRoles = append(b.ClusterRoles, o)
	case *rbacv1.ClusterRoleBinding:
		b.ClusterRoleBindings = append(b.ClusterRoleBindings, o)
	case *rbacv1.Role:
		b.Roles = append(b.Roles, o)
	case *rbacv1.RoleBinding:
		b.RoleBindings = append(b.RoleBindings, o)
	case *appsv1.Deployment:
		if b.Deployment != nil {
			return errors.New("the virt-import bundle must contain exactly one Deployment")
		}
		b.Deployment = o
	case *admissionregistrationv1.ValidatingAdmissionPolicy:
		b.ValidatingAdmissionPolicies = append(b.ValidatingAdmissionPolicies, o)
	case *admissionregistrationv1.ValidatingAdmissionPolicyBinding:
		b.ValidatingAdmissionPolicyBindings = append(b.ValidatingAdmissionPolicyBindings, o)
	default:
		return fmt.Errorf("unsupported kind %s in the virt-import bundle", obj.GetObjectKind().GroupVersionKind().Kind)
	}
	return nil
}

func (b *Bundle) validate() error {
	if len(b.CRDs) == 0 {
		return errors.New("the virt-import bundle must contain at least one CustomResourceDefinition")
	}
	if b.Deployment == nil {
		return errors.New("the virt-import bundle must contain exactly one Deployment")
	}

	containers := b.Deployment.Spec.Template.Spec.Containers
	if len(containers) != 1 || containers[0].Name != ManagerContainerName {
		return fmt.Errorf("the virt-import Deployment must have exactly one container named %q", ManagerContainerName)
	}

	for _, env := range containers[0].Env {
		if env.Name == FetcherImageEnvName {
			return nil
		}
	}
	return fmt.Errorf("the virt-import %s container must have the %s env var", ManagerContainerName, FetcherImageEnvName)
}

// DeepCopy returns a deep copy of all objects of the bundle.
func (b *Bundle) DeepCopy() *Bundle {
	out := &Bundle{
		CRDs:                              deepCopySlice(b.CRDs),
		ServiceAccounts:                   deepCopySlice(b.ServiceAccounts),
		ClusterRoles:                      deepCopySlice(b.ClusterRoles),
		ClusterRoleBindings:               deepCopySlice(b.ClusterRoleBindings),
		Roles:                             deepCopySlice(b.Roles),
		RoleBindings:                      deepCopySlice(b.RoleBindings),
		ValidatingAdmissionPolicies:       deepCopySlice(b.ValidatingAdmissionPolicies),
		ValidatingAdmissionPolicyBindings: deepCopySlice(b.ValidatingAdmissionPolicyBindings),
	}
	if b.Deployment != nil {
		out.Deployment = b.Deployment.DeepCopy()
	}
	return out
}

func deepCopySlice[T interface{ DeepCopy() T }](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	for i, o := range in {
		out[i] = o.DeepCopy()
	}
	return out
}
