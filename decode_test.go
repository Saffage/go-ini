package ini_test

import (
	"reflect"
	"testing"

	"github.com/saffage/ini"
)

// FIXME: this test is flaky because map is unordered.
// func TestMarshalUnmarshalMap(t *testing.T) {
// 	data := map[string]map[string]any{
// 		"Video": {
// 			"width":      1024,
// 			"height":     768,
// 			"fullscreen": true,
// 		},
// 	}
// 	t.Run("map", func(t *testing.T) {
// 		const expect = "[Video]\nwidth=1024\nheight=768\nfullscreen=true\n"
// 		testMarshal(t, expect, data)
// 	})
// }

func TestMarshalUnmarshal(t *testing.T) {
	type A struct {
		String string  `ini:"string"`
		Array  []byte  `ini:"array"`
		Int    int     `ini:"int"`
		Float  float32 `ini:"float"`
		Bool   bool    `ini:"bool"`
	}
	type B struct {
		String string
		Array  []byte
		Int    int
	}
	type C struct {
		B     `ini:",inline"`
		Float float32
		Bool  bool
	}
	type file struct {
		A
		C
	}

	var (
		f1, f2 file
		err    error
		b      []byte
	)

	f1 = file{
		A: A{
			Int:    100,
			Float:  0.15,
			String: "🎉",
			Array:  []byte{10, 20, 30},
			Bool:   true,
		},
	}

	b, err = ini.Marshal(f1)
	if err != nil {
		t.Error(err)
	}
	t.Logf("encoded data:\n%s", string(b))

	err = ini.Unmarshal(b, &f2)
	if err != nil {
		t.Error(err)
	}
	t.Logf("decoded data:\n%#v", f2)

	if !reflect.DeepEqual(f2, f1) {
		t.Errorf(
			"unmarshaled file does not match to the source data\n"+
				"src: %+v\n"+
				"got: %+v\n",
			f1,
			f2,
		)
	}
}
