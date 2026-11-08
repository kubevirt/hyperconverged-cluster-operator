package deep

import (
	"fmt"
	"reflect"

	"github.com/brunoga/deep/internal/unsafe"
)

// Copier is an interface that types can implement to provide their own
// custom deep copy logic. The type T in Copy() (T, error) must be the
// same concrete type as the receiver that implements this interface.
type Copier[T any] interface {
	Copy() (T, error)
}

// Copy creates a deep copy of src. It returns the copy and a nil error in case
// of success and the zero value for the type and a non-nil error on failure.
func Copy[T any](src T) (T, error) {
	return copyInternal(src, false)
}

// CopySkipUnsupported creates a deep copy of src. It returns the copy and a nil
// error in case of success and the zero value for the type and a non-nil error
// on failure. Unsupported types are skipped (the copy will have the zero value
// for the type) instead of returning an error.
func CopySkipUnsupported[T any](src T) (T, error) {
	return copyInternal(src, true)
}

// MustCopy creates a deep copy of src. It returns the copy on success or panics
// in case of any failure.
func MustCopy[T any](src T) T {
	dst, err := copyInternal(src, false)
	if err != nil {
		panic(err)
	}

	return dst
}

type pointersMapKey struct {
	ptr uintptr
	typ reflect.Type
}
type pointersMap map[pointersMapKey]reflect.Value

func copyInternal[T any](src T, skipUnsupported bool) (T, error) {
	v := reflect.ValueOf(src)

	// If src is the zero value for its type (e.g. an uninitialized interface,
	// or if T is 'any' and src is its zero value), v will be invalid.
	if !v.IsValid() {
		// This amounts to returning the zero value for T.
		var t T
		return t, nil
	}

	// Attempt to use Copier interface if src is suitable:
	// - A value type (struct, int, etc.)
	// - A non-nil pointer type
	// - A non-nil interface type
	// This logic avoids trying to call Copy() on a nil receiver if T itself
	// is a pointer or interface type that is nil.
	attemptCopier := false
	srcKind := v.Kind()
	if srcKind != reflect.Interface && srcKind != reflect.Ptr {
		attemptCopier = true
	} else {
		// Pointers or interface types are candidates only if they are not nil
		if !v.IsNil() {
			attemptCopier = true
		}
	}

	if attemptCopier {
		srcType := v.Type()

		// If T is an interface or pointer type, converting src to 'any' is generally
		// non-allocating for src's underlying data.
		if srcKind == reflect.Interface || srcKind == reflect.Ptr {
			if copier, ok := any(src).(Copier[T]); ok {
				return copier.Copy()
			}
		} else {
			// T is a value type (e.g. struct, array, basic type).
			// The any(src) conversion might allocate.
			// Check Implements first to avoid this allocation if T doesn't implement Copier[T].
			copierInterfaceType := reflect.TypeOf((*Copier[T])(nil)).Elem()
			if srcType.Implements(copierInterfaceType) {
				// T implements Copier[T]. Now the type assertion (and potential allocation)
				// is justified as we expect to call the custom method.
				if copier, ok := any(src).(Copier[T]); ok {
					return copier.Copy()
				}
			}
		}
	}

	dst, err := recursiveCopy(v, make(pointersMap),
		skipUnsupported)
	if err != nil {
		var t T
		return t, err
	}

	return dst.Interface().(T), nil
}

func recursiveCopy(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.String:
		// Direct type, just copy it.
		return v, nil
	case reflect.Array:
		return recursiveCopyArray(v, pointers, skipUnsupported)
	case reflect.Interface:
		return recursiveCopyInterface(v, pointers, skipUnsupported)
	case reflect.Map:
		return recursiveCopyMap(v, pointers, skipUnsupported)
	case reflect.Ptr:
		return recursiveCopyPtr(v, pointers, skipUnsupported)
	case reflect.Slice:
		return recursiveCopySlice(v, pointers, skipUnsupported)
	case reflect.Struct:
		return recursiveCopyStruct(v, pointers, skipUnsupported)
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if v.IsNil() {
			// If we have a nil function, unsafe pointer or channel, then we
			// can copy it.
			return v, nil
		} else {
			if skipUnsupported {
				return reflect.Zero(v.Type()), nil
			} else {
				return reflect.Value{}, fmt.Errorf("unsuported non-nil value for type: %s", v.Type())
			}
		}
	default:
		if skipUnsupported {
			return reflect.Zero(v.Type()), nil
		} else {
			return reflect.Value{}, fmt.Errorf("unsuported type: %s", v.Type())
		}
	}
}

func recursiveCopyArray(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	dst := reflect.New(v.Type()).Elem()

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		elemDst, err := recursiveCopy(elem, pointers, skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dst.Index(i).Set(elemDst)
	}

	return dst, nil
}

func recursiveCopyInterface(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	if v.IsNil() {
		// If the interface is nil, just return it.
		return v, nil
	}

	return recursiveCopy(v.Elem(), pointers, skipUnsupported)
}

func recursiveCopyMap(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	if v.IsNil() {
		// If the slice is nil, just return it.
		return v, nil
	}

	dst := reflect.MakeMap(v.Type())

	for _, key := range v.MapKeys() {
		elem := v.MapIndex(key)
		elemDst, err := recursiveCopy(elem, pointers,
			skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dst.SetMapIndex(key, elemDst)
	}

	return dst, nil
}

func recursiveCopyPtr(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	// If the pointer is nil, just return it.
	if v.IsNil() {
		return v, nil
	}

	ptr := v.Pointer()
	typ := v.Type()
	key := pointersMapKey{ptr, typ}

	// If the pointer is already in the pointers map, return it.
	if dst, ok := pointers[key]; ok {
		return dst, nil
	}

	// Otherwise, create a new pointer and add it to the pointers map.
	dst := reflect.New(v.Type().Elem())

	pointers[key] = dst

	// Proceed with the copy.
	elem := v.Elem()
	elemDst, err := recursiveCopy(elem, pointers, skipUnsupported)
	if err != nil {
		return reflect.Value{}, err
	}

	dst.Elem().Set(elemDst)

	return dst, nil
}

func recursiveCopySlice(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	if v.IsNil() {
		// If the slice is nil, just return it.
		return v, nil
	}

	dst := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		elemDst, err := recursiveCopy(elem, pointers,
			skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dst.Index(i).Set(elemDst)
	}

	return dst, nil
}

func recursiveCopyStruct(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	dst := reflect.New(v.Type()).Elem()

	for i := 0; i < v.NumField(); i++ {
		elem := v.Field(i)

		// If the field is unexported, we need to disable read-only mode. If it
		// is exported, doing this changes nothing so we just do it. We need to
		// do this here not because we are writting to the field (this is the
		// source), but because Interface() does not work if the read-only bits
		// are set.
		unsafe.DisableRO(&elem)

		elemDst, err := recursiveCopy(elem, pointers,
			skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dstField := dst.Field(i)

		// If the field is unexported, we need to disable read-only mode so we
		// can actually write to it.
		unsafe.DisableRO(&dstField)

		dstField.Set(elemDst)
	}

	return dst, nil
}

func deepCopyValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return reflect.Value{}
	}
	copied, err := recursiveCopy(v, make(pointersMap), false)
	if err != nil {
		return v
	}
	return copied
}
