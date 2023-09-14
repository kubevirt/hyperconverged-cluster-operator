package deploycr

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/hcocli/client"
	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/hcocli/consts"
)

func NewCommand(deploy embed.FS) *cobra.Command {
	dep := deployer{
		deploy: deploy,
	}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the HyperConverged custom resource",
		RunE:  dep.run,
	}

	return cmd
}

type deployer struct {
	deploy embed.FS
}

func (dep deployer) run(cmd *cobra.Command, _ []string) error {
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

	hc := &v1beta1.HyperConverged{}

	bytes, err := dep.deploy.ReadFile("deploy/hco.cr.yaml")
	if err != nil {
		return fmt.Errorf("can't open the HyperConverged yaml file: %w", err)
	}

	err = yaml.Unmarshal(bytes, hc)
	if err != nil {
		return fmt.Errorf("failed to parse the HyperConverged yaml file: %w", err)
	}

	hc.SetNamespace(consts.HCONamespace)
	cmd.Printf("applying the HyperConverged custom resource; namespace %s, name: %s...\n", consts.HCONamespace, hc.Name)
	err = cl.Patch(cmd.Context(), hc, crclient.RawPatch(types.ApplyPatchType, bytes), &crclient.PatchOptions{FieldManager: cmd.Parent().Name()})
	if err != nil {
		return fmt.Errorf("failed to apply HyperConverged: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	cmd.Println("waiting for HyperConverged to be ready...")
	key := crclient.ObjectKeyFromObject(hc)
	err = wait.PollUntilContextCancel(timeoutCtx, time.Second, true, func(ctx context.Context) (bool, error) {
		err := cl.Get(ctx, key, hc)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if meta.IsStatusConditionTrue(hc.Status.Conditions, v1beta1.ConditionAvailable) {
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		return fmt.Errorf("HyperConverged is not ready yet: %w", err)
	}

	return nil
}
