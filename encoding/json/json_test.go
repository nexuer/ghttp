package json

import (
	stdjson "encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestCodec_Marshal(t *testing.T) {
	c := codec{}

	tests := []struct {
		input interface{}
		want  string
	}{
		{
			input: nil,
			want:  "null",
		},
		{
			input: "",
			want:  `""`,
		},
	}
	for _, test := range tests {
		if got, _ := c.Marshal(test.input); string(got) != test.want {
			t.Errorf("Marshal(%#v) = %#v, want %#v", test.input, string(got), test.want)
		}
	}
}

func TestCodec_MarshalProtoUsesProtoNames(t *testing.T) {
	c := codec{}
	data, err := c.Marshal(&descriptorpb.FieldDescriptorProto{
		JsonName: proto.String("userId"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"json_name":"userId"`) {
		t.Fatalf("Marshal() = %s; want proto field name json_name", data)
	}
}

func TestCodec_Unmarshal(t *testing.T) {
	c := codec{}

	var regular struct {
		Name string `json:"name"`
	}
	if err := c.Unmarshal([]byte(`{"name":"ghttp"}`), &regular); err != nil {
		t.Fatal(err)
	}
	if regular.Name != "ghttp" {
		t.Fatalf("Unmarshal() name = %q; want ghttp", regular.Name)
	}

	message := new(wrapperspb.StringValue)
	if err := c.Unmarshal([]byte(`"direct"`), message); err != nil {
		t.Fatal(err)
	}
	if message.Value != "direct" {
		t.Fatalf("Unmarshal() protobuf value = %q; want direct", message.Value)
	}
}

func TestCodec_UnmarshalTypedNil(t *testing.T) {
	c := codec{}
	var message *wrapperspb.StringValue

	defer func() {
		if recover() == nil {
			t.Fatal("Unmarshal() typed nil did not panic")
		}
	}()
	_ = c.Unmarshal([]byte(`"value"`), message)
}

func TestCodec_UnmarshalEmpty(t *testing.T) {
	c := codec{}
	var target any
	if err := c.Unmarshal(nil, &target); err == nil {
		t.Fatal("Unmarshal() empty data returned nil error")
	}
}

type benchmarkPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var benchmarkJSON = []byte(`{"name":"ghttp","count":10}`)

func BenchmarkCodec_UnmarshalStruct(b *testing.B) {
	c := codec{}
	for i := 0; i < b.N; i++ {
		var target benchmarkPayload
		if err := c.Unmarshal(benchmarkJSON, &target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdJSON_UnmarshalStruct(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var target benchmarkPayload
		if err := stdjson.Unmarshal(benchmarkJSON, &target); err != nil {
			b.Fatal(err)
		}
	}
}
