package ini_test

import (
	"reflect"
	"testing"

	"github.com/saffage/go-ini"
)

func TestSectionsOf(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		sections, err := ini.SectionsOf(map[string]any{})
		if err != nil {
			t.Error(err)
		} else if !reflect.DeepEqual(sections, []ini.Section{}) {
			t.Error("SectionsOf returned non-empty tree")
		}
	})
}
