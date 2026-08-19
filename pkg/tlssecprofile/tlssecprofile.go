package tlssecprofile

import internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/tlssecprofile"

var (
	GetCipherSuitesAndMinTLSVersion               = internal.GetCipherSuitesAndMinTLSVersion
	GetCipherSuitesAndMinTLSVersionInGolangFormat = internal.GetCipherSuitesAndMinTLSVersionInGolangFormat
	GetTLSSecurityProfile                         = internal.GetTLSSecurityProfile
	GetGroups                                     = internal.GetGroups
	GetGroupsInGolangFormat                       = internal.GetGroupsInGolangFormat
	Refresh                                       = internal.Refresh
	SetHyperConvergedTLSSecurityProfile           = internal.SetHyperConvergedTLSSecurityProfile
	MutateTLSConfig                               = internal.MutateTLSConfig
)
