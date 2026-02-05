package tlssecprofile

import (
	"cmp"
	"context"
	"crypto/tls"

	"github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	APIServerCRName = "cluster"
)

// GetTLSSecurityProfile returns the most prioritized profile - first from HCO CR, then from the APIServer, the default
// It assumes fromHC is a result of ValidateAPIServerTLSSecurityProfile(hco.spec.TLSSecurityProfile)
func GetTLSSecurityProfile(fromHC *openshiftconfigv1.TLSSecurityProfile) *openshiftconfigv1.TLSSecurityProfile {
	profile := cmp.Or(
		fromHC,
		getAPIServerProfile(),
		defaultTLSSecurityProfile(),
	)

	// this should never happen, because it is validated in the webhook for HCO, and set in Refresh() for the APIServer CR
	if profile.Type == openshiftconfigv1.TLSProfileCustomType && profile.Custom == nil {
		logf.Log.WithName("tls-security-profile-logger").Info(`WARNING: The provided TLS Security Profile is  wrong: the type is "Custom", but the custom field is not set`)
		profile.Custom = &openshiftconfigv1.CustomTLSProfile{
			TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
				Ciphers:       openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].Ciphers,
				MinTLSVersion: openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
			},
		}
	}

	return profile
}

func GetCipherSuitesAndMinTLSVersion(fromHC *openshiftconfigv1.TLSSecurityProfile) ([]string, openshiftconfigv1.TLSProtocolVersion) {
	profile := GetTLSSecurityProfile(fromHC)

	if profile.Type == openshiftconfigv1.TLSProfileCustomType {
		return profile.Custom.Ciphers, profile.Custom.MinTLSVersion
	}

	return openshiftconfigv1.TLSProfiles[profile.Type].Ciphers, openshiftconfigv1.TLSProfiles[profile.Type].MinTLSVersion
}

func GetCipherSuitesAndMinTLSVersionInGolangFormat(fromHC *openshiftconfigv1.TLSSecurityProfile) (ciphers []uint16, minTLSVersion uint16) {
	cipherNames, minTypedTLSVersion := GetCipherSuitesAndMinTLSVersion(fromHC)

	goCiphers := crypto.CipherSuitesOrDie(crypto.OpenSSLToIANACipherSuites(cipherNames))
	goMinTLSVersion := crypto.TLSVersionOrDie(string(minTypedTLSVersion))

	return goCiphers, goMinTLSVersion
}

func SetHyperConvergedTLSSecurityProfile(fromHC *openshiftconfigv1.TLSSecurityProfile) {
	setHyperConvergedProfile(fromHC)
}

func MutateTLSConfig(cfg *tls.Config) {
	// This callback executes on each client call returning a new config to be used
	// please be aware that the APIServer is using http keepalive so this is going to
	// be executed only after a while for fresh connections and not on existing ones
	cfg.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
		cfg.CipherSuites, cfg.MinVersion = GetCipherSuitesAndMinTLSVersionInGolangFormat(getHyperConvergedProfile())

		return cfg, nil
	}
}

func Refresh(ctx context.Context, cl client.Client) (modified bool, err error) {
	apiServer := &openshiftconfigv1.APIServer{}

	logger := logr.FromContextOrDiscard(ctx)

	key := client.ObjectKey{Name: APIServerCRName}
	err = cl.Get(ctx, key, apiServer)
	if err != nil {
		return false, err
	}

	return setAPIServerProfile(validateAPIServerTLSSecurityProfile(apiServer.Spec.TLSSecurityProfile, logger)), nil
}

func defaultTLSSecurityProfile() *openshiftconfigv1.TLSSecurityProfile {
	return &openshiftconfigv1.TLSSecurityProfile{
		Type:         openshiftconfigv1.TLSProfileIntermediateType,
		Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
	}
}

func validateAPIServerTLSSecurityProfile(apiServerTLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile, logger logr.Logger) *openshiftconfigv1.TLSSecurityProfile {
	if apiServerTLSSecurityProfile == nil || apiServerTLSSecurityProfile.Type != openshiftconfigv1.TLSProfileCustomType {
		return apiServerTLSSecurityProfile
	}

	validatedAPIServerTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
		Type: openshiftconfigv1.TLSProfileCustomType,
		Custom: &openshiftconfigv1.CustomTLSProfile{
			TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
				Ciphers:       openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].Ciphers,
				MinTLSVersion: openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
			},
		},
	}

	if apiServerTLSSecurityProfile.Custom == nil {
		logger.Error(nil, "invalid custom configuration for TLSSecurityProfile on the APIServer CR, taking default values", "apiServerTLSSecurityProfile", apiServerTLSSecurityProfile)
	} else {
		validatedAPIServerTLSSecurityProfile.Custom.MinTLSVersion = apiServerTLSSecurityProfile.Custom.MinTLSVersion
		validatedAPIServerTLSSecurityProfile.Custom.Ciphers = nil
		for _, cipher := range apiServerTLSSecurityProfile.Custom.Ciphers {
			if isValidCipherName(cipher) {
				validatedAPIServerTLSSecurityProfile.Custom.Ciphers = append(validatedAPIServerTLSSecurityProfile.Custom.Ciphers, cipher)
			} else {
				logger.Error(nil, "invalid cipher name on the APIServer CR, ignoring it", "cipher", cipher)
			}
		}
	}

	return validatedAPIServerTLSSecurityProfile
}
