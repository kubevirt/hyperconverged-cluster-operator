package tlssecprofile

import (
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	validCipherNames sets.Set[string]
	validGroupNames  sets.Set[string]
)

func init() {
	validCipherNames = sets.New[string]()
	validGroupNames = sets.New[string]()

	for _, profile := range openshiftconfigv1.TLSProfiles {
		validCipherNames.Insert(profile.Ciphers...)
		for _, g := range profile.Groups {
			validGroupNames.Insert(string(g))
		}
	}
}

func isValidCipherName(cipher string) bool {
	return validCipherNames.Has(cipher)
}

func isValidGroupName(group openshiftconfigv1.TLSGroup) bool {
	return validGroupNames.Has(string(group))
}
