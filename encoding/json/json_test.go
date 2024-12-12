package json

import "testing"

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
