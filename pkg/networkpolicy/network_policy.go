package networkpolicy

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type Builder struct {
	name      string
	component hcoutil.AppComponent
	options   []Option
}

func NewBuilder(name string, component hcoutil.AppComponent, options ...Option) *Builder {
	return &Builder{name: name, component: component, options: options}
}

func (b *Builder) WithOption(option Option) {
	b.options = append(b.options, option)
}

func (b *Builder) Build() *networkingv1.NetworkPolicy {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    hcoutil.GetLabels(hcoutil.HyperConvergedName, b.component),
		},
	}

	for _, option := range b.options {
		option(np)
	}

	return np
}
