package ini_test

import (
	"reflect"
	"testing"

	"github.com/saffage/ini"
)

func TestMarshal(t *testing.T) {
	type VideoSettings struct {
		Width      int  `ini:"width"`
		Height     int  `ini:"height"`
		FullScreen bool `ini:"fullscreen,omitempty,commented"`
	}
	type Settings struct {
		Video VideoSettings
	}
	t.Run("empty", func(t *testing.T) {
		const expect = "[Video]\nwidth=0\nheight=0\n;fullscreen=\n"
		testMarshal(t, expect, Settings{})
	})
	t.Run("filled", func(t *testing.T) {
		const expect = "[Video]\nwidth=1024\nheight=768\n;fullscreen=true\n"
		testMarshal(t, expect, Settings{
			Video: VideoSettings{
				Width:      1024,
				Height:     768,
				FullScreen: true,
			},
		})
	})
}

type implMarshalINI struct{}

func (implMarshalINI) MarshalINI() ([]ini.Section, error) {
	return []ini.Section{
		{
			Name: "Foo",
			Fields: []ini.Field{
				{
					Name:      "bar",
					Value:     reflect.ValueOf(0),
					OmitEmpty: true,
				},
				{
					Name:  "baz",
					Value: reflect.ValueOf([]int{1, 2, 3}),
				},
			},
		},
	}, nil
}

type emptyMarshalINI struct{}

func (emptyMarshalINI) MarshalINI() ([]ini.Section, error) {
	return nil, nil
}

func TestMarshalINI(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		const expect = ""
		testMarshal(t, expect, emptyMarshalINI{})
	})
	t.Run("filled", func(t *testing.T) {
		const expect = "[Foo]\nbaz=1,2,3\n"
		testMarshal(t, expect, implMarshalINI{})
	})
}

func testMarshal(t *testing.T, expect string, data any) {
	encoded, err := ini.Marshal(data)
	if err != nil {
		t.Error(err)
	} else if string(encoded) != expect {
		t.Errorf("unexpected marshal output\n"+
			"expect:\n%+v\n"+
			"got:\n%+v\n",
			expect,
			string(encoded),
		)
	}
}
