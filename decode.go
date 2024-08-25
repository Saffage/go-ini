package ini

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Unmarshaler interface can be implemented to customize an INI tree
// while decoding.
//
// Note that it is not possible to implement Unmarshaler and
// [SectionUnmarshaler] for the same type simultaneously.
type Unmarshaler interface {
	UnmarshalINI([]Section) error
}

// Marshaler interface can be implemented to customize an INI tree
// while decoding a section.
//
// Note that it is not possible to implement [Unmarshaler] and
// SectionUnmarshaler for the same type simultaneously.
type SectionUnmarshaler interface {
	UnmarshalINI(Section) error
}

// Unmarshal deserializes an INI file into a Go value.
//
// Unmarshal supports tags for structure fields, more information can be found
// in the [SectionsOf] function documentation.
func Unmarshal(data []byte, value any) error {
	r := bytes.NewReader(data)
	d := Decoder{}
	d.Reset(r)
	return d.Decode(value)
}

// Decoder reads and decodes an INI file from the specified input.
type Decoder struct {
	r io.Reader
	scanner
}

// Reset resets the decoder to read from w, keeping all of its settings.
func (d *Decoder) Reset(r io.Reader) *Decoder {
	d.r = r
	return d
}

// Decode deserializes an INI file into a Go value.
//
// More information can be found in the [Unmarshal] function documentation.
func (d *Decoder) Decode(value any) error {
	sections, err := SectionsOf(value)
	if err != nil {
		return err
	}

	b, err := io.ReadAll(d.r)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	d.init(b)
	return d.scan(sections)
}

func (d *Decoder) scan(sections []Section) error {
	currentSection := (*Section)(nil)

	for char := d.peek(); ; char = d.peek() {
		d.skipSpaces()

		switch {
		case char == '\000':
			return nil

		case isNewlineChar(char):
			continue

		case isNameChar(char):
			fieldName := d.name()

			if !d.consume('=') {
				return errUnexpectedChar(d.peek(), d.lineNum, d.charNum)
			}

			value, err := d.value()
			if err != nil {
				return err
			}

			if currentSection == nil {
				return errors.New("key must be under section")
			}

			if field, present := currentSection.Field(fieldName); present {
				err := decode(strings.TrimSpace(value), field.Value)
				if err != nil {
					return err
				}
			}

		case char == '[':
			d.advance()
			sectionName := d.name()

			if sectionName == "" || !d.consume(']') {
				return errUnexpectedChar(d.peek(), d.lineNum, d.charNum)
			}

			if currentSection = findSection(
				sections,
				sectionName,
			); currentSection == nil {
				return fmt.Errorf("unknown section named '%s'", sectionName)
			}

		case char == '#', char == ';':
			d.takeUntil(isNewlineChar)

		default:
			return errUnexpectedChar(char, d.lineNum, d.charNum)
		}

		if !d.handleNewline() {
			return errExpectedNewLine(int(d.lineNum), int(d.charNum))
		}
	}
}

func (d *Decoder) skipSpaces() {
	d.takeWhile(func(char byte) bool { return char == ' ' })
}

func (d *Decoder) name() string {
	d.skipSpaces()
	name := d.takeWhile(isNameCharOrDigit)
	d.skipSpaces()
	return name
}

func (d *Decoder) value() (string, error) {
	d.skipSpaces()

	value := strings.Builder{}

	switch {
	case d.peek() == '\'':
		d.advance()
		s, err := d.take(d.takeStringChar)
		if err != nil {
			return "", err
		}
		value.WriteString(s)
		d.advance()

	case d.peek() == '"':
		d.advance()
		value.WriteString(d.takeUntil(func(char byte) bool {
			return char == '"'
		}))
		d.advance()

	case isDigit(d.peek()):
		value.WriteString(d.takeWhile(isDigit))
		if d.consume('.') {
			value.WriteByte('.')
			value.WriteString(d.takeWhile(isDigit))
		}

	case isNameChar(d.peek()):
		value.WriteString(d.takeWhile(isNameCharOrDigit))

	case isNewlineChar(d.peek()):
		// Empty value.

	default:
		return "", errUnexpectedChar(d.peek(), d.lineNum, d.charNum)
	}

	d.skipSpaces()

	if d.consume(',') {
		v, err := d.value()
		if err != nil {

		}
		value.WriteByte(',')
		value.WriteString(v)
	}

	if d.consume(';') {
		d.takeUntil(isNewlineChar)
	}

	return value.String(), nil
}

func (d *Decoder) takeStringChar() ([]byte, error) {
	char := d.peek()

	if char == '\000' || char == '\'' || isNewlineChar(char) {
		return nil, stop
	}

	if !d.consume('\\') {
		return []byte{d.advance()}, nil
	}

	switch d.advance() {
	case '\r':
		d.consume('\n')
		fallthrough

	case '\n':
		fallthrough

	case 'n':
		return []byte{'\n'}, nil

	case 'r':
		return []byte{'\r'}, nil

	case 't':
		return []byte{'\t'}, nil

	case '\\':
		return []byte{'\\'}, nil

	case '\'':
		return []byte{'\''}, nil

	case '"':
		return []byte{'"'}, nil

	case 'x':
		bytes := [2]byte{}
		decoded := [1]byte{}
		if !isHexDigit(d.peek()) || !isHexDigit(d.lookAhead(1)) {
			panic("TODO")
		}
		bytes[0] = d.advance()
		bytes[1] = d.advance()
		_, err := hex.Decode(decoded[:], bytes[:])
		if err != nil {
			panic(err)
		}
		return decoded[:], nil

	default:
		return nil, errors.New("invalid escape sequence")
	}
}

func findSection(sections []Section, name string) *Section {
	idx := slices.IndexFunc(sections, func(section Section) bool {
		return section.Name == name
	})
	if idx >= 0 {
		return &sections[idx]
	}
	return nil
}

func decode(str string, v reflect.Value) error {
	if !v.IsValid() || len(str) == 0 {
		return nil
	}

	if !v.CanSet() {
		return errors.New("value cannot be set")
	}

	switch v.Kind() {
	case reflect.Bool:
		x, err := strconv.ParseBool(str)
		if err != nil {
			return fmt.Errorf("parsing failed: %w", err)
		}
		v.SetBool(x)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing failed: %w", err)
		}
		v.SetInt(x)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		x, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing failed: %w", err)
		}
		v.SetUint(x)

	case reflect.Float32, reflect.Float64:
		x, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return fmt.Errorf("parsing failed: %w", err)
		}
		v.SetFloat(x)

	case reflect.Array, reflect.Slice:
		values := strings.Split(str, ",")
		if v.Cap() < len(values) {
			v.Grow(len(values))
			v.SetLen(len(values))
		}
		for i := range len(values) {
			err := decode(strings.TrimSpace(values[i]), v.Index(i))
			if err != nil {
				return fmt.Errorf("parsing failed: %w", err)
			}
		}

	case reflect.String:
		// String already unquoted.
		v.SetString(str)

	default:
		return fmt.Errorf("cannot decode value of type %s", v.Type().String())
	}

	return nil
}

func isNameChar(char byte) bool {
	return char >= 'a' && char <= 'z' ||
		char >= 'A' && char <= 'Z' ||
		char == '_'
}

func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

func isHexDigit(char byte) bool {
	return isDigit(char) ||
		char >= 'a' && char <= 'f' ||
		char >= 'A' && char <= 'F'
}

func isNameCharOrDigit(char byte) bool {
	return isNameChar(char) || isDigit(char)
}

func isNewlineChar(char byte) bool {
	return char == '\n' || char == '\r'
}

func errUnexpectedChar(char byte, line, column uint32) error {
	return fmt.Errorf("unexpected character '%c' at %d:%d", char, line, column)
}

func errExpectedNewLine(line, column int) error {
	return fmt.Errorf("expected new line at %d:%d", line, column)
}
