package lamenv

import (
	"encoding"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"
)

func (l *Lamenv) encode(value reflect.Value, parts []string) error {
	v := value
	// ptr will be used to try if the value is implementing the interface Marshaler.
	// if it's the case then, the implementation of the interface has the priority.
	ptr := value
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	} else {
		ptr = reflect.New(v.Type())
		ptr.Elem().Set(v)
	}

	if p, ok := ptr.Interface().(Marshaler); ok {
		return p.MarshalEnv(parts)
	}

	if p, ok := ptr.Interface().(encoding.TextMarshaler); ok {
		raw, err := p.MarshalText()
		if err != nil {
			return err
		}
		return os.Setenv(buildEnvVariable(parts), string(raw))
	}

	switch v.Kind() {
	case reflect.Map:
		if err := l.encodeMap(v, parts); err != nil {
			return err
		}
	case reflect.Slice:
		if err := l.encodeSlice(v, parts); err != nil {
			return err
		}
	case reflect.Struct:
		if err := l.encodeStruct(v, parts); err != nil {
			return err
		}
	default:
		return l.encodeNative(v, buildEnvVariable(parts))
	}
	return nil
}

func (l *Lamenv) encodeNative(value reflect.Value, input string) error {
	return os.Setenv(input, nativeToString(value))
}

func (l *Lamenv) encodeSlice(value reflect.Value, parts []string) error {
	if value.IsNil() {
		return nil
	}
	for i := 0; i < value.Len(); i++ {
		if err := l.encode(value.Index(i), append(parts, strconv.Itoa(i))); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lamenv) encodeMap(value reflect.Value, parts []string) error {
	if value.IsNil() {
		return nil
	}
	iter := value.MapRange()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		if err := l.encode(v, append(parts, nativeToString(k))); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lamenv) encodeStruct(value reflect.Value, parts []string) error {
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := value.Type().Field(i)
		if len(fieldType.PkgPath) > 0 {
			// the field is not exported, so no need to look at it as we won't be able to set it in a later stage
			continue
		}
		var fieldName string
		tags, ok := l.lookupTag(fieldType.Tag)
		if ok {
			fieldName = tags[0]
			tags = tags[1:]
			if fieldName == "-" {
				continue
			}
			if containStr(tags, squash) || containStr(tags, inline) {
				if err := l.encode(field, parts); err != nil {
					return err
				}
				continue
			}
			if containStr(tags, omitempty) && isZero(field) {
				continue
			}
		} else {
			fieldName = fieldType.Name
		}

		if err := l.encode(value.Field(i), append(parts, fieldName)); err != nil {
			return err
		}
	}
	return nil
}

func isZero(v reflect.Value) bool {
	kind := v.Kind()
	switch kind {
	case reflect.String:
		return len(v.String()) == 0
	case reflect.Interface,
		reflect.Ptr:
		return v.IsNil()
	case reflect.Slice:
		return v.Len() == 0
	case reflect.Map:
		return v.Len() == 0
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return v.Int() == 0
	case reflect.Float32,
		reflect.Float64:
		return v.Float() == 0
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Struct:
		vt := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if len(vt.Field(i).PkgPath) > 0 {
				continue // Private field
			}
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	}
	return false
}

func nativeToString(value reflect.Value) string {
	switch value.Kind() {
	case reflect.String:
		return value.String()
	case reflect.Bool:
		return fmt.Sprintf("%t", value.Bool())
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		if value.Type() == durationType {
			return value.Interface().(time.Duration).String()
		}
		return fmt.Sprintf("%d", value.Int())
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return fmt.Sprintf("%d", value.Uint())
	case reflect.Float32,
		reflect.Float64:
		return fmt.Sprintf("%f", value.Float())
	}
	return ""
}
