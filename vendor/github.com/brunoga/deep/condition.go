package deep

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/brunoga/deep/internal/unsafe"
)

// Condition represents a logical check against a value of type T.
type Condition[T any] interface {
	Evaluate(v *T) (bool, error)
}

// Path represents a path to a field or element within a structure.
// Syntax: "Field", "Field.SubField", "Slice[0]", "Map.Key", "Ptr.Field".
type Path string

// resolve traverses v using the path and returns the reflect.Value found.
func (p Path) resolve(v reflect.Value) (reflect.Value, error) {
	parts := parsePath(string(p))
	current := v
	for _, part := range parts {
		if !current.IsValid() {
			return reflect.Value{}, fmt.Errorf("path traversal failed: nil value at intermediate step")
		}

		// Automatically dereference pointers and interfaces.
		for current.Kind() == reflect.Ptr || current.Kind() == reflect.Interface {
			if current.IsNil() {
				return reflect.Value{}, fmt.Errorf("path traversal failed: nil pointer/interface")
			}
			current = current.Elem()
		}

		if part.isIndex {
			if current.Kind() != reflect.Slice && current.Kind() != reflect.Array {
				return reflect.Value{}, fmt.Errorf("cannot index into %v", current.Type())
			}
			if part.index < 0 || part.index >= current.Len() {
				return reflect.Value{}, fmt.Errorf("index out of bounds: %d", part.index)
			}
			current = current.Index(part.index)
		} else if current.Kind() == reflect.Map {
			keyType := current.Type().Key()
			var keyVal reflect.Value
			if keyType.Kind() == reflect.String {
				keyVal = reflect.ValueOf(part.key)
			} else if keyType.Kind() == reflect.Int {
				i, err := strconv.Atoi(part.key)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("invalid int key: %s", part.key)
				}
				keyVal = reflect.ValueOf(i)
			} else {
				return reflect.Value{}, fmt.Errorf("unsupported map key type for path: %v", keyType)
			}

			val := current.MapIndex(keyVal)
			if !val.IsValid() {
				return reflect.Value{}, nil
			}
			current = val
		} else {
			if current.Kind() != reflect.Struct {
				return reflect.Value{}, fmt.Errorf("cannot access field %s on %v", part.key, current.Type())
			}

			// We use FieldByName and disableRO to support unexported fields.
			f := current.FieldByName(part.key)
			if !f.IsValid() {
				return reflect.Value{}, fmt.Errorf("field %s not found", part.key)
			}
			unsafe.DisableRO(&f)
			current = f
		}
	}
	return current, nil
}

type pathPart struct {
	key     string
	index   int
	isIndex bool
}

func parsePath(path string) []pathPart {
	var parts []pathPart
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			parts = append(parts, pathPart{key: buf.String()})
			buf.Reset()
		}
	}
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch c {
		case '.':
			flush()
		case '[':
			flush()
			start := i + 1
			for i < len(path) && path[i] != ']' {
				i++
			}
			if i < len(path) {
				content := path[start:i]
				idx, err := strconv.Atoi(content)
				if err == nil {
					parts = append(parts, pathPart{index: idx, isIndex: true})
				}
			}
		default:
			buf.WriteByte(c)
		}
	}
	flush()
	return parts
}

type EqualCondition[T any] struct {
	Path  Path
	Value any
}

func (c EqualCondition[T]) Evaluate(v *T) (bool, error) {
	rv := reflect.ValueOf(v)
	target, err := c.Path.resolve(rv)
	if err != nil {
		return false, err
	}
	if !target.IsValid() {
		return c.Value == nil, nil
	}
	targetVal := target.Interface()
	convertedVal := convertValue(reflect.ValueOf(c.Value), reflect.TypeOf(targetVal)).Interface()
	return reflect.DeepEqual(targetVal, convertedVal), nil
}

func Equal[T any](path string, val any) Condition[T] {
	return EqualCondition[T]{Path: Path(path), Value: val}
}

type NotEqualCondition[T any] struct {
	Path  Path
	Value any
}

func (c NotEqualCondition[T]) Evaluate(v *T) (bool, error) {
	rv := reflect.ValueOf(v)
	target, err := c.Path.resolve(rv)
	if err != nil {
		return false, err
	}
	if !target.IsValid() {
		return c.Value != nil, nil
	}
	targetVal := target.Interface()
	convertedVal := convertValue(reflect.ValueOf(c.Value), reflect.TypeOf(targetVal)).Interface()
	return !reflect.DeepEqual(targetVal, convertedVal), nil
}

func NotEqual[T any](path string, val any) Condition[T] {
	return NotEqualCondition[T]{Path: Path(path), Value: val}
}

type CompareCondition[T any] struct {
	Path Path
	Val  any
	Op   string
}

func (c CompareCondition[T]) Evaluate(v *T) (bool, error) {
	rv := reflect.ValueOf(v)
	target, err := c.Path.resolve(rv)
	if err != nil {
		return false, err
	}
	if !target.IsValid() {
		return false, nil
	}
	tVal := target.Interface()
	v1 := reflect.ValueOf(tVal)
	v2 := convertValue(reflect.ValueOf(c.Val), v1.Type())

	if v1.Kind() != v2.Kind() {
		return false, nil
	}
	switch v1.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i1 := v1.Int()
		i2 := v2.Int()
		switch c.Op {
		case ">":
			return i1 > i2, nil
		case "<":
			return i1 < i2, nil
		case ">=":
			return i1 >= i2, nil
		case "<=":
			return i1 <= i2, nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u1 := v1.Uint()
		u2 := v2.Uint()
		switch c.Op {
		case ">":
			return u1 > u2, nil
		case "<":
			return u1 < u2, nil
		case ">=":
			return u1 >= u2, nil
		case "<=":
			return u1 <= u2, nil
		}
	case reflect.Float32, reflect.Float64:
		f1 := v1.Float()
		f2 := v2.Float()
		switch c.Op {
		case ">":
			return f1 > f2, nil
		case "<":
			return f1 < f2, nil
		case ">=":
			return f1 >= f2, nil
		case "<=":
			return f1 <= f2, nil
		}
	case reflect.String:
		s1 := v1.String()
		s2 := v2.String()
		switch c.Op {
		case ">":
			return s1 > s2, nil
		case "<":
			return s1 < s2, nil
		case ">=":
			return s1 >= s2, nil
		case "<=":
			return s1 <= s2, nil
		}
	}
	return false, fmt.Errorf("unsupported comparison for kind %v", v1.Kind())
}

func Greater[T any](path string, val any) Condition[T] {
	return CompareCondition[T]{Path: Path(path), Val: val, Op: ">"}
}

func Less[T any](path string, val any) Condition[T] {
	return CompareCondition[T]{Path: Path(path), Val: val, Op: "<"}
}

func GreaterEqual[T any](path string, val any) Condition[T] {
	return CompareCondition[T]{Path: Path(path), Val: val, Op: ">="}
}

func LessEqual[T any](path string, val any) Condition[T] {
	return CompareCondition[T]{Path: Path(path), Val: val, Op: "<="}
}

type AndCondition[T any] struct {
	Conditions []Condition[T]
}

func (c AndCondition[T]) Evaluate(v *T) (bool, error) {
	for _, sub := range c.Conditions {
		ok, err := sub.Evaluate(v)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func And[T any](conds ...Condition[T]) Condition[T] {
	return AndCondition[T]{Conditions: conds}
}

type OrCondition[T any] struct {
	Conditions []Condition[T]
}

func (c OrCondition[T]) Evaluate(v *T) (bool, error) {
	for _, sub := range c.Conditions {
		ok, err := sub.Evaluate(v)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func Or[T any](conds ...Condition[T]) Condition[T] {
	return OrCondition[T]{Conditions: conds}
}

type NotCondition[T any] struct {
	C Condition[T]
}

func (c NotCondition[T]) Evaluate(v *T) (bool, error) {
	ok, err := c.C.Evaluate(v)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

func Not[T any](c Condition[T]) Condition[T] {
	return NotCondition[T]{C: c}
}

// ParseCondition parses a string expression into a Condition[T] tree.
func ParseCondition[T any](expr string) (Condition[T], error) {
	p := &parser[T]{lexer: newLexer(expr)}
	p.next()
	return p.parseExpr()
}

type tokenKind int

const (
	tokError tokenKind = iota
	tokEOF
	tokIdent
	tokString
	tokNumber
	tokBool
	tokEq
	tokNeq
	tokGt
	tokLt
	tokGte
	tokLte
	tokAnd
	tokOr
	tokNot
	tokLParen
	tokRParen
)

type token struct {
	kind tokenKind
	val  string
}

type lexer struct {
	input string
	pos   int
}

func newLexer(input string) *lexer {
	return &lexer{input: input}
}

func (l *lexer) next() token {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return token{kind: tokEOF}
	}
	c := l.input[l.pos]
	switch {
	case c == '(':
		l.pos++
		return token{kind: tokLParen, val: "("}
	case c == ')':
		l.pos++
		return token{kind: tokRParen, val: ")"}
	case c == '=' && l.peek() == '=':
		l.pos += 2
		return token{kind: tokEq, val: "=="}
	case c == '!' && l.peek() == '=':
		l.pos += 2
		return token{kind: tokNeq, val: "!="}
	case c == '>' && l.peek() == '=':
		l.pos += 2
		return token{kind: tokGte, val: ">="}
	case c == '>':
		l.pos++
		return token{kind: tokGt, val: ">"}
	case c == '<' && l.peek() == '=':
		l.pos += 2
		return token{kind: tokLte, val: "<="}
	case c == '<':
		l.pos++
		return token{kind: tokLt, val: "<"}
	case c == '\'' || c == '"':
		return l.lexString(c)
	case isDigit(c):
		return l.lexNumber()
	case isAlpha(c):
		return l.lexIdent()
	}
	return token{kind: tokError, val: string(c)}
}

func (l *lexer) peek() byte {
	if l.pos+1 < len(l.input) {
		return l.input[l.pos+1]
	}
	return 0
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) && isWhitespace(l.input[l.pos]) {
		l.pos++
	}
}

func (l *lexer) lexString(quote byte) token {
	l.pos++
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != quote {
		l.pos++
	}
	val := l.input[start:l.pos]
	if l.pos < len(l.input) {
		l.pos++
	}
	return token{kind: tokString, val: val}
}

func (l *lexer) lexNumber() token {
	start := l.pos
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		l.pos++
	}
	return token{kind: tokNumber, val: l.input[start:l.pos]}
}

func (l *lexer) lexIdent() token {
	start := l.pos
	for l.pos < len(l.input) && (isAlpha(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '.' || l.input[l.pos] == '[' || l.input[l.pos] == ']') {
		l.pos++
	}
	val := l.input[start:l.pos]
	upper := strings.ToUpper(val)
	switch upper {
	case "AND":
		return token{kind: tokAnd, val: val}
	case "OR":
		return token{kind: tokOr, val: val}
	case "NOT":
		return token{kind: tokNot, val: val}
	case "TRUE":
		return token{kind: tokBool, val: "true"}
	case "FALSE":
		return token{kind: tokBool, val: "false"}
	}
	return token{kind: tokIdent, val: val}
}

func isWhitespace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool      { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' }

type parser[T any] struct {
	lexer *lexer
	curr  token
}

func (p *parser[T]) next() {
	p.curr = p.lexer.next()
}

func (p *parser[T]) parseExpr() (Condition[T], error) {
	return p.parseOr()
}

func (p *parser[T]) parseOr() (Condition[T], error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.curr.kind == tokOr {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Or(left, right)
	}
	return left, nil
}

func (p *parser[T]) parseAnd() (Condition[T], error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	for p.curr.kind == tokAnd {
		p.next()
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		left = And(left, right)
	}
	return left, nil
}

func (p *parser[T]) parseFactor() (Condition[T], error) {
	switch p.curr.kind {
	case tokNot:
		p.next()
		cond, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		return Not(cond), nil
	case tokLParen:
		p.next()
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.curr.kind != tokRParen {
			return nil, fmt.Errorf("expected ')', got %v", p.curr.val)
		}
		p.next()
		return cond, nil
	case tokIdent:
		return p.parseComparison()
	}
	return nil, fmt.Errorf("unexpected token: %v", p.curr.val)
}

func (p *parser[T]) parseComparison() (Condition[T], error) {
	path := p.curr.val
	p.next()
	opTok := p.curr
	if opTok.kind < tokEq || opTok.kind > tokLte {
		return nil, fmt.Errorf("expected comparison operator, got %v", opTok.val)
	}
	p.next()
	valTok := p.curr
	var val any
	switch valTok.kind {
	case tokString:
		val = valTok.val
	case tokNumber:
		if strings.Contains(valTok.val, ".") {
			f, _ := strconv.ParseFloat(valTok.val, 64)
			val = f
		} else {
			i, _ := strconv.ParseInt(valTok.val, 10, 64)
			val = int(i)
		}
	case tokBool:
		val = (valTok.val == "true")
	default:
		return nil, fmt.Errorf("expected value, got %v", valTok.val)
	}
	p.next()
	switch opTok.kind {
	case tokEq:
		return Equal[T](path, val), nil
	case tokNeq:
		return NotEqual[T](path, val), nil
	case tokGt:
		return Greater[T](path, val), nil
	case tokLt:
		return Less[T](path, val), nil
	case tokGte:
		return GreaterEqual[T](path, val), nil
	case tokLte:
		return LessEqual[T](path, val), nil
	}
	return nil, fmt.Errorf("unsupported operator")
}

func init() {
	gob.Register(&condSurrogate{})
}

type condSurrogate struct {
	Kind string `json:"k" gob:"k"`
	Data any    `json:"d,omitempty" gob:"d,omitempty"`
}

func marshalCondition[T any](c Condition[T]) (any, error) {
	if c == nil {
		return nil, nil
	}
	switch v := c.(type) {
	case EqualCondition[T]:
		return &condSurrogate{
			Kind: "equal",
			Data: map[string]any{
				"p": string(v.Path),
				"v": v.Value,
			},
		}, nil
	case NotEqualCondition[T]:
		return &condSurrogate{
			Kind: "not_equal",
			Data: map[string]any{
				"p": string(v.Path),
				"v": v.Value,
			},
		}, nil
	case CompareCondition[T]:
		return &condSurrogate{
			Kind: "compare",
			Data: map[string]any{
				"p": string(v.Path),
				"v": v.Val,
				"o": v.Op,
			},
		}, nil
	case AndCondition[T]:
		conds := make([]any, 0, len(v.Conditions))
		for _, sub := range v.Conditions {
			s, err := marshalCondition(sub)
			if err != nil {
				return nil, err
			}
			conds = append(conds, s)
		}
		return &condSurrogate{
			Kind: "and",
			Data: conds,
		}, nil
	case OrCondition[T]:
		conds := make([]any, 0, len(v.Conditions))
		for _, sub := range v.Conditions {
			s, err := marshalCondition(sub)
			if err != nil {
				return nil, err
			}
			conds = append(conds, s)
		}
		return &condSurrogate{
			Kind: "or",
			Data: conds,
		}, nil
	case NotCondition[T]:
		sub, err := marshalCondition(v.C)
		if err != nil {
			return nil, err
		}
		return &condSurrogate{
			Kind: "not",
			Data: sub,
		}, nil
	}
	return nil, fmt.Errorf("unknown condition type: %T", c)
}

func unmarshalCondition[T any](data []byte) (Condition[T], error) {
	var s condSurrogate
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return convertFromCondSurrogate[T](&s)
}

func convertFromCondSurrogate[T any](s any) (Condition[T], error) {
	if s == nil {
		return nil, nil
	}

	var kind string
	var data any

	switch v := s.(type) {
	case *condSurrogate:
		kind = v.Kind
		data = v.Data
	case map[string]any:
		kind = v["k"].(string)
		data = v["d"]
	default:
		return nil, fmt.Errorf("invalid condition surrogate type: %T", s)
	}

	switch kind {
	case "equal":
		d := data.(map[string]any)
		return EqualCondition[T]{Path: Path(d["p"].(string)), Value: d["v"]}, nil
	case "not_equal":
		d := data.(map[string]any)
		return NotEqualCondition[T]{Path: Path(d["p"].(string)), Value: d["v"]}, nil
	case "compare":
		d := data.(map[string]any)
		return CompareCondition[T]{Path: Path(d["p"].(string)), Val: d["v"], Op: d["o"].(string)}, nil
	case "and":
		d := data.([]any)
		conds := make([]Condition[T], 0, len(d))
		for _, subData := range d {
			sub, err := convertFromCondSurrogate[T](subData)
			if err != nil {
				return nil, err
			}
			conds = append(conds, sub)
		}
		return AndCondition[T]{Conditions: conds}, nil
	case "or":
		d := data.([]any)
		conds := make([]Condition[T], 0, len(d))
		for _, subData := range d {
			sub, err := convertFromCondSurrogate[T](subData)
			if err != nil {
				return nil, err
			}
			conds = append(conds, sub)
		}
		return OrCondition[T]{Conditions: conds}, nil
	case "not":
		sub, err := convertFromCondSurrogate[T](data)
		if err != nil {
			return nil, err
		}
		return NotCondition[T]{C: sub}, nil
	}

	return nil, fmt.Errorf("unknown condition kind: %s", kind)
}
