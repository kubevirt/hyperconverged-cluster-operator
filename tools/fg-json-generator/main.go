package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

//go:embed conversion.go.tmpl
var conversionTemplate string

// conversionField holds the info needed to generate conversion code for a single field.
type conversionField struct {
	FieldName string // Go struct field name (e.g. "DownwardMetrics")
	JSONName  string // JSON tag name (e.g. "downwardMetrics")
	IsBeta    bool   // true for beta phase (default=true), false for alpha (default=false)
}

const (
	structName               = "HyperConvergedFeatureGates"
	kubebuilderDefaultPrefix = "// +kubebuilder:default="
)

func main() {
	outFile := flag.String("out", "", "output file path (default: stdout)")
	inFile := flag.String("in", "", "input file path")
	conversion := flag.Bool("conversion", false, "generate Go conversion functions instead of JSON")
	flag.Parse()

	if *inFile == "" {
		fmt.Fprintln(os.Stderr, "usage: fg-json-generator [--out=<file>] [--conversion] --in=<input-file>")
		os.Exit(1)
	}

	file, err := parseFile(*inFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse file: %v", err)
		os.Exit(1)
	}

	st := findStruct(file, structName)
	if st == nil {
		fmt.Fprintf(os.Stderr, "struct %s not found", structName)
		os.Exit(1)
	}

	var data []byte
	if *conversion {
		data, err = generateConversion(st)
	} else {
		data, err = generateJSON(st)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate output: %v\n", err)
		os.Exit(1)
	}

	if *outFile != "" {
		if err := os.WriteFile(*outFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write output file: %v\n", err)
			os.Exit(1)
		}
	} else {
		os.Stdout.Write(data)
	}

	fmt.Fprintln(os.Stderr, "successfully generated the feature gate file")
}

func generateJSON(st *ast.StructType) ([]byte, error) {
	gates := extractFeatureGates(st)
	gates.Sort()

	data, err := json.MarshalIndent(gates, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(data, '\n'), nil
}

func generateConversion(st *ast.StructType) ([]byte, error) {
	fields := extractConversionFields(st)

	tmpl, err := template.New("conversion").Parse(conversionTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, fields); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return []byte(buf.String()), nil
}

// extractConversionFields returns the Go field name, JSON name, and phase info
// for each alpha or beta field in the struct (skipping deprecated and others).
func extractConversionFields(st *ast.StructType) []conversionField {
	var fields []conversionField

	for _, field := range st.Fields.List {
		if field.Tag == nil || len(field.Names) == 0 {
			continue
		}

		jsonName := fieldJSONName(field)
		if jsonName == "" || jsonName == "-" {
			continue
		}

		phase := fieldPhase(field)
		if phase != featuregates.PhaseAlpha && phase != featuregates.PhaseBeta {
			continue
		}

		fields = append(fields, conversionField{
			FieldName: field.Names[0].Name,
			JSONName:  jsonName,
			IsBeta:    phase == featuregates.PhaseBeta,
		})
	}

	return fields
}

func parseFile(path string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, path, nil, parser.ParseComments)
}

// findStruct locates a named struct type declaration in the AST.
func findStruct(file *ast.File, name string) *ast.StructType {
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok || d.Tok != token.TYPE {
			continue
		}

		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}

			st, ok := ts.Type.(*ast.StructType)
			if ok {
				return st
			}
		}
	}
	return nil
}

func extractFeatureGates(st *ast.StructType) featuregates.FeatureGates {
	var gates featuregates.FeatureGates

	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}

		name := fieldJSONName(field)
		if name == "" || name == "-" {
			continue
		}

		fg := featuregates.FeatureGate{
			Name:        name,
			Phase:       fieldPhase(field),
			Description: fieldDescription(field),
		}

		gates = append(gates, fg)
	}

	return gates
}

// fieldJSONName extracts the JSON field name from the struct tag.
func fieldJSONName(field *ast.Field) string {
	tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("json")
	return strings.Split(tag, ",")[0]
}

// fieldPhase determines the lifecycle phase of a feature gate field.
// It checks for "Deprecated:" in comments first, then looks at the kubebuilder default value.
func fieldPhase(field *ast.Field) featuregates.Phase {
	if field.Doc != nil {
		for _, comment := range field.Doc.List {
			text := strings.TrimPrefix(comment.Text, "//")
			text = strings.TrimSpace(text)
			if strings.HasPrefix(strings.ToLower(text), "deprecated") {
				return featuregates.PhaseDeprecated
			}
		}

		for _, comment := range field.Doc.List {
			if strings.HasPrefix(comment.Text, kubebuilderDefaultPrefix) {
				val := comment.Text[len(kubebuilderDefaultPrefix):]
				if val == "true" {
					return featuregates.PhaseBeta
				}
				return featuregates.PhaseAlpha
			}
		}
	}

	return featuregates.PhaseAlpha
}

// fieldDescription extracts the description from field comments,
// filtering out annotation lines (starting with "+") and TODO lines.
func fieldDescription(field *ast.Field) string {
	if field.Doc == nil {
		return ""
	}

	var lines []string
	for _, comment := range field.Doc.List {
		text := strings.TrimPrefix(comment.Text, "//")
		text = strings.TrimSpace(text)

		if text == "" {
			continue
		}

		if strings.HasPrefix(text, "+") {
			continue
		}

		if strings.HasPrefix(text, "TODO") {
			continue
		}

		lines = append(lines, text)
	}

	return strings.Join(lines, " ")
}
