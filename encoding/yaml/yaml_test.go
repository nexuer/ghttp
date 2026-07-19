package yaml

import "testing"

type testDocument struct {
	Value string `yaml:"value"`
}

func TestCodec_RoundTrip(t *testing.T) {
	c := codec{}
	data, err := c.Marshal(testDocument{Value: "ghttp"})
	if err != nil {
		t.Fatal(err)
	}

	var target testDocument
	if err := c.Unmarshal(data, &target); err != nil {
		t.Fatal(err)
	}
	if target.Value != "ghttp" {
		t.Fatalf("Unmarshal() value = %q; want ghttp", target.Value)
	}
}

func TestCodec_UnmarshalEmpty(t *testing.T) {
	var target testDocument
	if err := (codec{}).Unmarshal(nil, &target); err != nil {
		t.Fatal(err)
	}
	if target != (testDocument{}) {
		t.Fatalf("Unmarshal() target = %#v; want zero value", target)
	}
}
