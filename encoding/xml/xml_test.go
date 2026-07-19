package xml

import "testing"

type testDocument struct {
	Value string `xml:"value"`
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
	if err := (codec{}).Unmarshal(nil, new(testDocument)); err == nil {
		t.Fatal("Unmarshal() empty data returned nil error")
	}
}
