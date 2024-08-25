package ini

import (
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"slices"
)

// Field represents a key-value pair in the INI tree.
type Field struct {
	Name      string
	Value     reflect.Value // TODO: replace [reflect.Value] with any type.
	OmitEmpty bool
	Commented bool

	// TODO: add optional documentation for fields to emit it in the file.
}

func (f *Field) MarshalText() ([]byte, error) {
	if f.Value.IsValid() {
		if !f.OmitEmpty || !f.Value.IsZero() {
			return encode(f.Value)
		}
		return nil, nil
	}
	return nil, errors.New("field have invalid value")
}

func (f *Field) UnmarshalText(text []byte) error {
	if f.Value.IsValid() {
		return decode(string(text), f.Value)
	}
	return errors.New("field have invalid value")
}

// Section represents a table in the INI tree.
type Section struct {
	Name      string
	Fields    []Field
	OmitEmpty bool
}

// Field looks for a name in the section.
func (s *Section) Field(name string) (Field, bool) {
	for _, field := range s.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return Field{}, false
}

func (s *Section) MarshalINI() (Section, error) {
	return *s, nil
}

func (s *Section) UnmarshalINI(section Section) error {
	*s = section
	return nil
}

// SectionsOf builds an INI tree based on the value provided.
// Only a certain set of data types can be used for the value,
// as the INI format is very limited.
//
// # Allowed types
//
// Value must be one of:
//   - struct{ S... }
//   - map[string]S
//   - [Marshaler]
//
// S must be one of:
//   - struct{ F... }
//   - map[string]F
//   - [SectionMarshaler]
//
// F must be one of:
//   - int* \ uint*
//   - float*
//   - bool
//   - string
//   - []F \ [N]F
//   - [encoding.TextMarshaler]
//
// # Struct tags
//
// This package supports tags for structure fields, for example:
//
//	`ini:"[key]{,flag}"`
//
// Unexported fields or fields with the key "-" are ignored. Every flag must
// be prefixed with a comma.
//
// The following flags are currently supported:
//
//   - inline – inline all fields\values into the current section.
//     The type of this field must be S.
//
//   - omitempty – skip the field if it has a zero value.
//
//   - commented – prefix the field while encoding.
func SectionsOf(value any) ([]Section, error) {
	v := reflect.Indirect(reflect.ValueOf(value))
	t := v.Type()

	if t.Implements(tMarshaler) {
		return v.Interface().(Marshaler).MarshalINI()
	}

	if v.CanAddr() && reflect.PointerTo(t).Implements(tMarshaler) {
		return v.Addr().Interface().(Marshaler).MarshalINI()
	}

	if t.Kind() == reflect.Map {
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("cannot use type %s as map key", t.String())
		}
		return sectionsOfMap(v)
	}

	if t.Kind() == reflect.Struct {
		return sectionsOfStruct(v)
	}

	return nil, fmt.Errorf(
		"invalid type %s for encode\\decode operation",
		v.Type().String(),
	)
}

func sectionsOfMap(root reflect.Value) ([]Section, error) {
	return walkMap(root, func(v reflect.Value, flags flags) (Section, error) {
		flags.inline = true
		fields, err := fieldsOf(v, nil, reflect.StructField{}, flags)
		if err != nil {
			return Section{}, err
		}
		return Section{
			Name:      flags.key,
			Fields:    fields,
			OmitEmpty: flags.omitempty,
		}, nil
	})
}

func sectionsOfStruct(root reflect.Value) ([]Section, error) {
	return walkStructFields(
		root,
		func(v reflect.Value, f reflect.StructField, flags flags) (Section, error) {
			flags.inline = true
			fields, err := fieldsOf(v, root.Type(), f, flags)
			if err != nil {
				return Section{}, err
			}
			return Section{
				Name:      flags.key,
				Fields:    fields,
				OmitEmpty: flags.omitempty,
			}, nil
		},
	)
}

func fieldsOf(
	v reflect.Value,
	structType reflect.Type,
	field reflect.StructField,
	flags flags,
) ([]Field, error) {
	t := v.Type()

	if t.Implements(tSectionMarshaler) {
		section, err := v.Interface().(SectionMarshaler).MarshalINI()
		if err != nil {
			return nil, err
		}
		return section.Fields, nil
	}

	if v.CanAddr() && reflect.PointerTo(t).Implements(tSectionMarshaler) {
		section, err := v.Addr().Interface().(SectionMarshaler).MarshalINI()
		if err != nil {
			return nil, err
		}
		return section.Fields, nil
	}

	if t.Kind() == reflect.Map && flags.inline {
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key type must be string")
		}
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key type must be string")
		}
		return fieldsOfMap(v)
	}

	if t.Kind() == reflect.Struct && flags.inline {
		return fieldsOfStruct(v)
	}

	if isBasicType(t) {
		return []Field{
			{
				Name:      flags.key,
				Value:     v,
				OmitEmpty: flags.omitempty,
				Commented: flags.commented,
			},
		}, nil
	}

	if field.Type != nil {
		return nil, fmt.Errorf(
			"type of field '%s' in type '%s' must be bool, int, float, string, "+
				"array\\slice, or struct\\map[string]T with 'inline' tag",
			structType.String(),
			field.Name,
		)
	}

	return nil, fmt.Errorf(
		"cannot use type %s for encode\\decode operation",
		t.String(),
	)
}

func fieldsOfMap(section reflect.Value) ([]Field, error) {
	return walkMap(section, func(v reflect.Value, flags flags) (Field, error) {
		return Field{
			Name:      flags.key,
			Value:     v,
			OmitEmpty: flags.omitempty,
			Commented: flags.commented,
		}, nil
	})
}

func fieldsOfStruct(section reflect.Value) ([]Field, error) {
	fields, err := walkStructFields(
		section,
		func(v reflect.Value, f reflect.StructField, flags flags) ([]Field, error) {
			fields, err := fieldsOf(v, section.Type(), f, flags)
			if err != nil {
				return nil, err
			}
			return fields, nil
		},
	)
	return slices.Concat(fields...), err
}

func isBasicType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Array, reflect.Slice, reflect.String,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int, reflect.Uint, reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func isZeroOrEmpty(v reflect.Value) bool {
	return v.IsZero() ||
		(v.Kind() == reflect.Slice || v.Kind() == reflect.Map) && v.Len() == 0
}

func quoteString(s string) string {
	buf := make([]byte, 0, 2+len(s)+len(s)/2)
	buf = append(buf, '\'')
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7E:
			buf = append(buf, byte(r))

		case r == '\n':
			buf = append(buf, "\\n"...)

		case r == '\r':
			buf = append(buf, "\\r"...)

		default:
			encoded := hex.EncodeToString([]byte(string(r)))
			for i := 0; i < len(encoded); i += 2 {
				buf = append(buf, "\\x"...)
				buf = append(buf, encoded[i])
				buf = append(buf, encoded[i+1])
			}
		}
	}
	buf = append(buf, '\'')
	return string(buf)
}

type walkStructFunc[T any] func(v reflect.Value, f reflect.StructField, flags flags) (T, error)

type walkMapFunc[T any] func(v reflect.Value, flags flags) (T, error)

func walkStructFields[T any](v reflect.Value, f walkStructFunc[T]) ([]T, error) {
	vals := make([]T, 0, v.NumField())
	errs := make([]error, 0)

	for i := range v.NumField() {
		field := v.Type().Field(i)

		if !field.IsExported() {
			continue
		}

		flags, err := parseTag(v.Type(), field)

		if err != nil {
			errs = append(errs, err)
			continue
		}

		if flags.key == "-" {
			continue
		}

		if flags.key == "" {
			flags.key = field.Name
		}

		fieldValue := v.Field(i)

		switch fieldValue.Kind() {
		case reflect.Pointer:
			flags.omitempty = true
			fallthrough

		case reflect.Interface:
			fieldValue = fieldValue.Elem()
		}

		val, err := f(fieldValue, field, flags)

		if err != nil {
			errs = append(errs, err)
		} else {
			vals = append(vals, val)
		}
	}

	return vals, errors.Join(errs...)
}

func walkMap[T any](v reflect.Value, f walkMapFunc[T]) ([]T, error) {
	vals := make([]T, 0, v.Len())
	errs := make([]error, 0)

	for iter := v.MapRange(); iter.Next(); {
		omitempty := false
		k, v := iter.Key().Interface().(string), iter.Value()

		switch v.Kind() {
		case reflect.Pointer:
			omitempty = true
			fallthrough

		case reflect.Interface:
			v = v.Elem()
		}

		val, err := f(v, flags{key: k, omitempty: omitempty})

		if err != nil {
			errs = append(errs, err)
		} else {
			vals = append(vals, val)
		}
	}

	return vals, errors.Join(errs...)
}
