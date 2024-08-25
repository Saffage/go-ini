package ini

import (
	"bytes"
	"encoding"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
)

// Marshaler interface can be implemented to customize an INI tree
// while encoding.
//
// Note that it is not possible to implement Marshaler and
// [SectionMarshaler] for the same type simultaneously.
type Marshaler interface {
	MarshalINI() ([]Section, error)
}

// SectionMarshaler interface can be implemented to customize INI tree
// while encoding a section.
//
// Note that it is not possible to implement [Marshaler] and
// SectionMarshaler for the same type simultaneously.
type SectionMarshaler interface {
	MarshalINI() (Section, error)
}

// Marshal serializes the value provided into INI format.
//
// Marshal supports tags for structure fields, more information can be found
// in the [SectionsOf] function documentation.
//
// Example:
//
//	type VideoSettings struct {
//		Width      int  `ini:"width"`
//		Height     int  `ini:"height"`
//		FullScreen bool `ini:"fullscreen,omitempty,commented"`
//	}
//	type Settings struct {
//		Video VideoSettings
//	}
//	// [Video]
//	// width=0
//	// height=0
//	// ;fullscreen=
//	ini.Marshal(Settings{})
//	// [Video]
//	// width=1024
//	// height=768
//	// ;fullscreen=true
//	ini.Marshal(Settings{
//		Video: VideoSettings{
//			Width:      1024,
//			Height:     768,
//			FullScreen: true,
//		},
//	})
func Marshal(value any) ([]byte, error) {
	buf := bytes.Buffer{}
	e := Encoder{}
	e.Reset(&buf)
	if err := e.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Encoder writes an INI tree to the specified output.
type Encoder struct {
	w io.Writer

	skipFieldEncodeFailure bool
}

// NewEncoder creates a new [Encoder] that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// SkipFieldEncodeFailure allows the encoder to ignore an encoding error
// and not write a field that failed to encode.
func (e *Encoder) SkipFieldEncodeFailure(flag bool) *Encoder {
	e.skipFieldEncodeFailure = flag
	return e
}

// Reset resets the encoder to write to w, keeping all of its settings.
func (e *Encoder) Reset(w io.Writer) *Encoder {
	e.w = w
	return e
}

// Encode serializes the value provided into INI format.
//
// More information can be found in the [Marshal] function documentation.
func (e *Encoder) Encode(data any) error {
	sections, err := SectionsOf(data)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	for _, section := range sections {
		if err := e.section(&buf, section); err != nil {
			return err
		}
	}

	_, err = e.w.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}

func (e *Encoder) section(buf *bytes.Buffer, section Section) error {
	buf.WriteByte('[')
	buf.WriteString(section.Name)
	buf.WriteByte(']')
	buf.WriteByte('\n')

	for _, field := range section.Fields {
		if err := e.field(buf, field); err != nil && !e.skipFieldEncodeFailure {
			return err
		}
	}

	return nil
}

func (e *Encoder) field(buf *bytes.Buffer, field Field) error {
	b, err := field.MarshalText()
	if err != nil {
		return err
	}

	if field.Commented {
		buf.WriteByte(';')
	}

	if b != nil || field.Commented {
		buf.WriteString(field.Name)
		buf.WriteByte('=')
		buf.Write(b)
		buf.WriteByte('\n')
	}

	return nil
}

var (
	tMarshaler        = reflect.TypeFor[Marshaler]()
	tSectionMarshaler = reflect.TypeFor[SectionMarshaler]()
	tTextMarshaler    = reflect.TypeFor[encoding.TextMarshaler]()
)

func encode(v reflect.Value) ([]byte, error) {
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	t := v.Type()

	if t.Implements(tTextMarshaler) {
		b, err := v.Interface().(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return nil, err
		}
		return b, err
	}

	if v.CanAddr() && reflect.PointerTo(t).Implements(tMarshaler) {
		b, err := v.Addr().Interface().(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return nil, err
		}
		return b, err
	}

	switch v.Kind() {
	case reflect.Bool:
		encoded := strconv.FormatBool(v.Bool())
		return []byte(encoded), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		encoded := strconv.FormatInt(v.Int(), 10)
		return []byte(encoded), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		encoded := strconv.FormatUint(v.Uint(), 10)
		return []byte(encoded), nil

	case reflect.Float32, reflect.Float64:
		value, encoded := v.Float(), ""
		if math.IsNaN(value) {
			encoded = "nan"
		} else if math.IsInf(value, 1) {
			encoded = "inf"
		} else if math.IsInf(value, -1) {
			encoded = "-inf"
		} else {
			encoded = strconv.FormatFloat(value, 'f', -1, 64)
		}
		return []byte(encoded), nil

	case reflect.Array, reflect.Slice:
		buf := []byte{}
		for i := range v.Len() {
			if i > 0 {
				buf = append(buf, ',')
			}
			b, err := encode(v.Index(i))
			if err != nil {
				return nil, err
			}
			buf = append(buf, b...)
		}
		return buf, nil

	case reflect.String:
		encoded := quoteString(v.String())
		return []byte(encoded), nil

	default:
		return nil, fmt.Errorf(
			"invalid type %s for encode operation",
			v.Type().String(),
		)
	}
}
