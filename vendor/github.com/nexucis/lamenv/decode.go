package lamenv

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	omitempty = "omitempty"
	squash    = "squash"
	inline    = "inline"
)

var durationType = reflect.TypeOf(time.Duration(0))

func (l *Lamenv) decode(conf reflect.Value, parts []string) error {
	v := conf
	// ptr will be used to try if the value is implementing the interface Unmarshaler.
	// if it's the case then, the implementation of the interface has the priority.
	ptr := conf
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			// if the pointer is not initialized, then accessing to its element will return `reflect.invalid`
			// So we have to create a new instance of the pointer first
			v.Set(reflect.New(v.Type().Elem()))
			ptr = v
		}
		v = v.Elem()
	} else {
		ptr = reflect.New(v.Type())
		ptr.Elem().Set(v)
	}

	if p, ok := ptr.Interface().(Unmarshaler); ok {
		if err := p.UnmarshalEnv(parts); err != nil {
			return err
		}
		// in case the method UnmarshalEnv() is setting some parameter in the struct, we have to save these changes
		v.Set(ptr.Elem())
		return nil
	}

	if p, ok := ptr.Interface().(encoding.TextUnmarshaler); ok {
		if variable, input, exist := lookupEnv(parts); exist {
			// remove the variable to avoid reusing it later
			delete(l.env, variable)
			if err := p.UnmarshalText([]byte(input)); err != nil {
				return err
			}

			// in case the method UnmarshalEnv() is setting some parameter in the struct, we have to save these changes
			v.Set(ptr.Elem())
		}
		return nil
	}

	switch v.Kind() {
	case reflect.Map:
		if err := l.decodeMap(v, parts); err != nil {
			return err
		}
	case reflect.Slice:
		if err := l.decodeSlice(v, parts); err != nil {
			return err
		}
	case reflect.Struct:
		if err := l.decodeStruct(v, parts); err != nil {
			return err
		}
	default:
		if variable, input, exist := lookupEnv(parts); exist {
			// remove the variable to avoid to reuse it later
			delete(l.env, variable)
			return l.decodeNative(v, input)
		}
	}
	return nil
}

func (l *Lamenv) decodeNative(v reflect.Value, input string) error {
	switch v.Kind() {
	case reflect.String:
		l.decodeString(v, input)
	case reflect.Bool:
		if err := l.decodeBool(v, input); err != nil {
			return err
		}
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		if err := l.decodeInt(v, input); err != nil {
			return err
		}
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		if err := l.decodeUInt(v, input); err != nil {
			return err
		}
	case reflect.Float32,
		reflect.Float64:
		if err := l.decodeFloat(v, input); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lamenv) decodeString(v reflect.Value, input string) {
	v.SetString(input)
}

func (l *Lamenv) decodeBool(v reflect.Value, input string) error {
	b, err := strconv.ParseBool(strings.TrimSpace(input))
	if err != nil {
		return err
	}
	v.SetBool(b)
	return nil
}

func (l *Lamenv) decodeInt(v reflect.Value, input string) error {
	if v.Type() == durationType {
		i, err := time.ParseDuration(strings.TrimSpace(input))
		if err != nil {
			return err
		}
		v.SetInt(int64(i))
	} else {
		i, err := strconv.ParseInt(strings.TrimSpace(input), 10, 0)
		if err != nil {
			return err
		}
		v.SetInt(i)
	}

	return nil
}

func (l *Lamenv) decodeUInt(v reflect.Value, input string) error {
	i, err := strconv.ParseUint(strings.TrimSpace(input), 10, 0)
	if err != nil {
		return err
	}
	v.SetUint(i)
	return nil
}

func (l *Lamenv) decodeFloat(v reflect.Value, input string) error {
	i, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
	if err != nil {
		return err
	}
	v.SetFloat(i)
	return nil
}

// decodeSlice will support ony one syntax which is:
//
//	<PREFIX>_<SLICE_INDEX>(_<SUFFIX>)?
//
// This syntax is the only one that is able to manage smoothly every existing type in Golang and it is a determinist syntax.
func (l *Lamenv) decodeSlice(v reflect.Value, parts []string) error {
	sliceType := v.Type().Elem()
	// While we are able to find an environment variable that is starting by <PREFIX>_<SLICE_INDEX>
	// then it will create a new item in a slice and will use the next recursive loop to set it.
	i := 0
	for ok := contains(append(parts, strconv.Itoa(i))); ok; ok = contains(append(parts, strconv.Itoa(i))) {
		var sliceElem reflect.Value
		if i < v.Len() {
			// that means there is already an element in the slice and should just complete or override the value
			sliceElem = v.Index(i)
		} else {
			// in that case we have to create a new element
			sliceElem = reflect.Indirect(reflect.New(sliceType))
		}
		if err := l.decode(sliceElem, append(parts, strconv.Itoa(i))); err != nil {
			return err
		}

		if i >= v.Len() {
			// in case we have created a new element, then we need to add it to the slice
			v.Set(reflect.Append(v, sliceElem))
		}
		i++
	}
	return nil
}

func (l *Lamenv) decodeStruct(v reflect.Value, parts []string) error {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := v.Type().Field(i)
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
				if err := l.decode(field, parts); err != nil {
					return err
				}
				continue
			}
			if containStr(tags, omitempty) {
				// Here we only have to check if there is one environment variable that is starting by the current parts
				// It's not necessary accurate if you have one field that is a prefix of another field.
				// But it's not really a big deal since it will just loop another time for nothing and could eventually initialize the field. But this case will not occur so often.
				// To be more accurate, we would have to check the type of the field, because if it's a native type, then we will have to check if the parts are matching an environment variable.
				// If it's a struct or an array or a map, then we will have to check if there is at least one variable starting by the parts + "_" (which would remove the possibility of having a field being a prefix of another one)
				// So it's simpler like that. Let's see if I'm wrong or not.
				if !contains(append(parts, fieldName)) {
					continue
				}
			}
		} else {
			fieldName = fieldType.Name
		}
		if err := l.decode(field, append(parts, fieldName)); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lamenv) decodeMap(v reflect.Value, parts []string) error {
	keyType := v.Type().Key()
	valueType := v.Type().Elem()
	if keyType.Kind() != reflect.String {
		return fmt.Errorf("unable to unmarshal a map with a key that is not a string")
	}
	if valueType.Kind() == reflect.Map {
		return fmt.Errorf("unable to unmarshal a map of a map, it's not a determinist datamodel")
	}
	valMap := v
	if v.IsNil() {
		mapType := reflect.MapOf(keyType, valueType)
		valMap = reflect.MakeMap(mapType)
	}
	// The main issue with the map when you are dealing with environment variable is to be able to find the key of the map
	// A way to achieve it is to take a look at the type of the value of the map.
	// It will be used to find every potential future parts, which will be then used as a variable suffix.
	// Like that we are able catch the key that would be in the middle of the prefix parts and the future parts

	// Let's create first the struct that would represent what is behind the value of the map
	parser := newRing(valueType, l.tagSupports)

	// then foreach environment variable:
	// 1. Remove the prefix parts
	// 2. Pass the remaining parts to the parser that would return the prefix to be used.
	for e := range l.env {
		variable := buildEnvVariable(parts)
		trimEnv := strings.TrimPrefix(e, variable+"_")
		if trimEnv == e {
			// TrimPrefix didn't remove anything, so that means, the environment variable doesn't start with the prefix parts
			continue
		}
		futureParts := strings.Split(trimEnv, "_")
		prefix, err := guessPrefix(futureParts, parser)
		if err != nil {
			return err
		}
		if len(prefix) == 0 {
			// no prefix find, let's move to the next environment
			continue
		}
		keyString := strings.ToLower(prefix)
		value := reflect.Indirect(reflect.New(valueType))
		if err := l.decode(value, append(parts, keyString)); err != nil {
			return err
		}
		key := reflect.Indirect(reflect.New(reflect.TypeOf("")))
		key.SetString(strings.TrimSpace(strings.ToLower(keyString)))
		valMap.SetMapIndex(key, value)
	}
	// Set the built up map to the value
	v.Set(valMap)
	return nil
}
