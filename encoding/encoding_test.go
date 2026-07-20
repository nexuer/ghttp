package encoding

import "testing"

type namedCodec struct {
	name string
}

func (namedCodec) Marshal(interface{}) ([]byte, error) { return nil, nil }
func (namedCodec) Unmarshal([]byte, interface{}) error { return nil }
func (c namedCodec) Name() string                      { return c.name }

func TestCodecNamesAreCaseInsensitive(t *testing.T) {
	codec := namedCodec{name: "Mixed-Case-Test-Codec"}
	RegisterCodec(codec)

	for _, name := range []string{"mixed-case-test-codec", "MIXED-CASE-TEST-CODEC"} {
		if got := GetCodec(name); got != codec {
			t.Fatalf("GetCodec(%q) = %v; want registered codec", name, got)
		}
	}
}
