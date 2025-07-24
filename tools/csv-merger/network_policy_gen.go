package main

import (
	_ "embed"
	"flag"
	"os"
	"text/template"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	k8sDNSNamespaceSelector = "kube-system"
	k8sDNSPodSelectorLabel  = "k8s-app"
	k8sDNSPodSelectorVal    = "kube-dns"
	k8sDNSPort              = uint32(53)

	openshiftDNSNamespaceSelector = "openshift-dns"
	openshiftDNSPodSelectorLabel  = "dns.operator.openshift.io/daemonset-dns"
	openshiftDNSPodSelectorVal    = "default"
	openshiftDNSPort              = uint32(5353)
)

//go:embed network-policies-template.tmpl
var networkPoliciesTemplateFile string

type networkPoliciesParams struct {
	Namespace        string
	DNSSelectors     []DNSSelector
	WebhookPort      int32
	CLIDownloadsPort int32
}

type DNSSelector struct {
	DNSNamespaceSelector string
	DNSPodSelectorLabel  string
	DNSPodSelectorVal    string
	DNSPort              uint32
}

var (
	networkPoliciesTemplate = template.Must(template.New("network-policies").Parse(networkPoliciesTemplateFile))

	// deployK8sDNSNetworkPolicy is a flag to control whether the k8s DNS network policy should be deployed.
	// By default, the bundle image that includes the network policy yaml files is going to be deployed on openshift.
	// But it is also possible to deploy the bundle image on k8s clusters, in which case the k8s DNS network policy
	// should be deployed.
	// When building for both openshift and k8s, this flag should be set to true, in order to add also the k8s DNS
	// network policy rule to the bundle image, in addition to the openshift rule.
	deployK8sDNSNetworkPolicy = flag.Bool("deploy-k8s-dns-networkpolicy", false, "Deploy the k8s DNS network policy; only applied if output-mode is 'CSV' and --dump-network-policies is set")

	params = networkPoliciesParams{
		WebhookPort:      hcoutil.WebhookPort,
		CLIDownloadsPort: hcoutil.CliDownloadsServerPort,
		DNSSelectors: []DNSSelector{
			{
				DNSNamespaceSelector: openshiftDNSNamespaceSelector,
				DNSPodSelectorLabel:  openshiftDNSPodSelectorLabel,
				DNSPodSelectorVal:    openshiftDNSPodSelectorVal,
				DNSPort:              openshiftDNSPort,
			},
		},
	}
)

func generateNetworkPolicies() error {
	if *deployK8sDNSNetworkPolicy {
		params.DNSSelectors = append(params.DNSSelectors, DNSSelector{
			DNSNamespaceSelector: k8sDNSNamespaceSelector,
			DNSPodSelectorLabel:  k8sDNSPodSelectorLabel,
			DNSPodSelectorVal:    k8sDNSPodSelectorVal,
			DNSPort:              k8sDNSPort,
		})
	}

	return networkPoliciesTemplate.Execute(os.Stdout, params)
}
