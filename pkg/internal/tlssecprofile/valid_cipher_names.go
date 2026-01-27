package tlssecprofile

import (
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var validCipherNames sets.Set[string]

func init() {
	validCipherNames = sets.New[string]()

	for _, profiles := range openshiftconfigv1.TLSProfiles {
		validCipherNames.Insert(profiles.Ciphers...)
	}
}

func isValidCipherName(cipher string) bool {
	return validCipherNames.Has(cipher)
}
