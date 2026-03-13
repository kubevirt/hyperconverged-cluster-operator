package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
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

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

//go:embed featuregates_list.gotmplt
var templateContent []byte

const (
	featureGatesDetailsFile = "pkg/featuregatedetails/feature-gates.json"
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
	inFile, err := os.Open(featureGatesDetailsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get the input json file; run `make generate` to make sure it created; %w", err)
	}
	defer inFile.Close()

	dec := json.NewDecoder(inFile)

	var featureGates featuregates.FeatureGates
	err = dec.Decode(&featureGates)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the json file; run `make generate` to make sure it created; %w", err)
	}

	featureGates.Sort()

	return toMultilineFeatureGates(featureGates), nil
}

func toMultilineFeatureGates(src featuregates.FeatureGates) []featureGate {
	var dst []featureGate

	for _, fg := range src {
		newFG := featureGate{
			Name:  fg.Name,
			Phase: fg.Phase.String(),
		}

		description := fg.Description
		for len(description) > maxLineLength {
			line := description[:maxLineLength]
			idx := strings.LastIndexByte(line, ' ')
			line = line[:idx]
			description = description[idx+1:]
			newFG.Description = append(newFG.Description, line)
		}
		if len(description) > 0 {
			newFG.Description = append(newFG.Description, description)
		}

		dst = append(dst, newFG)
	}

	return dst
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

		field, err := findFeatureGateInSpecs(genDecl.Specs)
		if err != nil {
			return nil, err
		}

		if field != nil {
			return field, nil
		}
	}

	return nil, errors.New("couldn't find the FeatureGates field")
}

func findFeatureGateInSpecs(specs []ast.Spec) (*ast.Field, error) {
	for _, spec := range specs {
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

		idx := slices.IndexFunc(structType.Fields.List, func(field *ast.Field) bool {
			return len(field.Names) == 1 && field.Names[0].Name == "FeatureGates"
		})

		if idx >= 0 {
			return structType.Fields.List[idx], nil
		}
	}

	return nil, nil
}
