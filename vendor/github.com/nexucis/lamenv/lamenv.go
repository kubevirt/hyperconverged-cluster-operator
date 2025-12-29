// Package lamenv is proposing a way to support the environment variable for Golang
// Using this package, you can encode or decode the environment variable with a proper Golang structure.
//
// Source code and other details for the project are available at GitHub:
//
//   https://github.com/Nexucis/lamenv
//
package lamenv

import (
	"os"
	"reflect"
	"strings"
)

var defaultTagSupported = []string{
	"yaml", "json", "mapstructure",
}

// The Unmarshaler interface may be implemented by types to customize their
// behavior when being unmarshaled from a series of environment varialb.
//
// parts is the list of prefix that would composed the final environment variables.
// Note: for the moment, customize the way to decode a type may not work when using a map.
// The main reason is because the system is then not able to determinate which key it has to use when unmarshalling a map.
type Unmarshaler interface {
	UnmarshalEnv(parts []string) error
}

// The Marshaler interface may be implemented by types to customize their
// behavior when being marshaled into a series of environment variable document.
//
// parts is the list of prefix that would composed the final environment variables.
//
// If an error is returned by MarshalEnv, the marshaling procedure stops
// and returns with the provided error.
type Marshaler interface {
	MarshalEnv(parts []string) error
}

// Unmarshal is looking at the object to guess which environment variable is matching.
//
// Maps and pointers (to a struct, string, int, etc) are accepted as object.
// If an internal pointer within a struct is not initialized,
// the lamenv package will initialize it if necessary for unmarshalling the provided data.
// The object parameter must not be nil.
// The parts can be used to inject a prefix of the environment variable
//
// Struct fields are only unmarshalled if they are exported (have an
// upper case first letter), and are unmarshalled using the field name
// uppercased as the default key. Custom keys can be defined via the
// "json", "yaml" and "mapstructure" name in the field tag.
// If multiple tag name are defined, "json" is considered at first, then "yaml" and finally "mapstructure".
//
// Note: When using a map, it's possible for the Unmarshal method to fail because it's finding multiple way to unmarshal
// the same environment variable for different field in the struct (that could be at different depth).
// It's usually because when using a map, the method has to guess which key to use to unmarshal the environment variable.
// And sometimes, it's possible there are several keys found.
//
// Example of how to use it with the following environment variables available:
//    MY_PREFIX_A = 1
//    MY_PREFIX_B = 2
//
//    type T struct {
//    	F int `json:"a,omitempty"`
//    	B int
//    }
//    var t T
//    lamenv.Unmarshal(&t, []string{"MY_PREFIX"})
func Unmarshal(object interface{}, parts []string) error {
	return New().Unmarshal(object, parts)
}

// Marshal serializes the value provided into a series of environment variable.
// The series of environment variable generated will reflect the structure of the value itself
// Maps and pointers (to struct, string, int, etc) are accepted as the object parameter.
//
// Struct fields are only marshalled if they are exported (have an upper case
// first letter), and are marshalled using the field name uppercased as the
// default key. Custom keys may be defined via the "json", "yaml" or "mapstructure" (by default) name in the field
// tag: the content preceding the first comma is used as the key, and the
// following comma-separated options are used to tweak the marshalling process.
//
//
// The field tag format accepted is:
//
//     `(...) json|yaml|mapstructure:"[<key>][,<flag1>[,<flag2>]]" (...)`
//
// The following flags are currently supported:
//
//     omitempty              Only include the field if it's not set to the zero
//                            value for the type or to empty slices or maps.
//                            Zero valued structs will be omitted if all their public
//                            fields are zero.
//
//     inline or squash       Inline the field, which must be a struct,
//                            causing all of its fields or keys to be processed as if
//                            they were part of the outer struct.
//
// In addition, if the key is "-", the field is ignored.
//
// parts is the list of prefix of the future environment variable. It can be empty.
func Marshal(object interface{}, parts []string) error {
	return New().Marshal(object, parts)
}

// Lamenv is the exported struct of the package that can be used to fine-tune the way to unmarshall the different struct.
type Lamenv struct {
	// tagSupports is a list of tag like "yaml", "json"
	// that the code will look at it to know the name of the field
	tagSupports []string
	// env is the map that is representing the list of the environment variable visited
	// The key is the name of the variable.
	// The value is not important, since once the variable would be used, then the key will be removed
	// It will be useful when a map is involved in order to not parse every possible variable
	// but only the one that are still not used.
	env map[string]bool
}

// New is the method to use to initialize the struct Lamenv.
// The struct can then be fine tuned using the appropriate exported method.
func New() *Lamenv {
	env := make(map[string]bool)
	for _, e := range os.Environ() {
		envSplit := strings.SplitN(e, "=", 2)
		if len(envSplit) != 2 {
			continue
		}
		env[envSplit[0]] = true
	}
	return &Lamenv{
		tagSupports: []string{
			"yaml", "json", "mapstructure",
		},
		env: env,
	}
}

// Unmarshal reads the object to guess and find the appropriate environment variable to use for the decoding.
// Once the environment variable matching the field looked is found, it will unmarshall the value and the set the field with it.
func (l *Lamenv) Unmarshal(object interface{}, parts []string) error {
	return l.decode(reflect.ValueOf(object), parts)
}

func (l *Lamenv) Marshal(object interface{}, parts []string) error {
	return l.encode(reflect.ValueOf(object), parts)
}

// AddTagSupport modify the current tag list supported by adding the one passed as a parameter.
// If you prefer to override the default tag list supported by Lamenv, use the method OverrideTagSupport instead.
func (l *Lamenv) AddTagSupport(tags ...string) *Lamenv {
	l.tagSupports = append(l.tagSupports, tags...)
	return l
}

// OverrideTagSupport overrides the current tag list supported by the one passed as a parameter.
// If you prefer to add new tag supported instead of overriding the current list, use the method AddTagSupport instead.
func (l *Lamenv) OverrideTagSupport(tags ...string) *Lamenv {
	l.tagSupports = tags
	return l
}

func (l *Lamenv) lookupTag(tag reflect.StructTag) ([]string, bool) {
	return lookupTag(tag, l.tagSupports)
}

func contains(parts []string) bool {
	variable := buildEnvVariable(parts)
	for _, e := range os.Environ() {
		envSplit := strings.SplitN(e, "=", 2)
		if len(envSplit) != 2 {
			continue
		}
		if strings.Contains(envSplit[0], variable) {
			return true
		}
	}
	return false
}

// lookupEnv is returning:
// 1. the name of the environment variable
// 2. the value of the environment variable
// 3. if the environment variable exists
func lookupEnv(parts []string) (string, string, bool) {
	variable := buildEnvVariable(parts)
	value, ok := os.LookupEnv(variable)
	return variable, value, ok
}

func lookupTag(tag reflect.StructTag, tagSupports []string) ([]string, bool) {
	for _, tagSupport := range tagSupports {
		if s, ok := tag.Lookup(tagSupport); ok {
			return strings.Split(s, ","), ok
		}
	}
	return nil, false
}

func buildEnvVariable(parts []string) string {
	newParts := make([]string, len(parts))
	for i, s := range parts {
		newParts[i] = strings.ToUpper(s)
	}
	return strings.Join(newParts, "_")
}

// containStr returns true if s is one element of series
func containStr(series []string, s string) bool {
	for _, str := range series {
		if str == s {
			return true
		}
	}
	return false
}
