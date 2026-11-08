package deep

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/brunoga/deep/internal/unsafe"
)

// Patch represents a set of changes that can be applied to a value of type T.
type Patch[T any] interface {
	fmt.Stringer

	// Apply applies the patch to the value pointed to by v.
	// The value v must not be nil.
	Apply(v *T)

	// ApplyChecked applies the patch only if specific conditions are met.
	// 1. If the patch has a global Condition, it must evaluate to true.
	// 2. For every modification, the target value must match the 'oldVal' recorded in the patch.
	ApplyChecked(v *T) error

	// WithCondition returns a new Patch with the given condition attached.
	WithCondition(c Condition[T]) Patch[T]

	// Reverse returns a new Patch that undoes the changes in this patch.
	Reverse() Patch[T]
}

// NewPatch returns a new, empty patch for type T.
func NewPatch[T any]() Patch[T] {
	return &typedPatch[T]{}
}

// Register registers the Patch implementation for type T with the gob package.
// This is required if you want to use Gob serialization with Patch[T].
func Register[T any]() {
	gob.Register(&typedPatch[T]{})
}

type typedPatch[T any] struct {
	inner diffPatch
	cond  Condition[T]
}

func (p *typedPatch[T]) Apply(v *T) {
	if p.inner == nil {
		return
	}
	rv := reflect.ValueOf(v).Elem()
	p.inner.apply(rv)
}

func (p *typedPatch[T]) ApplyChecked(v *T) error {
	if p.cond != nil {
		ok, err := p.cond.Evaluate(v)
		if err != nil {
			return fmt.Errorf("condition evaluation failed: %w", err)
		}
		if !ok {
			return fmt.Errorf("condition failed")
		}
	}

	if p.inner == nil {
		return nil
	}

	rv := reflect.ValueOf(v).Elem()
	return p.inner.applyChecked(rv)
}

func (p *typedPatch[T]) WithCondition(c Condition[T]) Patch[T] {
	return &typedPatch[T]{
		inner: p.inner,
		cond:  c,
	}
}

func (p *typedPatch[T]) Reverse() Patch[T] {
	if p.inner == nil {
		return &typedPatch[T]{}
	}
	return &typedPatch[T]{inner: p.inner.reverse()}
}

func (p *typedPatch[T]) String() string {
	if p.inner == nil {
		return "<nil>"
	}
	return p.inner.format(0)
}

func (p *typedPatch[T]) MarshalJSON() ([]byte, error) {
	inner, err := marshalDiffPatch(p.inner)
	if err != nil {
		return nil, err
	}
	cond, err := marshalCondition(p.cond)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"inner": inner,
		"cond":  cond,
	})
}

func (p *typedPatch[T]) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if innerData, ok := m["inner"]; ok && len(innerData) > 0 && string(innerData) != "null" {
		inner, err := unmarshalDiffPatch(innerData)
		if err != nil {
			return err
		}
		p.inner = inner
	}
	if condData, ok := m["cond"]; ok && len(condData) > 0 && string(condData) != "null" {
		cond, err := unmarshalCondition[T](condData)
		if err != nil {
			return err
		}
		p.cond = cond
	}
	return nil
}

func (p *typedPatch[T]) GobEncode() ([]byte, error) {
	inner, err := marshalDiffPatch(p.inner)
	if err != nil {
		return nil, err
	}
	cond, err := marshalCondition(p.cond)
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(map[string]any{
		"inner": inner,
		"cond":  cond,
	}); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func (p *typedPatch[T]) GobDecode(data []byte) error {
	var m map[string]any
	dec := gob.NewDecoder(strings.NewReader(string(data)))
	if err := dec.Decode(&m); err != nil {
		return err
	}
	if innerData, ok := m["inner"]; ok && innerData != nil {
		// If it's Gob, innerData might already be the right type if registered,
		// but we use our surrogate for consistency.
		inner, err := convertFromSurrogate(innerData)
		if err != nil {
			return err
		}
		p.inner = inner
	}
	if condData, ok := m["cond"]; ok && condData != nil {
		cond, err := convertFromCondSurrogate[T](condData)
		if err != nil {
			return err
		}
		p.cond = cond
	}
	return nil
}

// diffPatch is the internal recursive interface for all patch types.
type diffPatch interface {
	apply(v reflect.Value)
	applyChecked(v reflect.Value) error
	reverse() diffPatch
	format(indent int) string
}

// valuePatch handles replacement of basic types and full replacement of complex types.
type valuePatch struct {
	oldVal reflect.Value
	newVal reflect.Value
}

func (p *valuePatch) apply(v reflect.Value) {
	if !v.CanSet() {
		unsafe.DisableRO(&v)
	}
	setValue(v, p.newVal)
}

func init() {
	gob.Register(&patchSurrogate{})
	gob.Register(map[string]any{})
	gob.Register([]any{})
	gob.Register([]map[string]any{})
}

func (p *valuePatch) applyChecked(v reflect.Value) error {
	if p.oldVal.IsValid() {
		if v.IsValid() {
			convertedOldVal := convertValue(p.oldVal, v.Type())
			if !reflect.DeepEqual(v.Interface(), convertedOldVal.Interface()) {
				return fmt.Errorf("value mismatch: expected %v, got %v", convertedOldVal, v)
			}
		} else {
			return fmt.Errorf("value mismatch: expected %v, got invalid", p.oldVal)
		}
	}
	p.apply(v)
	return nil
}

func (p *valuePatch) reverse() diffPatch {
	return &valuePatch{oldVal: p.newVal, newVal: p.oldVal}
}

func (p *valuePatch) format(indent int) string {
	if !p.oldVal.IsValid() && !p.newVal.IsValid() {
		return "nil"
	}
	oldStr := "nil"
	if p.oldVal.IsValid() {
		oldStr = fmt.Sprintf("%v", p.oldVal)
	}
	newStr := "nil"
	if p.newVal.IsValid() {
		newStr = fmt.Sprintf("%v", p.newVal)
	}
	return fmt.Sprintf("%s -> %s", oldStr, newStr)
}

// ptrPatch handles changes to the content pointed to by a pointer.
type ptrPatch struct {
	elemPatch diffPatch
}

func (p *ptrPatch) apply(v reflect.Value) {
	if v.IsNil() {
		val := reflect.New(v.Type().Elem())
		p.elemPatch.apply(val.Elem())
		v.Set(val)
		return
	}
	p.elemPatch.apply(v.Elem())
}

func (p *ptrPatch) applyChecked(v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("cannot apply pointer patch to nil value")
	}
	return p.elemPatch.applyChecked(v.Elem())
}

func (p *ptrPatch) reverse() diffPatch {
	return &ptrPatch{elemPatch: p.elemPatch.reverse()}
}

func (p *ptrPatch) format(indent int) string {
	return p.elemPatch.format(indent)
}

// interfacePatch handles changes to the value stored in an interface.
type interfacePatch struct {
	elemPatch diffPatch
}

func (p *interfacePatch) apply(v reflect.Value) {
	if v.IsNil() {
		return
	}
	elem := v.Elem()
	newElem := reflect.New(elem.Type()).Elem()
	newElem.Set(elem)
	p.elemPatch.apply(newElem)
	v.Set(newElem)
}

func (p *interfacePatch) applyChecked(v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("cannot apply interface patch to nil value")
	}
	elem := v.Elem()
	newElem := reflect.New(elem.Type()).Elem()
	newElem.Set(elem)
	if err := p.elemPatch.applyChecked(newElem); err != nil {
		return err
	}
	v.Set(newElem)
	return nil
}

func (p *interfacePatch) reverse() diffPatch {
	return &interfacePatch{elemPatch: p.elemPatch.reverse()}
}

func (p *interfacePatch) format(indent int) string {
	return p.elemPatch.format(indent)
}

// structPatch handles field-level modifications in a struct.
type structPatch struct {
	fields map[string]diffPatch
}

func (p *structPatch) apply(v reflect.Value) {
	for name, patch := range p.fields {
		f := v.FieldByName(name)
		if f.IsValid() {
			if !f.CanSet() {
				unsafe.DisableRO(&f)
			}
			patch.apply(f)
		}
	}
}

func (p *structPatch) applyChecked(v reflect.Value) error {
	for name, patch := range p.fields {
		f := v.FieldByName(name)
		if !f.IsValid() {
			return fmt.Errorf("field %s not found", name)
		}
		if !f.CanSet() {
			unsafe.DisableRO(&f)
		}
		if err := patch.applyChecked(f); err != nil {
			return fmt.Errorf("field %s: %w", name, err)
		}
	}
	return nil
}

func (p *structPatch) reverse() diffPatch {
	newFields := make(map[string]diffPatch)
	for k, v := range p.fields {
		newFields[k] = v.reverse()
	}
	return &structPatch{fields: newFields}
}

func (p *structPatch) format(indent int) string {
	var b strings.Builder
	b.WriteString("Struct{\n")
	prefix := strings.Repeat("  ", indent+1)
	for name, patch := range p.fields {
		b.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, name, patch.format(indent+1)))
	}
	b.WriteString(strings.Repeat("  ", indent) + "}")
	return b.String()
}

// arrayPatch handles index-level modifications in a fixed-size array.
type arrayPatch struct {
	indices map[int]diffPatch
}

func (p *arrayPatch) apply(v reflect.Value) {
	for i, patch := range p.indices {
		if i < v.Len() {
			e := v.Index(i)
			if !e.CanSet() {
				unsafe.DisableRO(&e)
			}
			patch.apply(e)
		}
	}
}

func (p *arrayPatch) applyChecked(v reflect.Value) error {
	for i, patch := range p.indices {
		if i >= v.Len() {
			return fmt.Errorf("index %d out of bounds", i)
		}
		e := v.Index(i)
		if !e.CanSet() {
			unsafe.DisableRO(&e)
		}
		if err := patch.applyChecked(e); err != nil {
			return fmt.Errorf("index %d: %w", i, err)
		}
	}
	return nil
}

func (p *arrayPatch) reverse() diffPatch {
	newIndices := make(map[int]diffPatch)
	for k, v := range p.indices {
		newIndices[k] = v.reverse()
	}
	return &arrayPatch{indices: newIndices}
}

func (p *arrayPatch) format(indent int) string {
	var b strings.Builder
	b.WriteString("Array{\n")
	prefix := strings.Repeat("  ", indent+1)
	for i, patch := range p.indices {
		b.WriteString(fmt.Sprintf("%s[%d]: %s\n", prefix, i, patch.format(indent+1)))
	}
	b.WriteString(strings.Repeat("  ", indent) + "}")
	return b.String()
}

// mapPatch handles additions, removals, and modifications in a map.
type mapPatch struct {
	added    map[interface{}]reflect.Value
	removed  map[interface{}]reflect.Value
	modified map[interface{}]diffPatch
	keyType  reflect.Type
}

func (p *mapPatch) apply(v reflect.Value) {
	if v.IsNil() {
		if len(p.added) > 0 {
			newMap := reflect.MakeMap(v.Type())
			v.Set(newMap)
		} else {
			return
		}
	}
	for k := range p.removed {
		v.SetMapIndex(convertValue(reflect.ValueOf(k), v.Type().Key()), reflect.Value{})
	}
	for k, patch := range p.modified {
		keyVal := convertValue(reflect.ValueOf(k), v.Type().Key())
		elem := v.MapIndex(keyVal)
		if elem.IsValid() {
			newElem := reflect.New(elem.Type()).Elem()
			newElem.Set(elem)
			patch.apply(newElem)
			v.SetMapIndex(keyVal, newElem)
		}
	}
	for k, val := range p.added {
		keyVal := convertValue(reflect.ValueOf(k), v.Type().Key())
		v.SetMapIndex(keyVal, convertValue(val, v.Type().Elem()))
	}
}

func (p *mapPatch) applyChecked(v reflect.Value) error {
	if v.IsNil() {
		if len(p.added) > 0 {
			newMap := reflect.MakeMap(v.Type())
			v.Set(newMap)
		} else if len(p.removed) > 0 || len(p.modified) > 0 {
			return fmt.Errorf("cannot modify/remove from nil map")
		}
	}
	for k, oldVal := range p.removed {
		keyVal := convertValue(reflect.ValueOf(k), v.Type().Key())
		val := v.MapIndex(keyVal)
		if !val.IsValid() {
			return fmt.Errorf("key %v not found for removal", k)
		}
		if !reflect.DeepEqual(val.Interface(), oldVal.Interface()) {
			return fmt.Errorf("map removal mismatch for key %v: expected %v, got %v", k, oldVal, val)
		}
	}
	for k, patch := range p.modified {
		keyVal := convertValue(reflect.ValueOf(k), v.Type().Key())
		val := v.MapIndex(keyVal)
		if !val.IsValid() {
			return fmt.Errorf("key %v not found for modification", k)
		}
		newElem := reflect.New(val.Type()).Elem()
		newElem.Set(val)
		if err := patch.applyChecked(newElem); err != nil {
			return fmt.Errorf("key %v: %w", k, err)
		}
		v.SetMapIndex(keyVal, newElem)
	}
	for k := range p.removed {
		v.SetMapIndex(reflect.ValueOf(k), reflect.Value{})
	}
	for k, val := range p.added {
		keyVal := convertValue(reflect.ValueOf(k), v.Type().Key())
		curr := v.MapIndex(keyVal)
		if curr.IsValid() {
			return fmt.Errorf("key %v already exists", k)
		}
		v.SetMapIndex(keyVal, convertValue(val, v.Type().Elem()))
	}
	return nil
}

func (p *mapPatch) reverse() diffPatch {
	newModified := make(map[interface{}]diffPatch)
	for k, v := range p.modified {
		newModified[k] = v.reverse()
	}
	return &mapPatch{
		added:    p.removed,
		removed:  p.added,
		modified: newModified,
		keyType:  p.keyType,
	}
}

func (p *mapPatch) format(indent int) string {
	var b strings.Builder
	b.WriteString("Map{\n")
	prefix := strings.Repeat("  ", indent+1)
	for k, v := range p.added {
		b.WriteString(fmt.Sprintf("%s+ %v: %v\n", prefix, k, v))
	}
	for k := range p.removed {
		b.WriteString(fmt.Sprintf("%s- %v\n", prefix, k))
	}
	for k, patch := range p.modified {
		b.WriteString(fmt.Sprintf("%s  %v: %s\n", prefix, k, patch.format(indent+1)))
	}
	b.WriteString(strings.Repeat("  ", indent) + "}")
	return b.String()
}

type opKind int

const (
	opAdd opKind = iota
	opDel
	opMod
)

type sliceOp struct {
	Kind  opKind
	Index int
	Val   reflect.Value
	Patch diffPatch
}

// slicePatch handles complex edits (insertions, deletions, modifications) in a slice.
type slicePatch struct {
	ops []sliceOp
}

func (p *slicePatch) apply(v reflect.Value) {
	newSlice := reflect.MakeSlice(v.Type(), 0, v.Len())
	curIdx := 0
	for _, op := range p.ops {
		if op.Index > curIdx {
			for k := curIdx; k < op.Index; k++ {
				if k < v.Len() {
					newSlice = reflect.Append(newSlice, v.Index(k))
				}
			}
			curIdx = op.Index
		}
		switch op.Kind {
		case opAdd:
			newSlice = reflect.Append(newSlice, convertValue(op.Val, v.Type().Elem()))
		case opDel:
			curIdx++
		case opMod:
			if curIdx < v.Len() {
				elem := deepCopyValue(v.Index(curIdx))
				if op.Patch != nil {
					op.Patch.apply(elem)
				}
				newSlice = reflect.Append(newSlice, elem)
				curIdx++
			}
		}
	}
	for k := curIdx; k < v.Len(); k++ {
		newSlice = reflect.Append(newSlice, v.Index(k))
	}
	v.Set(newSlice)
}

func (p *slicePatch) applyChecked(v reflect.Value) error {
	newSlice := reflect.MakeSlice(v.Type(), 0, v.Len())
	curIdx := 0
	for _, op := range p.ops {
		if op.Index > curIdx {
			for k := curIdx; k < op.Index; k++ {
				if k < v.Len() {
					newSlice = reflect.Append(newSlice, v.Index(k))
				}
			}
			curIdx = op.Index
		}
		switch op.Kind {
		case opAdd:
			newSlice = reflect.Append(newSlice, convertValue(op.Val, v.Type().Elem()))
		case opDel:
			if curIdx >= v.Len() {
				return fmt.Errorf("slice deletion index %d out of bounds", curIdx)
			}
			curr := v.Index(curIdx)
			if op.Val.IsValid() {
				convertedVal := convertValue(op.Val, v.Type().Elem())
				if !reflect.DeepEqual(curr.Interface(), convertedVal.Interface()) {
					return fmt.Errorf("slice deletion mismatch at %d: expected %v, got %v", curIdx, convertedVal, curr)
				}
			}
			curIdx++
		case opMod:
			if curIdx >= v.Len() {
				return fmt.Errorf("slice modification index %d out of bounds", curIdx)
			}
			elem := deepCopyValue(v.Index(curIdx))
			if err := op.Patch.applyChecked(elem); err != nil {
				return fmt.Errorf("slice index %d: %w", curIdx, err)
			}
			newSlice = reflect.Append(newSlice, elem)
			curIdx++
		}
	}
	for k := curIdx; k < v.Len(); k++ {
		newSlice = reflect.Append(newSlice, v.Index(k))
	}
	v.Set(newSlice)
	return nil
}

func (p *slicePatch) reverse() diffPatch {
	var revOps []sliceOp
	curA := 0
	curB := 0
	for _, op := range p.ops {
		delta := op.Index - curA
		curB += delta
		curA = op.Index
		switch op.Kind {
		case opAdd:
			revOps = append(revOps, sliceOp{
				Kind:  opDel,
				Index: curB,
				Val:   op.Val,
			})
			curB++
		case opDel:
			revOps = append(revOps, sliceOp{
				Kind:  opAdd,
				Index: curB,
				Val:   op.Val,
			})
			curA++
		case opMod:
			revOps = append(revOps, sliceOp{
				Kind:  opMod,
				Index: curB,
				Patch: op.Patch.reverse(),
			})
			curA++
			curB++
		}
	}
	return &slicePatch{ops: revOps}
}

func (p *slicePatch) format(indent int) string {
	var b strings.Builder
	b.WriteString("Slice{\n")
	prefix := strings.Repeat("  ", indent+1)
	for _, op := range p.ops {
		switch op.Kind {
		case opAdd:
			b.WriteString(fmt.Sprintf("%s+ [%d]: %v\n", prefix, op.Index, op.Val))
		case opDel:
			b.WriteString(fmt.Sprintf("%s- [%d]\n", prefix, op.Index))
		case opMod:
			b.WriteString(fmt.Sprintf("%s  [%d]: %s\n", prefix, op.Index, op.Patch.format(indent+1)))
		}
	}
	b.WriteString(strings.Repeat("  ", indent) + "}")
	return b.String()
}

type patchSurrogate struct {
	Kind string `json:"k" gob:"k"`
	Data any    `json:"d,omitempty" gob:"d,omitempty"`
}

func marshalDiffPatch(p diffPatch) (any, error) {
	if p == nil {
		return nil, nil
	}
	switch v := p.(type) {
	case *valuePatch:
		return &patchSurrogate{
			Kind: "value",
			Data: map[string]any{
				"o": valueToInterface(v.oldVal),
				"n": valueToInterface(v.newVal),
			},
		}, nil
	case *ptrPatch:
		elem, err := marshalDiffPatch(v.elemPatch)
		if err != nil {
			return nil, err
		}
		return &patchSurrogate{
			Kind: "ptr",
			Data: elem,
		}, nil
	case *interfacePatch:
		elem, err := marshalDiffPatch(v.elemPatch)
		if err != nil {
			return nil, err
		}
		return &patchSurrogate{
			Kind: "interface",
			Data: elem,
		}, nil
	case *structPatch:
		fields := make(map[string]any)
		for name, patch := range v.fields {
			p, err := marshalDiffPatch(patch)
			if err != nil {
				return nil, err
			}
			fields[name] = p
		}
		return &patchSurrogate{
			Kind: "struct",
			Data: fields,
		}, nil
	case *arrayPatch:
		indices := make(map[string]any)
		for idx, patch := range v.indices {
			p, err := marshalDiffPatch(patch)
			if err != nil {
				return nil, err
			}
			indices[fmt.Sprintf("%d", idx)] = p
		}
		return &patchSurrogate{
			Kind: "array",
			Data: indices,
		}, nil
	case *mapPatch:
		added := make([]map[string]any, 0, len(v.added))
		for k, val := range v.added {
			added = append(added, map[string]any{"k": k, "v": valueToInterface(val)})
		}
		removed := make([]map[string]any, 0, len(v.removed))
		for k, val := range v.removed {
			removed = append(removed, map[string]any{"k": k, "v": valueToInterface(val)})
		}
		modified := make([]map[string]any, 0, len(v.modified))
		for k, patch := range v.modified {
			p, err := marshalDiffPatch(patch)
			if err != nil {
				return nil, err
			}
			modified = append(modified, map[string]any{"k": k, "p": p})
		}
		return &patchSurrogate{
			Kind: "map",
			Data: map[string]any{
				"a": added,
				"r": removed,
				"m": modified,
			},
		}, nil
	case *slicePatch:
		ops := make([]map[string]any, 0, len(v.ops))
		for _, op := range v.ops {
			p, err := marshalDiffPatch(op.Patch)
			if err != nil {
				return nil, err
			}
			ops = append(ops, map[string]any{
				"k": int(op.Kind),
				"i": op.Index,
				"v": valueToInterface(op.Val),
				"p": p,
			})
		}
		return &patchSurrogate{
			Kind: "slice",
			Data: ops,
		}, nil
	}
	return nil, fmt.Errorf("unknown patch type: %T", p)
}

func unmarshalDiffPatch(data []byte) (diffPatch, error) {
	var s patchSurrogate
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return convertFromSurrogate(&s)
}

func convertFromSurrogate(s any) (diffPatch, error) {
	if s == nil {
		return nil, nil
	}

	var kind string
	var data any

	switch v := s.(type) {
	case *patchSurrogate:
		kind = v.Kind
		data = v.Data
	case map[string]any:
		kind = v["k"].(string)
		data = v["d"]
	default:
		return nil, fmt.Errorf("invalid surrogate type: %T", s)
	}

	switch kind {
	case "value":
		d := data.(map[string]any)
		return &valuePatch{
			oldVal: interfaceToValue(d["o"]),
			newVal: interfaceToValue(d["n"]),
		}, nil
	case "ptr":
		elem, err := convertFromSurrogate(data)
		if err != nil {
			return nil, err
		}
		return &ptrPatch{elemPatch: elem}, nil
	case "interface":
		elem, err := convertFromSurrogate(data)
		if err != nil {
			return nil, err
		}
		return &interfacePatch{elemPatch: elem}, nil
	case "struct":
		d := data.(map[string]any)
		fields := make(map[string]diffPatch)
		for name, pData := range d {
			p, err := convertFromSurrogate(pData)
			if err != nil {
				return nil, err
			}
			fields[name] = p
		}
		return &structPatch{fields: fields}, nil
	case "array":
		d := data.(map[string]any)
		indices := make(map[int]diffPatch)
		for idxStr, pData := range d {
			var idx int
			fmt.Sscanf(idxStr, "%d", &idx)
			p, err := convertFromSurrogate(pData)
			if err != nil {
				return nil, err
			}
			indices[idx] = p
		}
		return &arrayPatch{indices: indices}, nil
	case "map":
		d := data.(map[string]any)
		added := make(map[interface{}]reflect.Value)
		if a := d["a"]; a != nil {
			if slice, ok := a.([]any); ok {
				for _, entry := range slice {
					e := entry.(map[string]any)
					added[e["k"]] = interfaceToValue(e["v"])
				}
			} else if slice, ok := a.([]map[string]any); ok {
				for _, e := range slice {
					added[e["k"]] = interfaceToValue(e["v"])
				}
			}
		}
		removed := make(map[interface{}]reflect.Value)
		if r := d["r"]; r != nil {
			if slice, ok := r.([]any); ok {
				for _, entry := range slice {
					e := entry.(map[string]any)
					removed[e["k"]] = interfaceToValue(e["v"])
				}
			} else if slice, ok := r.([]map[string]any); ok {
				for _, e := range slice {
					removed[e["k"]] = interfaceToValue(e["v"])
				}
			}
		}
		modified := make(map[interface{}]diffPatch)
		if m := d["m"]; m != nil {
			if slice, ok := m.([]any); ok {
				for _, entry := range slice {
					e := entry.(map[string]any)
					p, err := convertFromSurrogate(e["p"])
					if err != nil {
						return nil, err
					}
					modified[e["k"]] = p
				}
			} else if slice, ok := m.([]map[string]any); ok {
				for _, e := range slice {
					p, err := convertFromSurrogate(e["p"])
					if err != nil {
						return nil, err
					}
					modified[e["k"]] = p
				}
			}
		}
		return &mapPatch{
			added:    added,
			removed:  removed,
			modified: modified,
		}, nil
	case "slice":
		var opsData []map[string]any
		if slice, ok := data.([]any); ok {
			for _, entry := range slice {
				opsData = append(opsData, entry.(map[string]any))
			}
		} else if slice, ok := data.([]map[string]any); ok {
			opsData = slice
		}

		ops := make([]sliceOp, 0, len(opsData))
		for _, o := range opsData {
			p, err := convertFromSurrogate(o["p"])
			if err != nil {
				return nil, err
			}

			var kind float64
			switch k := o["k"].(type) {
			case float64:
				kind = k
			case int:
				kind = float64(k)
			}

			var index float64
			switch i := o["i"].(type) {
			case float64:
				index = i
			case int:
				index = float64(i)
			}

			ops = append(ops, sliceOp{
				Kind:  opKind(int(kind)),
				Index: int(index),
				Val:   interfaceToValue(o["v"]),
				Patch: p,
			})
		}
		return &slicePatch{ops: ops}, nil
	}
	return nil, fmt.Errorf("unknown patch kind: %s", kind)
}

func convertValue(v reflect.Value, targetType reflect.Type) reflect.Value {
	if !v.IsValid() {
		return reflect.Zero(targetType)
	}

	if v.Type().AssignableTo(targetType) {
		return v
	}

	if v.Type().ConvertibleTo(targetType) {
		return v.Convert(targetType)
	}

	// Handle JSON/Gob numbers
	if v.Kind() == reflect.Float64 {
		switch targetType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(int64(v.Float())).Convert(targetType)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return reflect.ValueOf(uint64(v.Float())).Convert(targetType)
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(v.Float()).Convert(targetType)
		}
	}

	return v
}

func setValue(v, newVal reflect.Value) {
	if !newVal.IsValid() {
		if v.CanSet() {
			v.Set(reflect.Zero(v.Type()))
		}
		return
	}

	converted := convertValue(newVal, v.Type())
	v.Set(converted)
}

func valueToInterface(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	if !v.CanInterface() {
		unsafe.DisableRO(&v)
	}
	return v.Interface()
}

func interfaceToValue(i any) reflect.Value {
	if i == nil {
		return reflect.Value{}
	}
	return reflect.ValueOf(i)
}
