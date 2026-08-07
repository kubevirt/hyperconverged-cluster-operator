package tlssecprofile

// TODO: Replace this file with github.com/openshift/library-go/pkg/crypto once
// https://github.com/openshift/library-go/pull/2347 is merged and vendored.

import (
	"crypto/tls"
	"sort"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
)

var tlsGroupToCurveIDMap = map[openshiftconfigv1.TLSGroup]tls.CurveID{
	openshiftconfigv1.TLSGroupX25519:             tls.X25519,
	openshiftconfigv1.TLSGroupSecP256r1:          tls.CurveP256,
	openshiftconfigv1.TLSGroupSecP384r1:          tls.CurveP384,
	openshiftconfigv1.TLSGroupSecP521r1:          tls.CurveP521,
	openshiftconfigv1.TLSGroupX25519MLKEM768:     tls.X25519MLKEM768,
	openshiftconfigv1.TLSGroupSecP256r1MLKEM768:  tls.SecP256r1MLKEM768,
	openshiftconfigv1.TLSGroupSecP384r1MLKEM1024: tls.SecP384r1MLKEM1024,
}

// CurveIDForTLSGroup maps an OpenShift API TLSGroup constant to Go's tls.CurveID.
// Returns (0, false) if the group is not recognized.
func CurveIDForTLSGroup(group openshiftconfigv1.TLSGroup) (tls.CurveID, bool) {
	id, ok := tlsGroupToCurveIDMap[group]
	return id, ok
}

// CurveIDsForTLSGroups converts a slice of TLSGroup values to []tls.CurveID,
// returning both the valid curve IDs and the unsupported/unrecognized groups.
func CurveIDsForTLSGroups(groups []openshiftconfigv1.TLSGroup) ([]tls.CurveID, []openshiftconfigv1.TLSGroup) {
	var curveIDs []tls.CurveID
	var unsupported []openshiftconfigv1.TLSGroup

	for _, g := range groups {
		if id, ok := CurveIDForTLSGroup(g); ok {
			curveIDs = append(curveIDs, id)
		} else {
			unsupported = append(unsupported, g)
		}
	}

	return curveIDs, unsupported
}

// ValidTLSGroups returns the sorted list of TLS group names that are mapped/known.
func ValidTLSGroups() []string {
	names := make([]string, 0, len(tlsGroupToCurveIDMap))
	for g := range tlsGroupToCurveIDMap {
		names = append(names, string(g))
	}
	sort.Strings(names)
	return names
}
