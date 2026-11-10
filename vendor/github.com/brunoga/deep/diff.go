package deep

import (
	"reflect"

	"github.com/brunoga/deep/internal/unsafe"
)

// Diff compares two values a and b and returns a Patch that can be applied
// to a to make it equal to b.
//
// It uses a combination of Myers' Diff algorithm for slices and recursive
// type-specific comparison for structs, maps, and pointers.
//
// If a and b are deeply equal, it returns nil.
func Diff[T any](a, b T) Patch[T] {
	// We take the address of a and b to ensure that if T is an interface,
	// reflect.ValueOf doesn't "peek through" to the concrete type immediately,
	// preserving the interface wrapper which is important for ApplyChecked.
	va := reflect.ValueOf(&a).Elem()
	vb := reflect.ValueOf(&b).Elem()

	patch, err := diffRecursive(va, vb, make(map[visitKey]bool))
	if err != nil {
		panic(err)
	}

	if patch == nil {
		return nil
	}

	return &typedPatch[T]{inner: patch}
}

type visitKey struct {
	a, b uintptr
	typ  reflect.Type
}

func diffRecursive(a, b reflect.Value, visited map[visitKey]bool) (diffPatch, error) {
	if !a.IsValid() && !b.IsValid() {
		return nil, nil
	}
	if !a.IsValid() || !b.IsValid() {
		if !b.IsValid() {
			return &valuePatch{oldVal: deepCopyValue(a), newVal: reflect.Value{}}, nil
		}
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}

	if a.Type() != b.Type() {
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}

	if a.CanInterface() && b.CanInterface() && reflect.DeepEqual(a.Interface(), b.Interface()) {
		return nil, nil
	}

	switch a.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.String:
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil

	case reflect.Ptr:
		return diffPtr(a, b, visited)
	case reflect.Interface:
		return diffInterface(a, b, visited)
	case reflect.Struct:
		return diffStruct(a, b, visited)
	case reflect.Slice:
		return diffSlice(a, b, visited)
	case reflect.Map:
		return diffMap(a, b, visited)
	case reflect.Array:
		return diffArray(a, b, visited)
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if a.IsNil() && b.IsNil() {
			return nil, nil
		}
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	default:
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}
}

func diffPtr(a, b reflect.Value, visited map[visitKey]bool) (diffPatch, error) {
	if a.IsNil() && b.IsNil() {
		return nil, nil
	}
	if a.IsNil() {
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}
	if b.IsNil() {
		return &valuePatch{oldVal: deepCopyValue(a), newVal: reflect.Zero(a.Type())}, nil
	}

	k := visitKey{a.Pointer(), b.Pointer(), a.Type()}
	if visited[k] {
		return nil, nil
	}
	visited[k] = true

	elemPatch, err := diffRecursive(a.Elem(), b.Elem(), visited)
	if err != nil {
		return nil, err
	}
	if elemPatch == nil {
		return nil, nil
	}

	return &ptrPatch{elemPatch: elemPatch}, nil
}

func diffInterface(a, b reflect.Value, visited map[visitKey]bool) (diffPatch, error) {
	if a.IsNil() && b.IsNil() {
		return nil, nil
	}
	if a.IsNil() || b.IsNil() {
		if !b.IsValid() {
			return &valuePatch{oldVal: deepCopyValue(a), newVal: reflect.Value{}}, nil
		}
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}

	if a.Elem().Type() != b.Elem().Type() {
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}

	elemPatch, err := diffRecursive(a.Elem(), b.Elem(), visited)
	if err != nil {
		return nil, err
	}
	if elemPatch == nil {
		return nil, nil
	}

	return &interfacePatch{elemPatch: elemPatch}, nil
}

func diffStruct(a, b reflect.Value, visited map[visitKey]bool) (diffPatch, error) {
	fields := make(map[string]diffPatch)

	for i := 0; i < a.NumField(); i++ {
		fA := a.Field(i)
		fB := b.Field(i)

		unsafe.DisableRO(&fA)
		unsafe.DisableRO(&fB)

		patch, err := diffRecursive(fA, fB, visited)
		if err != nil {
			return nil, err
		}
		if patch != nil {
			fieldName := a.Type().Field(i).Name
			fields[fieldName] = patch
		}
	}

	if len(fields) == 0 {
		return nil, nil
	}

	return &structPatch{fields: fields}, nil
}

func diffArray(a, b reflect.Value, visited map[visitKey]bool) (diffPatch, error) {
	indices := make(map[int]diffPatch)

	for i := 0; i < a.Len(); i++ {
		patch, err := diffRecursive(a.Index(i), b.Index(i), visited)
		if err != nil {
			return nil, err
		}
		if patch != nil {
			indices[i] = patch
		}
	}

	if len(indices) == 0 {
		return nil, nil
	}

	return &arrayPatch{indices: indices}, nil
}

func diffMap(a, b reflect.Value, visited map[visitKey]bool) (diffPatch, error) {
	if a.IsNil() && b.IsNil() {
		return nil, nil
	}
	if a.IsNil() || b.IsNil() {
		if !b.IsValid() {
			return &valuePatch{oldVal: deepCopyValue(a), newVal: reflect.Value{}}, nil
		}
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}

	added := make(map[interface{}]reflect.Value)
	removed := make(map[interface{}]reflect.Value)
	modified := make(map[interface{}]diffPatch)

	iterA := a.MapRange()
	for iterA.Next() {
		k := iterA.Key()
		vA := iterA.Value()

		vB := b.MapIndex(k)
		if !vB.IsValid() {
			removed[k.Interface()] = deepCopyValue(vA)
		} else {
			patch, err := diffRecursive(vA, vB, visited)
			if err != nil {
				return nil, err
			}
			if patch != nil {
				modified[k.Interface()] = patch
			}
		}
	}

	iterB := b.MapRange()
	for iterB.Next() {
		k := iterB.Key()
		vB := iterB.Value()

		vA := a.MapIndex(k)
		if !vA.IsValid() {
			added[k.Interface()] = deepCopyValue(vB)
		}
	}

	if len(added) == 0 && len(removed) == 0 && len(modified) == 0 {
		return nil, nil
	}

	return &mapPatch{
		added:    added,
		removed:  removed,
		modified: modified,
		keyType:  a.Type().Key(),
	}, nil
}

func diffSlice(a, b reflect.Value, visited map[visitKey]bool) (diffPatch, error) {
	if a.IsNil() && b.IsNil() {
		return nil, nil
	}
	if a.IsNil() || b.IsNil() {
		if !b.IsValid() {
			return &valuePatch{oldVal: deepCopyValue(a), newVal: reflect.Value{}}, nil
		}
		return &valuePatch{oldVal: deepCopyValue(a), newVal: deepCopyValue(b)}, nil
	}

	lenA := a.Len()
	lenB := b.Len()

	// 1. Identify common prefix
	prefix := 0
	for prefix < lenA && prefix < lenB {
		vA := a.Index(prefix)
		vB := b.Index(prefix)
		if reflect.DeepEqual(vA.Interface(), vB.Interface()) {
			prefix++
		} else {
			break
		}
	}

	// 2. Identify common suffix
	suffix := 0
	for suffix < (lenA-prefix) && suffix < (lenB-prefix) {
		vA := a.Index(lenA - 1 - suffix)
		vB := b.Index(lenB - 1 - suffix)
		if reflect.DeepEqual(vA.Interface(), vB.Interface()) {
			suffix++
		} else {
			break
		}
	}

	midAStart := prefix
	midAEnd := lenA - suffix
	midBStart := prefix
	midBEnd := lenB - suffix

	if midAStart >= midAEnd && midBStart >= midBEnd {
		return nil, nil
	}

	// 3. Diff the middle part
	ops := computeSliceEdits(a, b, midAStart, midAEnd, midBStart, midBEnd)

	return &slicePatch{ops: ops}, nil
}

// computeSliceEdits uses dynamic programming to find the shortest edit script
// for the middle portion of two slices.
func computeSliceEdits(a, b reflect.Value, aStart, aEnd, bStart, bEnd int) []sliceOp {
	n := aEnd - aStart
	m := bEnd - bStart

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 0; i <= n; i++ {
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			vA := a.Index(aStart + i - 1)
			vB := b.Index(bStart + j - 1)

			cost := 1
			if reflect.DeepEqual(vA.Interface(), vB.Interface()) {
				cost = 0
			}

			delCost := dp[i-1][j] + 1
			insCost := dp[i][j-1] + 1
			subCost := dp[i-1][j-1] + cost

			min := delCost
			if insCost < min {
				min = insCost
			}
			if subCost < min {
				min = subCost
			}
			dp[i][j] = min
		}
	}

	var ops []sliceOp
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 {
			vA := a.Index(aStart + i - 1)
			vB := b.Index(bStart + j - 1)

			cost := 1
			if reflect.DeepEqual(vA.Interface(), vB.Interface()) {
				cost = 0
			}

			if dp[i][j] == dp[i-1][j-1]+cost {
				if cost == 1 {
					p, _ := diffRecursive(vA, vB, make(map[visitKey]bool))
					ops = append(ops, sliceOp{
						Kind:  opMod,
						Index: aStart + i - 1,
						Patch: p,
					})
				}
				i--
				j--
				continue
			}
		}

		if i > 0 && dp[i][j] == dp[i-1][j]+1 {
			ops = append(ops, sliceOp{
				Kind:  opDel,
				Index: aStart + i - 1,
				Val:   deepCopyValue(a.Index(aStart + i - 1)),
			})
			i--
			continue
		}

		if j > 0 && dp[i][j] == dp[i][j-1]+1 {
			ops = append(ops, sliceOp{
				Kind:  opAdd,
				Index: aStart + i,
				Val:   deepCopyValue(b.Index(bStart + j - 1)),
			})
			j--
			continue
		}
	}

	for k := 0; k < len(ops)/2; k++ {
		ops[k], ops[len(ops)-1-k] = ops[len(ops)-1-k], ops[k]
	}

	return ops
}
