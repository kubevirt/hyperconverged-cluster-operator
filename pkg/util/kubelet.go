package util

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/coreos/ign-converter/translate/v33tov32"
	"github.com/coreos/ign-converter/translate/v34tov33"
	ign2error "github.com/coreos/ignition/config/shared/errors"
	ign2 "github.com/coreos/ignition/config/v2_2"
	ign2types "github.com/coreos/ignition/config/v2_2/types"
	ign3error "github.com/coreos/ignition/v2/config/shared/errors"
	ign3types "github.com/coreos/ignition/v2/config/v3_2/types"
	ign3_4 "github.com/coreos/ignition/v2/config/v3_4"
	ign3_4types "github.com/coreos/ignition/v2/config/v3_4/types"
	"github.com/vincent-petithory/dataurl"
)

// IgnParseWrapper parses rawIgn for both V2 and V3 ignition configs and returns
// a V2 or V3 Config or an error. This wrapper is necessary since V2 and V3 use different parsers.
func IgnParseWrapper(rawIgn []byte) (interface{}, error) {
	// ParseCompatibleVersion will parse any config <= N to version N
	ignCfgV3, rptV3, errV3 := ign3_4.ParseCompatibleVersion(rawIgn)
	if errV3 == nil && !rptV3.IsFatal() {

		// TODO(jkyros): This removes/ignores the kargs for the downconversion to 3.2 since 3.2 doesn't
		// support the kargs fields (and the MCO doesn't either) but it still needs to be okay if we receive one.
		// This is okay because in our MachineConfig render we merge them into MachineConfig kargs before
		// these get stripped out.
		ignCfgV3.KernelArguments = ign3_4types.KernelArguments{}

		// Regardless of the input version it has now been translated to a 3.4 config, downtranslate to 3.3 so we
		// can get down to 3.2
		ignCfgV3_3, errV3_3 := v34tov33.Translate(ignCfgV3)
		if errV3_3 != nil {
			// This case should only be hit if fields that only exist in v3_4 are being used which are not
			// currently supported by the MCO
			return ign3types.Config{}, fmt.Errorf("translating Ignition config 3.4 to 3.3 failed with error: %v", errV3_3)
		}
		// Regardless of the input version it has now been translated to a 3.3 config, downtranslate to 3.2 to match
		// what is used internally
		ignCfgV3_2, errV3_2 := v33tov32.Translate(ignCfgV3_3)
		if errV3_2 != nil {
			// This case should only be hit if fields that only exist in v3_3 are being used which are not
			// currently supported by the MCO
			return ign3types.Config{}, fmt.Errorf("translating Ignition config 3.3 to 3.2 failed with error: %v", errV3_2)
		}
		return ignCfgV3_2, nil
	}

	// ParseCompatibleVersion differentiates between ErrUnknownVersion ("I know what it is and we don't support it") and
	// ErrInvalidVersion ("I can't parse it to find out what it is"), but our old 3.2 logic didn't, so this is here to make sure
	// our error message for invalid version is still helpful.
	if errV3.Error() == ign3error.ErrInvalidVersion.Error() {
		return ign3types.Config{}, fmt.Errorf("parsing Ignition config failed: invalid version. Supported spec versions: 2.2, 3.0, 3.1, 3.2, 3.3, 3.4")
	}

	if errV3.Error() == ign3error.ErrUnknownVersion.Error() {
		ignCfgV2, rptV2, errV2 := ign2.Parse(rawIgn)
		if errV2 == nil && !rptV2.IsFatal() {
			return ignCfgV2, nil
		}

		// If the error is still UnknownVersion it's not a 3.3/3.2/3.1/3.0 or 2.x config, thus unsupported
		if errV2.Error() == ign2error.ErrUnknownVersion.Error() {
			return ign3types.Config{}, fmt.Errorf("parsing Ignition config failed: unknown version. Supported spec versions: 2.2, 3.0, 3.1, 3.2, 3.3, 3.4")
		}
		return ign3types.Config{}, fmt.Errorf("parsing Ignition spec v2 failed with error: %v\nReport: %v", errV2, rptV2)
	}

	return ign3types.Config{}, fmt.Errorf("parsing Ignition config spec v3 failed with error: %v\nReport: %v", errV3, rptV3)
}

// ParseAndConvertConfig parses rawIgn for both V2 and V3 ignition configs and returns
// a V3 or an error.
func ParseAndConvertConfig(rawIgn []byte) (ign3types.Config, error) {
	ignconfigi, err := IgnParseWrapper(rawIgn)
	if err != nil {
		return ign3types.Config{}, fmt.Errorf("failed to parse Ignition config: %w", err)
	}

	switch typedConfig := ignconfigi.(type) {
	case ign3types.Config:
		return ignconfigi.(ign3types.Config), nil
	case ign2types.Config:
		/*ignconfv2, err := removeIgnDuplicateFilesUnitsUsers(ignconfigi.(ign2types.Config))
		if err != nil {
			return ign3types.Config{}, err
		}
		convertedIgnV3, err := convertIgnition2to3(ignconfv2)
		if err != nil {
			return ign3types.Config{}, fmt.Errorf("failed to convert Ignition config spec v2 to v3: %w", err)
		}*/
		return ign3types.Config{}, fmt.Errorf("ignition files v2 are not supported")
	default:
		return ign3types.Config{}, fmt.Errorf("unexpected type for ignition config: %v", typedConfig)
	}
}

func FindKubeletConfig(files []ign3types.File) (ign3types.File, error) {
	for _, file := range files {
		if file.Path == "/etc/kubernetes/kubelet.conf" {
			return file, nil
		}
	}
	return ign3types.File{}, fmt.Errorf("unable to locate the kubelet configuration")
}

// DecodeIgnitionFileContents returns uncompressed, decoded inline file contents.
// This function does not handle remote resources; it assumes they have already
// been fetched.
func DecodeIgnitionFileContents(source, compression *string) ([]byte, error) {
	var contentsBytes []byte

	// To allow writing of "empty" files we'll allow source to be nil
	if source != nil {
		source, err := dataurl.DecodeString(*source)
		if err != nil {
			return []byte{}, fmt.Errorf("could not decode file content string: %w", err)
		}
		if compression != nil {
			switch *compression {
			case "":
				contentsBytes = source.Data
			case "gzip":
				reader, err := gzip.NewReader(bytes.NewReader(source.Data))
				if err != nil {
					return []byte{}, fmt.Errorf("could not create gzip reader: %w", err)
				}
				defer reader.Close()
				contentsBytes, err = io.ReadAll(reader)
				if err != nil {
					return []byte{}, fmt.Errorf("failed decompressing: %w", err)
				}
			default:
				return []byte{}, fmt.Errorf("unsupported compression type %q", *compression)
			}
		} else {
			contentsBytes = source.Data
		}
	}
	return contentsBytes, nil
}
