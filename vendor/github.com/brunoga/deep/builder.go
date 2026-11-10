package deep

import (
	"fmt"
	"reflect"
)

// Builder allows constructing a Patch[T] manually with on-the-fly type validation.
type Builder[T any] struct {
	typ   reflect.Type
	patch diffPatch
	err   error
}

// NewBuilder returns a new Builder for type T.
func NewBuilder[T any]() *Builder[T] {
	var t T
	typ := reflect.TypeOf(t)
	return &Builder[T]{
		typ: typ,
	}
}

// Build returns the constructed Patch or an error if any operation was invalid.
func (b *Builder[T]) Build() (Patch[T], error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.patch == nil {
		return nil, nil
	}
	return &typedPatch[T]{inner: b.patch}, nil
}

// Root returns a Node representing the root of the value being patched.
func (b *Builder[T]) Root() *Node {
	return &Node{
		typ: b.typ,
		update: func(p diffPatch) {
			b.patch = p
		},
		current: b.patch,
	}
}

// Node represents a specific location within a value's structure.
type Node struct {
	typ     reflect.Type
	update  func(diffPatch)
	current diffPatch
}

// Set replaces the value at the current node. It requires the 'old' value
// to enable patch reversibility and strict application checking.
func (n *Node) Set(old, new any) error {
	vOld := reflect.ValueOf(old)
	vNew := reflect.ValueOf(new)
	if n.typ != nil {
		if vOld.IsValid() && vOld.Type() != n.typ {
			return fmt.Errorf("invalid old value type: expected %v, got %v", n.typ, vOld.Type())
		}
		if vNew.IsValid() && vNew.Type() != n.typ {
			return fmt.Errorf("invalid new value type: expected %v, got %v", n.typ, vNew.Type())
		}
	}
	n.update(&valuePatch{
		oldVal: deepCopyValue(vOld),
		newVal: deepCopyValue(vNew),
	})
	return nil
}

// Field returns a Node for the specified struct field. It automatically descends
// into pointers and interfaces if necessary.
func (n *Node) Field(name string) (*Node, error) {
	if n.typ.Kind() == reflect.Ptr || n.typ.Kind() == reflect.Interface {
		return n.Elem().Field(name)
	}
	if n.typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("not a struct: %v", n.typ)
	}
	field, ok := n.typ.FieldByName(name)
	if !ok {
		return nil, fmt.Errorf("field not found: %s", name)
	}
	sp, ok := n.current.(*structPatch)
	if !ok {
		sp = &structPatch{fields: make(map[string]diffPatch)}
		n.update(sp)
		n.current = sp
	}
	return &Node{
		typ: field.Type,
		update: func(p diffPatch) {
			sp.fields[name] = p
		},
		current: sp.fields[name],
	}, nil
}

// Index returns a Node for the specified array or slice index.
func (n *Node) Index(i int) (*Node, error) {
	if n.typ.Kind() == reflect.Ptr || n.typ.Kind() == reflect.Interface {
		return n.Elem().Index(i)
	}
	kind := n.typ.Kind()
	if kind != reflect.Slice && kind != reflect.Array {
		return nil, fmt.Errorf("not a slice or array: %v", n.typ)
	}
	if kind == reflect.Array && (i < 0 || i >= n.typ.Len()) {
		return nil, fmt.Errorf("index out of bounds: %d", i)
	}
	if kind == reflect.Array {
		ap, ok := n.current.(*arrayPatch)
		if !ok {
			ap = &arrayPatch{indices: make(map[int]diffPatch)}
			n.update(ap)
			n.current = ap
		}
		return &Node{
			typ: n.typ.Elem(),
			update: func(p diffPatch) {
				ap.indices[i] = p
			},
			current: ap.indices[i],
		}, nil
	}
	sp, ok := n.current.(*slicePatch)
	if !ok {
		sp = &slicePatch{}
		n.update(sp)
		n.current = sp
	}
	var modOp *sliceOp
	for j := range sp.ops {
		if sp.ops[j].Index == i && sp.ops[j].Kind == opMod {
			modOp = &sp.ops[j]
			break
		}
	}
	if modOp == nil {
		sp.ops = append(sp.ops, sliceOp{
			Kind:  opMod,
			Index: i,
		})
		modOp = &sp.ops[len(sp.ops)-1]
	}
	return &Node{
		typ: n.typ.Elem(),
		update: func(p diffPatch) {
			modOp.Patch = p
		},
		current: modOp.Patch,
	}, nil
}

// MapKey returns a Node for the specified map key.
func (n *Node) MapKey(key any) (*Node, error) {
	if n.typ.Kind() == reflect.Ptr || n.typ.Kind() == reflect.Interface {
		return n.Elem().MapKey(key)
	}
	if n.typ.Kind() != reflect.Map {
		return nil, fmt.Errorf("not a map: %v", n.typ)
	}
	vKey := reflect.ValueOf(key)
	if vKey.Type() != n.typ.Key() {
		return nil, fmt.Errorf("invalid key type: expected %v, got %v", n.typ.Key(), vKey.Type())
	}
	mp, ok := n.current.(*mapPatch)
	if !ok {
		mp = &mapPatch{
			added:    make(map[interface{}]reflect.Value),
			removed:  make(map[interface{}]reflect.Value),
			modified: make(map[interface{}]diffPatch),
			keyType:  n.typ.Key(),
		}
		n.update(mp)
		n.current = mp
	}
	return &Node{
		typ: n.typ.Elem(),
		update: func(p diffPatch) {
			mp.modified[key] = p
		},
		current: mp.modified[key],
	}, nil
}

// Elem returns a Node for the element type of a pointer or interface.
func (n *Node) Elem() *Node {
	if n.typ.Kind() != reflect.Ptr && n.typ.Kind() != reflect.Interface {
		return n
	}
	updateFunc := n.update
	var currentPatch diffPatch
	if n.typ.Kind() == reflect.Ptr {
		pp, ok := n.current.(*ptrPatch)
		if !ok {
			pp = &ptrPatch{}
			n.update(pp)
			n.current = pp
		}
		updateFunc = func(p diffPatch) { pp.elemPatch = p }
		currentPatch = pp.elemPatch
	} else {
		ip, ok := n.current.(*interfacePatch)
		if !ok {
			ip = &interfacePatch{}
			n.update(ip)
			n.current = ip
		}
		updateFunc = func(p diffPatch) { ip.elemPatch = p }
		currentPatch = ip.elemPatch
	}
	return &Node{
		typ:     n.typ.Elem(),
		update:  updateFunc,
		current: currentPatch,
	}
}

// Add appends an addition operation to a slice node.
func (n *Node) Add(i int, val any) error {
	if n.typ.Kind() != reflect.Slice {
		return fmt.Errorf("Add only supported on slices, got %v", n.typ)
	}
	v := reflect.ValueOf(val)
	if v.Type() != n.typ.Elem() {
		return fmt.Errorf("invalid value type: expected %v, got %v", n.typ.Elem(), v.Type())
	}
	sp, ok := n.current.(*slicePatch)
	if !ok {
		sp = &slicePatch{}
		n.update(sp)
		n.current = sp
	}
	sp.ops = append(sp.ops, sliceOp{
		Kind:  opAdd,
		Index: i,
		Val:   deepCopyValue(v),
	})
	return nil
}

// Delete appends a deletion operation to a slice or map node.
func (n *Node) Delete(keyOrIndex any, oldVal any) error {
	if n.typ.Kind() == reflect.Slice {
		i, ok := keyOrIndex.(int)
		if !ok {
			return fmt.Errorf("index must be int for slices")
		}
		vOld := reflect.ValueOf(oldVal)
		if vOld.Type() != n.typ.Elem() {
			return fmt.Errorf("invalid old value type: expected %v, got %v", n.typ.Elem(), vOld.Type())
		}
		sp, ok := n.current.(*slicePatch)
		if !ok {
			sp = &slicePatch{}
			n.update(sp)
			n.current = sp
		}
		sp.ops = append(sp.ops, sliceOp{
			Kind:  opDel,
			Index: i,
			Val:   deepCopyValue(vOld),
		})
		return nil
	}
	if n.typ.Kind() == reflect.Map {
		vKey := reflect.ValueOf(keyOrIndex)
		if vKey.Type() != n.typ.Key() {
			return fmt.Errorf("invalid key type: expected %v, got %v", n.typ.Key(), vKey.Type())
		}
		vOld := reflect.ValueOf(oldVal)
		if vOld.Type() != n.typ.Elem() {
			return fmt.Errorf("invalid old value type: expected %v, got %v", n.typ.Elem(), vOld.Type())
		}
		mp, ok := n.current.(*mapPatch)
		if !ok {
			mp = &mapPatch{
				added:    make(map[interface{}]reflect.Value),
				removed:  make(map[interface{}]reflect.Value),
				modified: make(map[interface{}]diffPatch),
				keyType:  n.typ.Key(),
			}
			n.update(mp)
			n.current = mp
		}
		mp.removed[keyOrIndex] = deepCopyValue(vOld)
		return nil
	}
	return fmt.Errorf("Delete only supported on slices and maps, got %v", n.typ)
}

// AddMapEntry adds a new entry to a map node.
func (n *Node) AddMapEntry(key, val any) error {
	if n.typ.Kind() != reflect.Map {
		return fmt.Errorf("AddMapEntry only supported on maps, got %v", n.typ)
	}
	vKey := reflect.ValueOf(key)
	if vKey.Type() != n.typ.Key() {
		return fmt.Errorf("invalid key type: expected %v, got %v", n.typ.Key(), vKey.Type())
	}
	vVal := reflect.ValueOf(val)
	if vVal.Type() != n.typ.Elem() {
		return fmt.Errorf("invalid value type: expected %v, got %v", n.typ.Elem(), vVal.Type())
	}
	mp, ok := n.current.(*mapPatch)
	if !ok {
		mp = &mapPatch{
			added:    make(map[interface{}]reflect.Value),
			removed:  make(map[interface{}]reflect.Value),
			modified: make(map[interface{}]diffPatch),
			keyType:  n.typ.Key(),
		}
		n.update(mp)
		n.current = mp
	}
	mp.added[key] = deepCopyValue(vVal)
	return nil
}
