package v1

import "sigs.k8s.io/controller-runtime/pkg/conversion"

var _ conversion.Hub = &HyperConverged{}

// Hub implements the conversion.Hub interface, for automatic creation of a conversion webhook.
func (*HyperConverged) Hub() { /* no implementation. Just to satisfy the Hub interface */ }
