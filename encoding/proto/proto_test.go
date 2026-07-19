package proto

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestCodec_RoundTrip(t *testing.T) {
	c := codec{}
	data, err := c.Marshal(wrapperspb.String("ghttp"))
	if err != nil {
		t.Fatal(err)
	}

	target := new(wrapperspb.StringValue)
	if err := c.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
	if target.Value != "ghttp" {
		t.Fatalf("Unmarshal() value = %q; want ghttp", target.Value)
	}
}

func TestCodec_RejectsNonMessage(t *testing.T) {
	c := codec{}
	if _, err := c.Marshal("not a message"); err == nil {
		t.Fatal("Marshal() returned nil error for non-message")
	}
	if err := c.Unmarshal(nil, new(string)); err == nil {
		t.Fatal("Unmarshal() returned nil error for non-message")
	}
}

func TestCodec_UnmarshalEmpty(t *testing.T) {
	c := codec{}
	target := wrapperspb.String("before")
	if err := c.Unmarshal(nil, target); err != nil {
		t.Fatal(err)
	}
	if target.Value != "" {
		t.Fatalf("Unmarshal() value = %q; want zero value", target.Value)
	}
}
