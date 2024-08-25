package ini

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type flags struct {
	key       string
	inline    bool
	omitempty bool
	commented bool
}

func parseTag(t reflect.Type, field reflect.StructField) (flags, error) {
	name, rest, found := strings.Cut(field.Tag.Get("ini"), ",")
	rest = strings.TrimSpace(rest)
	flags := flags{key: strings.TrimSpace(name)}

	if found {
		if rest == "" {
			return flags, errors.New("unexpected comma in field tag")
		}

		for _, flag := range strings.Split(rest, ",") {
			flag = strings.TrimSpace(flag)

			switch flag {
			case "inline":
				if flags.inline {
					return flags, errDuplicateFlag(flag, field.Name, t.String())
				}
				flags.inline = true

			case "omitempty":
				if flags.omitempty {
					return flags, errDuplicateFlag(flag, field.Name, t.String())
				}
				flags.omitempty = true

			case "commented":
				if flags.commented {
					return flags, errDuplicateFlag(flag, field.Name, t.String())
				}
				flags.commented = true

			default:
				return flags, errUnknownFlag(flag, field.Name, t.String())
			}
		}
	}

	return flags, nil
}

func errUnknownFlag(tag, field, t string) error {
	return fmt.Errorf(
		"unknown flag '%s' for field '%s' in type '%s'",
		tag,
		field,
		t,
	)
}

func errDuplicateFlag(tag, field, t string) error {
	return fmt.Errorf(
		"duplicate flag '%s' for field '%s' in type '%s'",
		tag,
		field,
		t,
	)
}
