package install

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	openshiftcv1 "github.com/openshift/api/config/v1"
	"github.com/spf13/cobra"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/hcocli/client"
	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/hcocli/consts"
)

var skipLabels = sets.New[string]()

func NewCommand(deploy embed.FS) *cobra.Command {
	ins := installer{deploy: deploy}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install all the dependencies, the required resources and the kubevirt hyperconverged cluster operator",
		RunE:  ins.run,
	}

	return cmd
}

type installer struct {
	deploy embed.FS
}

func (ins installer) run(cmd *cobra.Command, _ []string) error {
	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	cl, err := client.GetClient(kubeconfig)
	if err != nil {
		return err
	}

	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	cmd.SetContext(ctx)

	openshiftCluster, err := isOpenshift(ctx, cl)
	if err != nil {
		return err
	}

	if !openshiftCluster {
		skipLabels.Insert("ssp-operator", "hyperconverged-cluster-cli-download")
	}

	cmd.Printf("creating the %s namespace...\n", consts.HCONamespace)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: consts.HCONamespace}}
	err = cl.Create(cmd.Context(), ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil
	}

	dir, err := ins.deploy.ReadDir("deploy/crds")
	if err != nil {
		return err
	}

	for _, entry := range dir {
		if entry.Type() == 0 {
			name := path.Join("deploy/crds", entry.Name())

			err = ins.deployCRD(cmd, cl, name, skipLabels)
			if err != nil {
				continue
			}
		}
	}

	err = ins.deployFromYamlFile(cmd, cl, "deploy/cert-manager.yaml", "", skipLabels)
	if err != nil {
		return err
	}

	for _, deployment := range []string{"cert-manager", "cert-manager-webhook"} {
		err = ins.waitForDeployment(cmd, cl, "cert-manager", deployment)
		if err != nil {
			return err
		}
	}

	for _, yamlFile := range []string{
		"deploy/cluster_role.yaml",
		"deploy/service_account.yaml",
		"deploy/cluster_role_binding.yaml",
		"deploy/webhooks.yaml",
		"deploy/operator.yaml",
	} {
		err = ins.deployFromYamlFile(cmd, cl, yamlFile, consts.HCONamespace, skipLabels)
		if err != nil {
			return err
		}
	}

	err = ins.waitForDeployment(cmd, cl, "kubevirt-hyperconverged", "hyperconverged-cluster-webhook")
	if err != nil {
		return err
	}

	cmd.Println("Successfully installed the KubeVirt hyperconverged cluster operator.")
	cmd.Println("To complete the installation, run:")
	cmd.Printf("\n\t%s deploy\n", cmd.Parent().Name())

	return nil
}

func (ins installer) deployCRD(cmd *cobra.Command, cl crclient.Client, name string, skipLabels sets.Set[string]) error {
	crdBytes, err := ins.deploy.ReadFile(name)
	if err != nil {
		return fmt.Errorf("can't read file %q: %w", name, err)
	}

	crd := &extv1.CustomResourceDefinition{}
	err = yaml.Unmarshal(crdBytes, crd)
	if err != nil {
		return fmt.Errorf("can't parse yaml file %q: %w", name, err)
	}

	if nameLbl, ok := crd.Labels["name"]; ok && skipLabels.Has(nameLbl) {
		return nil
	}

	crdName := crd.Name
	cmd.Printf("applying CustomResourceDefinition %s...\n", crdName)
	err = cl.Patch(cmd.Context(), crd, crclient.RawPatch(types.ApplyPatchType, crdBytes), &crclient.PatchOptions{FieldManager: cmd.Parent().Name()})
	if err != nil {
		return fmt.Errorf("failed to apply CustomResourceDefinition %q: %w", name, err)
	}

	return nil
}

func (ins installer) deployFromYamlFile(cmd *cobra.Command, cl crclient.Client, fileName string, ns string, skipLabels sets.Set[string]) error {
	fileBytes, err := ins.deploy.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("can't read file %s: %w", fileName, err)
	}

	objectStrs := strings.Split(string(fileBytes), "---\n")
	for _, str := range objectStrs {
		var objectMap map[string]any
		err = yaml.Unmarshal([]byte(str), &objectMap)
		if err != nil {
			return fmt.Errorf("can't parse file %s; %v", fileName, err)
		} else if objectMap == nil {
			continue
		}

		obj := &unstructured.Unstructured{}
		runtime.Unstructured.SetUnstructuredContent(obj, objectMap)

		if name, ok := obj.GetLabels()["name"]; ok && skipLabels.Has(name) {
			continue
		}

		if len(ns) > 0 {
			obj.SetNamespace(ns)
		}

		cmd.Printf("applying %s %s...\n", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		err = cl.Patch(cmd.Context(), obj, crclient.RawPatch(types.ApplyPatchType, []byte(str)), &crclient.PatchOptions{FieldManager: cmd.Parent().Name()})
		if err != nil {
			return fmt.Errorf("failed to apply %v %q: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
		}
	}

	return nil
}

func (ins installer) waitForDeployment(cmd *cobra.Command, cl crclient.Client, namespace string, name string) error {
	cmd.Printf("waiting for deployment to be ready; namespace: %s, name: %s...\n", namespace, name)
	dep := &appv1.Deployment{}
	timeoutCtx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	err := wait.PollUntilContextCancel(timeoutCtx, time.Second, true, func(ctx context.Context) (bool, error) {
		err := cl.Get(ctx, crclient.ObjectKey{Name: name, Namespace: namespace}, dep)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		for _, cond := range dep.Status.Conditions {
			if cond.Type == "Available" && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil {
		return fmt.Errorf("deployment %s/%s is not ready yet: %w", namespace, name, err)
	}

	return nil
}

func isOpenshift(ctx context.Context, cl crclient.Client) (bool, error) {
	clusterVersion := &openshiftcv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
	}

	if err := cl.Get(ctx, crclient.ObjectKeyFromObject(clusterVersion), clusterVersion); err != nil {
		var gdferr *discovery.ErrGroupDiscoveryFailed
		if meta.IsNoMatchError(err) || k8serrors.IsNotFound(err) || errors.As(err, &gdferr) {
			return false, nil
		}

		return false, fmt.Errorf("failed to read cluster version: %w", err)
	}
	return true, nil
}
