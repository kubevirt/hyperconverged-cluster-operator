package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"text/template"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func main() {
	if err := generateNetworkPolicies(); err != nil {
		fmt.Fprintf(os.Stderr, "error generating network policies: %v\n", err)
		os.Exit(1)
	}
}

//go:embed network-policies-template.tmpl
var networkPoliciesTemplateFile string

type networkPoliciesParams struct {
	Namespace            string
	DNSNamespaceSelector string
	DNSPodSelectorLabel  string
	DNSPodSelectorVal    string
	WebhookPort          int32
	CLIDownloadsPort     int32
}

var (
	networkPoliciesTemplate = template.Must(template.New("network-policies").Parse(networkPoliciesTemplateFile))
	params                  = networkPoliciesParams{
		WebhookPort:      hcoutil.WebhookPort,
		CLIDownloadsPort: hcoutil.CliDownloadsServerPort,
	}
)

func init() {
	flag.StringVar(&params.Namespace, "namespace", "kubevirt-hyperconverged", "Namespace")
	flag.StringVar(&params.DNSNamespaceSelector, "dns-namespace-selector", "", "DNS Namespace Selector for the DNS network policy; only applied if output-mode is 'CRDs' and --dump-network-policies is set")
	flag.StringVar(&params.DNSPodSelectorLabel, "dns-pod-selector-label", "", "DNS Pod Selector label for the DNS network policy; only applied if output-mode is 'CRDs' and --dump-network-policies is set")
	flag.StringVar(&params.DNSPodSelectorVal, "dns-pod-selector-val", "", "DNS Pod Selector for label value the DNS network policy; only applied if output-mode is 'CRDs' and --dump-network-policies is set")

	flag.Parse()

	if params.DNSNamespaceSelector == "" {
		fmt.Fprintln(os.Stderr, "error: --dns-namespace-selector is required")
		flag.Usage()
		os.Exit(1)
	}

	if params.DNSPodSelectorLabel == "" {
		fmt.Fprintln(os.Stderr, "error: --dns-pod-selector-label is required")
		flag.Usage()
		os.Exit(1)
	}

	if params.DNSPodSelectorVal == "" {
		fmt.Fprintln(os.Stderr, "error: --dns-pod-selector-val is required")
		flag.Usage()
		os.Exit(1)
	}
}

func generateNetworkPolicies() error {
	return networkPoliciesTemplate.Execute(os.Stdout, params)
}
