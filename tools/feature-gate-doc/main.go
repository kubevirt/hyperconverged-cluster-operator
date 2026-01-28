package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"text/template"
)

//go:embed featuregates_list.gotmplt
var templateContent []byte

const (
	Alpha      = "Alpha"
	Beta       = "Beta"
	GA         = "GA"
	Deprecated = "Deprecated"

	featureGatesDetailsFile = "api/v1/featuregates/feature_gates_details.go"
	apiSrcFileName          = "api/v1/hyperconverged_types.go"

	maxLineLength = 75
)

var featureGatesTemplate = template.Must(template.New("featuregates").Parse(string(templateContent)))

type featureGate struct {
	Name        string
	Description []string
	Phase       string
}

func main() {
	featureGates, err := parseFGs()
	if err != nil {
		panic(err)
	}

	newCommentBlockText := &bytes.Buffer{}
	err = featureGatesTemplate.Execute(newCommentBlockText, featureGates)
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(newCommentBlockText)
	err = applyNewComments(scanner)
	if err != nil {
		panic(err)
	}
}

func parseFGs() ([]featureGate, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, featureGatesDetailsFile, nil, 0)
	if err != nil {
		return nil, err
	}

	fgsDecl, err := findFeatureGatesDetailsDecl(file)
	if err != nil {
		return nil, err
	}

	featureGates, err := parseFeatureGateEntries(fgsDecl)
	if err != nil {
		return nil, err
	}

	sortFeatureGates(featureGates)

	return featureGates, nil
}

func findFeatureGatesDetailsDecl(file *ast.File) (*ast.CompositeLit, error) {
	for _, d := range file.Decls {
		decl, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range decl.Specs {
			v, ok := spec.(*ast.ValueSpec)
			if !ok || len(v.Names) != 1 || v.Names[0].Name != "featureGatesDetails" {
				continue
			}

			if len(v.Values) != 1 {
				return nil, fmt.Errorf("featureGatesDetails should have 1 value but has %d", len(v.Values))
			}

			content, ok := v.Values[0].(*ast.CompositeLit)
			if !ok {
				return nil, errors.New("expected a composite literal")
			}

			return content, nil
		}
	}

	return nil, errors.New("couldn't find featureGatesDetails declaration")
}

func parseFeatureGateEntries(fgsDecl *ast.CompositeLit) ([]featureGate, error) {
	var featureGates []featureGate

	for _, elt := range fgsDecl.Elts {
		fg, err := parseFeatureGateEntry(elt)
		if err != nil {
			return nil, err
		}
		featureGates = append(featureGates, fg)
	}

	return featureGates, nil
}

func parseFeatureGateEntry(elt ast.Expr) (featureGate, error) {
	kv, ok := elt.(*ast.KeyValueExpr)
	if !ok {
		return featureGate{}, errors.New("expected a key value expression")
	}

	fgName := strings.Trim(kv.Key.(*ast.BasicLit).Value, `"`)
	fgDetails, ok := kv.Value.(*ast.CompositeLit)
	if !ok {
		return featureGate{}, fmt.Errorf("expected composite literal for feature gate %s", fgName)
	}

	phase, description := parseFeatureGateDetails(fgDetails)

	return featureGate{
		Name:        fgName,
		Description: description,
		Phase:       phase,
	}, nil
}

func parseFeatureGateDetails(fgDetails *ast.CompositeLit) (phase string, description []string) {
	for _, detail := range fgDetails.Elts {
		detailKV := detail.(*ast.KeyValueExpr)
		fieldName := detailKV.Key.(*ast.Ident).Name

		switch fieldName {
		case "phase":
			phase = parsePhase(detailKV.Value.(*ast.Ident).Name)
		case "description":
			description = parseDescription(detailKV.Value)
		}
	}
	return phase, description
}

func parsePhase(phaseIdent string) string {
	switch phaseIdent {
	case "PhaseAlpha":
		return Alpha
	case "PhaseBeta":
		return Beta
	case "PhaseGA":
		return GA
	case "PhaseDeprecated":
		return Deprecated
	default:
		return ""
	}
}

func parseDescription(value ast.Expr) []string {
	switch val := value.(type) {
	case *ast.BasicLit:
		line := strings.Trim(val.Value, `"`)
		line = strings.Trim(line, "`")
		return []string{line}
	case *ast.BinaryExpr:
		return multilinesDescription(val, nil)
	default:
		return nil
	}
}

func sortFeatureGates(featureGates []featureGate) {
	slices.SortFunc(featureGates, func(a, b featureGate) int {
		if phaseCmp := cmpPhase(a.Phase, b.Phase); phaseCmp != 0 {
			return phaseCmp
		}
		return strings.Compare(a.Name, b.Name)
	})
}

func cmpPhase(a, b string) int {
	if a == b {
		return 0
	}

	return toComparablePhase(a) - toComparablePhase(b)
}

func toComparablePhase(phase string) int {
	switch phase {
	case Alpha:
		return 1
	case Beta:
		return 2
	case GA:
		return 3
	case Deprecated:
		return 4
	}

	return 5
}

func applyNewComments(newComments *bufio.Scanner) error {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, apiSrcFileName, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	field, err := findFeatureGatesNode(file)
	if err != nil {
		return err
	}

	list, err := buildCommentList(newComments, field)
	if err != nil {
		return err
	}

	updateCommentInAST(field, list, fset, file)

	return updateAPISrcFile(fset, file)
}

func updateAPISrcFile(fset *token.FileSet, file *ast.File) error {
	b := bytes.Buffer{}
	err := format.Node(&b, fset, file)
	if err != nil {
		return err
	}

	return os.WriteFile(apiSrcFileName, b.Bytes(), 0644)
}

func updateCommentInAST(field *ast.Field, list []*ast.Comment, fset *token.FileSet, file *ast.File) {
	field.Doc = &ast.CommentGroup{
		List: list,
	}

	comments := ast.NewCommentMap(fset, file, file.Comments)
	commentGroups := comments[field]

	if len(commentGroups) == 0 {
		comments[field] = []*ast.CommentGroup{field.Doc}
	} else {
		// don't drop the auto-generated warning comment. Only set the actual doc comment.
		commentGroups[len(commentGroups)-1] = field.Doc
	}

	file.Comments = comments.Filter(file).Comments()
}

func buildCommentList(newComments *bufio.Scanner, field *ast.Field) ([]*ast.Comment, error) {
	var list []*ast.Comment

	for newComments.Scan() {
		list = append(list,
			&ast.Comment{
				Text:  newComments.Text(),
				Slash: field.Pos() - 1,
			},
		)
	}
	if err := newComments.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func findFeatureGatesNode(file *ast.File) (*ast.Field, error) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range genDecl.Specs {
			v, ok := spec.(*ast.TypeSpec)
			if !ok || v.Name.Name != "HyperConvergedSpec" {
				continue
			}

			structType, ok := v.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("expected HyperConvergedSpec to be a struct type")
			}

			if structType.Fields == nil {
				return nil, fmt.Errorf("expected HyperConvergedSpec to not be empty struct")
			}

			for _, field := range structType.Fields.List {
				if len(field.Names) != 1 || field.Names[0].Name != "FeatureGates" {
					continue
				}

				return field, nil
			}
		}
	}

	return nil, errors.New("couldn't find the FeatureGates field")
}

func multilinesDescription(expr *ast.BinaryExpr, lines []string) []string {
	switch val := expr.X.(type) {
	case *ast.BasicLit:
		line := val.Value
		line = strings.Trim(line, `"`)
		line = strings.Trim(line, "`")
		lines = addLine(line, lines)

	case *ast.BinaryExpr:
		lines = multilinesDescription(val, lines)
	}

	line := expr.Y.(*ast.BasicLit).Value
	line = strings.Trim(line, `"`)
	line = strings.Trim(line, "`")
	lines = addLine(line, lines)

	return lines
}

func addLine(line string, lines []string) []string {
	for len(line) > maxLineLength {
		for i := maxLineLength; i > 0; i-- {
			if line[i] == ' ' {
				lines = append(lines, line[:i+1])
				line = line[i+1:]
				break
			}
		}
	}

	lines = append(lines, line)

	return lines
}
