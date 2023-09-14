package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/hcocli/deploycr"
	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/hcocli/install"
)

//go:embed deploy/crds/* deploy/*.yaml
var deploy embed.FS

func main() {
	cmd := &cobra.Command{
		Use:              "hcocli",
		SilenceUsage:     true,
		TraverseChildren: true,

		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}

	timeout := time.Minute * 20
	cmd.PersistentFlags().DurationVar(&timeout, "timeout", timeout, "timeout")
	cmd.PersistentFlags().String("kubeconfig", "", "path of the kubeconfig file")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd.AddCommand(install.NewCommand(deploy))
	cmd.AddCommand(deploycr.NewCommand(deploy))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		<-sigs
		fmt.Fprintln(os.Stderr, "\nAborted...")
		cancel()
	}()

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)

		os.Exit(1)
	}
}
