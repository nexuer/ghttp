package plain

import (
	"testing"
)

func TestCodec_Marshal(t *testing.T) {
	c := codec{}

	tests := []struct {
		input interface{}
		want  string
	}{
		// empty
		{
			input: nil,
			want:  "<nil>",
		},
		{
			input: "",
			want:  "",
		},

		// string
		{
			input: "text",
			want:  "text",
		},

		// []byte
		{
			input: []byte("text"),
			want:  "text",
		},
		// bool
		{
			input: true,
			want:  "true",
		},
		{
			input: false,
			want:  "false",
		},
		// number
		{
			input: 100,
			want:  "100",
		},
		{
			input: 200,
			want:  "200",
		},
		{
			input: 200.2,
			want:  "200.2",
		},
	}

	for _, test := range tests {
		if got, _ := c.Marshal(test.input); string(got) != test.want {
			t.Errorf("Marshal(%#v) = %#v, want %#v", test.input, string(got), test.want)
		}
	}
}

func TestCodec_Unmarshal(t *testing.T) {
	c := codec{}
	var textString string
	if err := c.Unmarshal([]byte("text"), &textString); err != nil {
		t.Error(err)
	}
	if textString != "text" {
		t.Errorf("textString = %#v, want text", textString)
	}

	var bytesString []byte
	if err := c.Unmarshal([]byte("bytes"), &bytesString); err != nil {
		t.Error(err)
	}
	if string(bytesString) != "bytes" {
		t.Errorf("bytesString = %#v, want bytes", bytesString)
	}

	var anyData any
	if err := c.Unmarshal([]byte("bytes"), &anyData); err == nil {
		t.Errorf("Unmarshal(any) = %#v, want error", anyData)
	}
}
