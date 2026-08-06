package main

import (
	_ "embed"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strings"
	"text/template"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

//go:embed conversion.go.tmpl
var conversionTemplate string

// conversionField holds the info needed to generate conversion code for a single field.
type conversionField struct {
	FieldName string // Go struct field name (e.g. "DownwardMetrics")
	JSONName  string // JSON tag name (e.g. "downwardMetrics")
	Phase     featuregates.Phase
}

func (field conversionField) IsBeta() bool {
	return field.Phase == featuregates.PhaseBeta
}

func (field conversionField) IsDeprecated() bool {
	return field.Phase == featuregates.PhaseDeprecated
}

const (
	structName = "HyperConvergedFeatureGates"
)

func main() {
	outFile := flag.String("out", "", "output file path (default: stdout)")
	inFile := flag.String("in", "", "input file path")
	flag.Parse()

	if *inFile == "" {
		fmt.Fprintln(os.Stderr, "usage: fg-conversion-generator [--out=<file>] --in=<input-file>")
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

	data, err := generateConversion(st)

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

		phase, exists := featuregatedetails.GetFeatureGatePhase(jsonName)
		if !exists {
			continue
		}

		switch phase {
		case featuregates.PhaseAlpha, featuregates.PhaseBeta, featuregates.PhaseDeprecated:
		default:
			continue
		}

		fields = append(fields, conversionField{
			FieldName: field.Names[0].Name,
			JSONName:  jsonName,
			Phase:     phase,
		})
	}

	slices.SortFunc(fields, func(a, b conversionField) int {
		if delta := a.Phase - b.Phase; delta != 0 {
			return int(delta)
		}

		return strings.Compare(a.JSONName, b.JSONName)
	})

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

// fieldJSONName extracts the JSON field name from the struct tag.
func fieldJSONName(field *ast.Field) string {
	tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("json")
	return strings.Split(tag, ",")[0]
}
