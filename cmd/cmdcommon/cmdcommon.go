package cmdcommon

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"slices"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// list of namespace allowed for HCO installations (for tests)
const (
	operatorTestNamespace       = "test-operators"
	operatorHubNamespace        = "operators"
	communityHubNamespace       = "community-kubevirt-hyperconverged"
	communityHubTargetNamespace = "community-kubevirt-hyperconverged-target"
)

type HcCmdHelper struct {
	Logger     logr.Logger
	runInLocal bool
	Name       string
}

func NewHelper(logger logr.Logger, name string) *HcCmdHelper {
	return &HcCmdHelper{
		Logger:     logger,
		Name:       name,
		runInLocal: hcoutil.IsRunModeLocal(),
	}
}

// InitiateCommand adds flags registered by imported packages (e.g. glog and
// controller-runtime)
func (h HcCmdHelper) InitiateCommand() {
	zapFlagSet := flag.NewFlagSet("zap", flag.ExitOnError)

	updateFlagSet(flag.CommandLine, zapFlagSet)
	pflag.Parse()

	zapLogger := getZapLogger(zapFlagSet)
	logf.SetLogger(zapLogger)

	h.printVersion()

	h.checkNameSpace()
}

const pprofAddrEnvVar = "HCO_PPROF_ADDR"

// Registers a pprof server for cpu and memory profiling the running operator.
func (h HcCmdHelper) RegisterPPROFServer(mgr manager.Manager) error {
	pprofAddr := os.Getenv(pprofAddrEnvVar)
	if len(pprofAddr) == 0 {
		return nil
	}

	h.Logger.Info("Registering pprof server.")

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s := &http.Server{Addr: pprofAddr, Handler: mux}
	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		errCh := make(chan error)
		defer func() {
			// drain errCh for GC
			<-errCh
		}()

		go func() {
			// start http Server
			defer close(errCh)
			errCh <- s.ListenAndServe()
		}()

		select {
		case err := <-errCh:
			return err
		case <-ctx.Done():
			s.Close()
			return nil
		}
	}))
}

func (h HcCmdHelper) ExitOnError(err error, message string, keysAndValues ...interface{}) {
	if err != nil {
		h.Logger.Error(err, message, keysAndValues...)
		os.Exit(1)
	}
}

func (h HcCmdHelper) AddToScheme(scheme *apiruntime.Scheme, addToSchemeFuncs []func(*apiruntime.Scheme) error) {
	for _, f := range addToSchemeFuncs {
		err := f(scheme)
		h.ExitOnError(err, "Failed to add to scheme")
	}
}

func (h HcCmdHelper) printVersion() {
	h.Logger.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	h.Logger.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func (h HcCmdHelper) checkNameSpace() {
	// Get the namespace that we should be deployed in.
	requiredNS, err := getOperatorNamespaceFromEnv()
	h.ExitOnError(err, "Failed to get namespace from the environment")

	// Get the namespace we are currently deployed in.
	var actualNS string
	if !h.runInLocal {
		var err error
		actualNS, err = hcoutil.GetOperatorNamespace(h.Logger)
		h.ExitOnError(err, "Failed to get namespace")
	} else {
		h.Logger.Info("running locally")
		actualNS = requiredNS
	}

	// Allowing the operator to be deployed in OperatorTestNamespace, in addition to OPERATOR_NAMESPACE env var,
	// to unblock its publish in OperatorHub.io
	nsAllowList := []string{
		requiredNS,
		operatorTestNamespace,
		operatorHubNamespace,
		communityHubNamespace,
		communityHubTargetNamespace,
	}
	if !slices.Contains(nsAllowList, actualNS) {
		err := fmt.Errorf("%s is running in different namespace than expected", h.Name)
		msg := fmt.Sprintf("Please re-deploy this %s into %v namespace", h.Name, requiredNS)
		h.ExitOnError(err, msg, "Expected.Namespace", requiredNS, "Deployed.Namespace", actualNS)
	}
}

func getZapLogger(zapFlagSet *flag.FlagSet) logr.Logger {
	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	zapOpts := &zap.Options{}
	zapOpts.BindFlags(zapFlagSet)
	return zap.New(zap.UseFlagOptions(zapOpts))
}

func updateFlagSet(flags ...*flag.FlagSet) {
	for _, f := range flags {
		pflag.CommandLine.AddGoFlagSet(f)
	}
}

func getOperatorNamespaceFromEnv() (string, error) {
	namespace := os.Getenv(hcoutil.OperatorNamespaceEnv)
	if len(namespace) == 0 {
		return "", fmt.Errorf("%s unset or empty in environment", hcoutil.OperatorNamespaceEnv)
	}

	return namespace, nil
}
