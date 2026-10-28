package tlssecprofile

import internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/tlssecprofile"

var (
	GetCipherSuitesAndMinTLSVersion               = internal.GetCipherSuitesAndMinTLSVersion
	GetCipherSuitesAndMinTLSVersionInGolangFormat = internal.GetCipherSuitesAndMinTLSVersionInGolangFormat
	GetTLSSecurityProfile                         = internal.GetTLSSecurityProfile
	GetGroups                                     = internal.GetGroups
	GetFIPSCompliantGroups                        = internal.GetFIPSCompliantGroups
	GetGroupsInGolangFormat                       = internal.GetGroupsInGolangFormat
	GetGroupsInStringFormat                       = internal.GetGroupsInStringFormat
	Refresh                                       = internal.Refresh
	SetHyperConvergedTLSSecurityProfile           = internal.SetHyperConvergedTLSSecurityProfile
	MutateTLSConfig                               = internal.MutateTLSConfig(false)
	MutateTLSConfigCertBasedAuth                  = internal.MutateTLSConfig(true)
	RefreshClientCA                               = internal.RefreshClientCA
)
