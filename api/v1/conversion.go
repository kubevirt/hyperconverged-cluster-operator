package v1

import "sigs.k8s.io/controller-runtime/pkg/conversion"

var _ conversion.Hub = &HyperConverged{}

func (*HyperConverged) Hub() { /* no implementation. Just to satisfy the Hub interface */ }
