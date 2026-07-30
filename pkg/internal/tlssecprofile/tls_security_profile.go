package tlssecprofile

import (
	"cmp"
	"context"
	"crypto/tls"
	"slices"

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
		fromHC.DeepCopy(),
		getAPIServerProfile(),
		defaultTLSSecurityProfile(),
	)

	// this should never happen, because it is validated in the webhook for HCO, and set in Refresh() for the APIServer CR
	if profile.Type == openshiftconfigv1.TLSProfileCustomType && profile.Custom == nil {
		logf.Log.WithName("tls-security-profile-logger").Info(`WARNING: The provided TLS Security Profile is  wrong: the type is "Custom", but the custom field is not set`)
		intermediateProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType]
		profile.Custom = &openshiftconfigv1.CustomTLSProfile{
			TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
				Ciphers:       slices.Clone(intermediateProfile.Ciphers),
				Groups:        slices.Clone(intermediateProfile.Groups),
				MinTLSVersion: intermediateProfile.MinTLSVersion,
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

	return slices.Clone(openshiftconfigv1.TLSProfiles[profile.Type].Ciphers), openshiftconfigv1.TLSProfiles[profile.Type].MinTLSVersion
}

func GetCipherSuitesAndMinTLSVersionInGolangFormat(fromHC *openshiftconfigv1.TLSSecurityProfile) (ciphers []uint16, minTLSVersion uint16) {
	cipherNames, minTypedTLSVersion := GetCipherSuitesAndMinTLSVersion(fromHC)

	goCiphers := crypto.CipherSuitesOrDie(crypto.OpenSSLToIANACipherSuites(cipherNames))
	goMinTLSVersion := crypto.TLSVersionOrDie(string(minTypedTLSVersion))

	return goCiphers, goMinTLSVersion
}

// GetGroups returns the TLS groups from the effective profile.
// For Custom profiles, returns the custom groups (which may be nil/empty).
// For named profiles (Old/Intermediate/Modern), returns groups from TLSProfiles.
func GetGroups(fromHC *openshiftconfigv1.TLSSecurityProfile) []openshiftconfigv1.TLSGroup {
	profile := GetTLSSecurityProfile(fromHC)

	if profile.Type == openshiftconfigv1.TLSProfileCustomType {
		return profile.Custom.Groups
	}

	return slices.Clone(openshiftconfigv1.TLSProfiles[profile.Type].Groups)
}

// GetGroupsInGolangFormat returns the TLS groups as Go tls.CurveID values.
func GetGroupsInGolangFormat(fromHC *openshiftconfigv1.TLSSecurityProfile) []tls.CurveID {
	groups := GetGroups(fromHC)
	if len(groups) == 0 {
		return nil
	}

	curveIDs, unsupported := crypto.TLSGroupsToCurveIDs(groups)
	if len(unsupported) > 0 {
		logf.Log.WithName("tls-security-profile-logger").Info("unsupported TLS groups ignored", "groups", unsupported)
	}

	return curveIDs
}

func SetHyperConvergedTLSSecurityProfile(fromHC *openshiftconfigv1.TLSSecurityProfile) {
	setHyperConvergedProfile(fromHC)
}

func MutateTLSConfig(cfg *tls.Config) {
	// This callback executes on each client call returning a new config to be used
	// please be aware that the APIServer is using http keepalive so this is going to
	// be executed only after a while for fresh connections and not on existing ones
	cfg.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
		hcProfile := getHyperConvergedProfile()
		cipherSuites, minVersion := GetCipherSuitesAndMinTLSVersionInGolangFormat(hcProfile)
		config := cfg.Clone()

		config.MinVersion = minVersion
		if minVersion < tls.VersionTLS13 {
			config.CipherSuites = cipherSuites
		}

		if curvePrefs := GetGroupsInGolangFormat(hcProfile); len(curvePrefs) > 0 {
			config.CurvePreferences = curvePrefs
		}

		return config, nil
	}
}

var validTLSAdherence = []openshiftconfigv1.TLSAdherencePolicy{
	openshiftconfigv1.TLSAdherencePolicyNoOpinion,
	openshiftconfigv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
	openshiftconfigv1.TLSAdherencePolicyStrictAllComponents,
}

func Refresh(ctx context.Context, cl client.Client) (modified bool, err error) {
	apiServer := &openshiftconfigv1.APIServer{}

	logger := logr.FromContextOrDiscard(ctx)

	key := client.ObjectKey{Name: APIServerCRName}
	err = cl.Get(ctx, key, apiServer)
	if err != nil {
		return false, err
	}

	// HCO is already in StrictAllComponents mode, so we don't really care about the spec.tlsAdherence field
	// We just log for unknown value. But no behavior changes are needed.
	if !slices.Contains(validTLSAdherence, apiServer.Spec.TLSAdherence) {
		logger.Info("Unknown value for the APIServer's spec.tlsAdherence", "value", string(apiServer.Spec.TLSAdherence))
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

	intermediateProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType]
	validatedAPIServerTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
		Type: openshiftconfigv1.TLSProfileCustomType,
		Custom: &openshiftconfigv1.CustomTLSProfile{
			TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
				Ciphers:       slices.Clone(intermediateProfile.Ciphers),
				Groups:        slices.Clone(intermediateProfile.Groups),
				MinTLSVersion: intermediateProfile.MinTLSVersion,
			},
		},
	}

	if apiServerTLSSecurityProfile.Custom == nil {
		logger.Error(nil, "invalid custom configuration for TLSSecurityProfile on the APIServer CR, taking default values", "apiServerTLSSecurityProfile", apiServerTLSSecurityProfile)
		return validatedAPIServerTLSSecurityProfile
	}

	validatedAPIServerTLSSecurityProfile.Custom = &openshiftconfigv1.CustomTLSProfile{
		TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
			MinTLSVersion: apiServerTLSSecurityProfile.Custom.MinTLSVersion,
			Ciphers:       filterAndValidateCiphers(apiServerTLSSecurityProfile.Custom.Ciphers, logger),
			Groups:        filterAndValidateGroups(apiServerTLSSecurityProfile.Custom.Groups, logger),
		},
	}

	return validatedAPIServerTLSSecurityProfile
}

func filterAndValidateCiphers(ciphers []string, logger logr.Logger) []string {
	var filtered []string
	for _, cipher := range ciphers {
		if isValidCipherName(cipher) {
			filtered = append(filtered, cipher)
		} else {
			logger.Error(nil, "invalid cipher name on the APIServer CR, ignoring it", "cipher", cipher)
		}
	}

	return filtered
}

func filterAndValidateGroups(groups []openshiftconfigv1.TLSGroup, logger logr.Logger) []openshiftconfigv1.TLSGroup {
	var filtered []openshiftconfigv1.TLSGroup
	for _, group := range groups {
		if isValidGroupName(group) {
			filtered = append(filtered, group)
		} else {
			logger.Error(nil, "invalid group name on the APIServer CR, ignoring it", "group", group)
		}
	}

	return filtered
}
