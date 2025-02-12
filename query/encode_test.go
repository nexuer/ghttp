package query

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// test that Values(input) matches want.  If not, report an error on t.
func testValue(t *testing.T, input interface{}, want url.Values) {
	v, err := Values(input)
	if err != nil {
		t.Errorf("Values(%q) returned error: %v", input, err)
	}
	if diff := cmp.Diff(want, v); diff != "" {
		t.Errorf("Values(%#v) mismatch:\n%s", input, diff)
	}
}

func TestValues_string(t *testing.T) {
	tests := []struct {
		input string
		want  url.Values
	}{
		// zero values
		{input: "", want: url.Values{}},
		{input: "?", want: url.Values{}},
		{input: "??", want: url.Values{}},

		// simple non-zero values
		{input: "a=a&b=b", want: url.Values{"a": {"a"}, "b": {"b"}}},
		{input: "?a=a&b=b", want: url.Values{"a": {"a"}, "b": {"b"}}},
		{input: "??a=a&b=b", want: url.Values{"a": {"a"}, "b": {"b"}}},

		// slice non-zero values
		{input: "a=1&a=2", want: url.Values{"a": {"1", "2"}}},
		{input: "a[]=1&a[]=2", want: url.Values{"a[]": {"1", "2"}}},

		// hash map non-zero values
		{input: "a[i]=1&a[j]=2", want: url.Values{"a[i]": {"1"}, "a[j]": {"2"}}},
		{input: "a.i=1&a.j=2", want: url.Values{"a.i": {"1"}, "a.j": {"2"}}},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_map(t *testing.T) {

	type ts struct {
		Name string `query:"name,omitempty"`
		Age  int    `query:"age,omitempty"`
	}

	tests := []struct {
		input interface{}
		want  url.Values
	}{
		// zero values
		{input: make(map[string]interface{}), want: url.Values{}},
		{input: make(map[interface{}]interface{}), want: url.Values{}},

		// simple non-zero values
		{input: map[string]interface{}{"a": 1, "b": 2}, want: url.Values{"a": {"1"}, "b": {"2"}}},

		// map
		{
			map[string]interface{}{
				"a": map[string]string{
					"name": "",
				},
			},
			url.Values{},
		},
		{
			map[string]interface{}{
				"a": map[string]string{
					"name": "1",
				},
			},
			url.Values{"a[name]": {"1"}},
		},

		// struct
		{
			map[string]interface{}{
				"a": ts{
					Name: "",
				},
			},
			url.Values{},
		},
		{
			map[string]interface{}{
				"user": &ts{
					Name: "",
				},
			},
			url.Values{},
		},
		{
			map[string]interface{}{
				"user": ts{
					Name: "a",
				},
			},
			url.Values{"user[name]": {"a"}},
		},
		{
			map[string]interface{}{
				"user": &ts{
					Name: "a",
				},
			},
			url.Values{"user[name]": {"a"}},
		},

		// slices
		{input: map[string][]int{"a": []int{1, 2}}, want: url.Values{"a": {"1", "2"}}},
		{input: map[string][]string{"a": []string{"1", "2"}}, want: url.Values{"a": {"1", "2"}}},
		{input: map[string][]bool{"a": []bool{true, false}}, want: url.Values{"a": {"true", "false"}}},
		{input: map[string][]map[string]string{
			"first": []map[string]string{
				{
					"a": "1",
					"b": "1",
				},
			},
			"second": []map[string]string{
				{
					"a": "3",
					"b": "4",
				},
			},
		}, want: url.Values{"first[a]": {"1"}, "first[b]": {"1"}, "second[a]": {"3"}, "second[b]": {"4"}}},
		{input: map[string]interface{}{
			"a": []map[string][]string{
				{
					"b": []string{"c", "d"},
				},
			},
		}, want: url.Values{"a[b]": {"c", "d"}}},
		{
			map[string]interface{}{
				"a": []ts{
					{
						Name: "",
					},
				},
			},
			url.Values{},
		},
		{
			map[string]interface{}{
				"users": []ts{
					{
						Name: "1",
					},
					{
						Name: "2",
						Age:  10,
					},
				},
			},
			url.Values{"users[age]": {"10"}, "users[name]": {"1", "2"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_array_or_slice(t *testing.T) {
	var empty [0]string
	tests := []struct {
		input interface{}
		want  url.Values
	}{
		// zero values
		{input: make([]string, 0), want: url.Values{}},
		{input: empty, want: url.Values{}},

		// simple non-zero values
		{input: []string{"a", "1", "b", "2"}, want: url.Values{"a": {"1"}, "b": {"2"}}},
		{input: [4]string{"a", "1", "b", "2"}, want: url.Values{"a": {"1"}, "b": {"2"}}},

		// simple non-zero values of odd length
		{input: []string{"a", "1", "b"}, want: url.Values{"a": {"1"}}},
		{input: [3]string{"a", "1", "b"}, want: url.Values{"a": {"1"}}},

		// non-zero values
		{input: []interface{}{"a", "1", "b", 2}, want: url.Values{"a": {"1"}, "b": {"2"}}},
		{input: [4]interface{}{"a", 1, "b", "2"}, want: url.Values{"a": {"1"}, "b": {"2"}}},

		// complex types use fmt.Sprint to handle
		{input: []interface{}{"a", []string{"1", "1"}}, want: url.Values{"a": {"[1 1]"}}},
		{input: []interface{}{"a", map[string]string{"1": "1"}}, want: url.Values{"a": {"map[1:1]"}}},

		{input: []interface{}{
			map[string]string{"a": "1", "b": "2"},
			map[string]string{"a": "3", "b": "4"},
		}, want: url.Values{"a": {"1", "3"}, "b": {"2", "4"}}},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_BasicTypes(t *testing.T) {
	tests := []struct {
		input interface{}
		want  url.Values
	}{
		//zero values
		{struct{ V string }{}, url.Values{"V": {""}}},
		{struct{ V int }{}, url.Values{"V": {"0"}}},
		{struct{ V uint }{}, url.Values{"V": {"0"}}},
		{struct{ V float32 }{}, url.Values{"V": {"0"}}},
		{struct{ V bool }{}, url.Values{"V": {"false"}}},

		// simple non-zero values
		{struct{ V string }{"v"}, url.Values{"V": {"v"}}},
		{struct{ V int }{1}, url.Values{"V": {"1"}}},
		{struct{ V uint }{1}, url.Values{"V": {"1"}}},
		{struct{ V float32 }{0.1}, url.Values{"V": {"0.1"}}},
		{struct{ V bool }{true}, url.Values{"V": {"true"}}},

		// bool-specific options
		{
			struct {
				V bool `query:",int"`
			}{false},
			url.Values{"V": {"0"}},
		},
		{
			struct {
				V bool `query:",int"`
			}{true},
			url.Values{"V": {"1"}},
		},

		// time values
		{
			struct {
				V time.Time
			}{time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC)},
			url.Values{"V": {"2000-01-01T12:34:56Z"}},
		},
		{
			struct {
				V time.Time `query:",unix"`
			}{time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC)},
			url.Values{"V": {"946730096"}},
		},
		{
			struct {
				V time.Time `query:",unixmilli"`
			}{time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC)},
			url.Values{"V": {"946730096000"}},
		},
		{
			struct {
				V time.Time `query:",unixnano"`
			}{time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC)},
			url.Values{"V": {"946730096000000000"}},
		},
		{
			struct {
				Date    time.Time `query:"date" layout:"2006-01-02"`
				Time    time.Time `query:"time" layout:"2006-01-02 15:04:05"`
				ISO8601 time.Time `query:"iso8601" layout:"2006-01-02T15:04:05.000Z07:00"`
			}{
				time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
				time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
				time.Date(2000, 1, 1, 12, 34, 56, int(time.Millisecond*20), time.UTC),
			},
			url.Values{"date": {"2000-01-01"}, "time": {"2000-01-01 12:34:56"}, "iso8601": {"2000-01-01T12:34:56.020Z"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_time(t *testing.T) {
	var zeroTime time.Time
	tests := []struct {
		input interface{}
		want  url.Values
	}{
		// string
		{input: zeroTime, want: url.Values{}},
		// map
		{input: map[string]time.Time{
			"start": time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
			"end":   time.Date(2000, 1, 1, 13, 34, 56, 0, time.UTC),
		}, want: url.Values{"start": {"2000-01-01T12:34:56Z"}, "end": {"2000-01-01T13:34:56Z"}}},

		// slices
		{input: []interface{}{
			time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
			time.Date(2000, 1, 1, 13, 34, 56, 0, time.UTC),
		}, want: url.Values{"2000-01-01T12:34:56Z": {"2000-01-01T13:34:56Z"}}},

		// struct
		{
			struct {
				Date    time.Time `query:"date" layout:"2006-01-02"`
				Time    time.Time `query:"time" layout:"2006-01-02 15:04:05"`
				ISO8601 time.Time `query:"iso8601" layout:"2006-01-02T15:04:05.000Z07:00"`
			}{
				time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
				time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
				time.Date(2000, 1, 1, 12, 34, 56, int(time.Millisecond*20), time.UTC),
			},
			url.Values{"date": {"2000-01-01"}, "time": {"2000-01-01 12:34:56"}, "iso8601": {"2000-01-01T12:34:56.020Z"}},
		},
		{
			struct {
				Times []time.Time `query:"times" layout:"2006-01-02"`
			}{
				Times: []time.Time{
					time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
					time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
					time.Date(2000, 1, 1, 12, 34, 56, int(time.Millisecond*20), time.UTC),
				},
			},
			url.Values{"times": {"2000-01-01", "2000-01-01", "2000-01-01"}},
		},
		{
			struct {
				Times map[string]time.Time `query:"times" layout:"2006-01-02"`
			}{
				Times: map[string]time.Time{
					"date":  time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC),
					"date1": time.Date(2000, 1, 1, 12, 34, 56, int(time.Millisecond*20), time.UTC),
				},
			},
			url.Values{"times[date]": {"2000-01-01"}, "times[date1]": {"2000-01-01"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_Pointers(t *testing.T) {
	str := "s"
	strPtr := &str

	tests := []struct {
		input interface{}
		want  url.Values
	}{
		// nil pointers (zero values)
		{struct{ V *string }{}, url.Values{"V": {""}}},
		{struct{ V *int }{}, url.Values{"V": {""}}},

		// non-zero pointer values
		{struct{ V *string }{&str}, url.Values{"V": {"s"}}},
		{struct{ V **string }{&strPtr}, url.Values{"V": {"s"}}},

		// slices of pointer values
		{struct{ V []*string }{}, url.Values{}},
		{struct{ V []*string }{[]*string{&str, &str}}, url.Values{"V": {"s", "s"}}},

		// pointer to slice
		{struct{ V *[]string }{}, url.Values{"V": {""}}},
		{struct{ V *[]string }{&[]string{"a", "b"}}, url.Values{"V": {"a", "b"}}},

		// pointer values for the input struct itself
		{(*struct{})(nil), url.Values{}},
		{&struct{}{}, url.Values{}},
		{&struct{ V string }{}, url.Values{"V": {""}}},
		{&struct{ V string }{"v"}, url.Values{"V": {"v"}}},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_Slices(t *testing.T) {
	tests := []struct {
		input interface{}
		want  url.Values
	}{
		// slices of strings
		{
			struct{ V []string }{},
			url.Values{},
		},
		{
			struct{ V []string }{[]string{}},
			url.Values{},
		},
		{
			struct{ V []string }{[]string{""}},
			url.Values{},
		},
		{
			struct{ V []string }{[]string{"a", "b"}},
			url.Values{"V": {"a", "b"}},
		},
		{
			struct {
				V []string `query:",comma"`
			}{[]string{}},
			url.Values{},
		},
		{
			struct {
				V []string `query:",comma"`
			}{[]string{""}},
			url.Values{"V": {""}},
		},
		{
			struct {
				V []string `query:",comma"`
			}{[]string{"a", "b"}},
			url.Values{"V": {"a,b"}},
		},
		{
			struct {
				V []string `query:",space"`
			}{[]string{"a", "b"}},
			url.Values{"V": {"a b"}},
		},
		{
			struct {
				V []string `query:",semicolon"`
			}{[]string{"a", "b"}},
			url.Values{"V": {"a;b"}},
		},
		{
			struct {
				V []string `query:",brackets"`
			}{[]string{"a", "b"}},
			url.Values{"V[]": {"a", "b"}},
		},
		{
			struct {
				V []string `query:",numbered"`
			}{[]string{"a", "b"}},
			url.Values{"V0": {"a"}, "V1": {"b"}},
		},
		{input: struct {
			V []map[string]string `query:",numbered"`
		}{[]map[string]string{
			map[string]string{"a": "1", "b": "2"},
			map[string]string{"a": "3", "b": "4"},
		},
		}, want: url.Values{"V0[a]": {"1"}, "V0[b]": {"2"}, "V1[a]": {"3"}, "V1[b]": {"4"}}},
		{
			struct {
				V []string `query:",idx"`
			}{[]string{"a", "b"}},
			url.Values{"V[0]": {"a"}, "V[1]": {"b"}},
		},
		{input: struct {
			V []map[string]string `query:",idx"`
		}{[]map[string]string{
			map[string]string{"a": "1", "b": "2"},
			map[string]string{"a": "3", "b": "4"},
		},
		}, want: url.Values{"V[0][a]": {"1"}, "V[0][b]": {"2"}, "V[1][a]": {"3"}, "V[1][b]": {"4"}}},

		// arrays of strings
		{
			struct{ V [2]string }{},
			url.Values{},
		},
		{
			struct{ V [2]string }{[2]string{"a", "b"}},
			url.Values{"V": {"a", "b"}},
		},
		{
			struct {
				V [2]string `query:",comma"`
			}{[2]string{"a", "b"}},
			url.Values{"V": {"a,b"}},
		},
		{
			struct {
				V [2]string `query:",space"`
			}{[2]string{"a", "b"}},
			url.Values{"V": {"a b"}},
		},
		{
			struct {
				V [2]string `query:",semicolon"`
			}{[2]string{"a", "b"}},
			url.Values{"V": {"a;b"}},
		},
		{
			struct {
				V [2]string `query:",brackets"`
			}{[2]string{"a", "b"}},
			url.Values{"V[]": {"a", "b"}},
		},
		{
			struct {
				V [2]string `query:",numbered"`
			}{[2]string{"a", "b"}},
			url.Values{"V0": {"a"}, "V1": {"b"}},
		},
		{
			struct {
				V [2]string `query:",idx"`
			}{[2]string{"a", "b"}},
			url.Values{"V[0]": {"a"}, "V[1]": {"b"}},
		},

		// custom delimiters
		{
			struct {
				V []string `del:","`
			}{[]string{"a", "b"}},
			url.Values{"V": {"a,b"}},
		},
		{
			struct {
				V []string `del:"|"`
			}{[]string{"a", "b"}},
			url.Values{"V": {"a|b"}},
		},
		{
			struct {
				V []string `del:"🥑"`
			}{[]string{"a", "b"}},
			url.Values{"V": {"a🥑b"}},
		},

		// slice of bools with additional options
		{
			struct {
				V []bool `query:",space,int"`
			}{[]bool{true, false}},
			url.Values{"V": {"1 0"}},
		},

		// map
		{
			struct {
				V map[string]string `query:"vmap"`
			}{
				map[string]string{
					"a": "1",
					"b": "1",
				},
			},
			url.Values{"vmap[a]": {"1"}, "vmap[b]": {"1"}},
		},
		{
			struct {
				V map[string]string `query:",inline"`
			}{
				map[string]string{
					"a": "1",
					"b": "1",
				},
			},
			url.Values{"a": {"1"}, "b": {"1"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_inline(t *testing.T) {
	type in struct {
		Val string `query:"val"`
		Key string `query:"key"`
	}

	type inin struct {
		In in `query:",inline"`
	}

	type outin struct {
		In in `query:"out"`
	}

	tests := []struct {
		input interface{}
		want  url.Values
	}{
		// map
		{
			struct {
				Name  string                 `query:"name"`
				Pages map[string]interface{} `query:",inline"`
			}{Pages: map[string]interface{}{"a": "1", "b": "2"}},
			url.Values{"name": {""}, "a": {"1"}, "b": {"2"}},
		},
		{
			struct {
				Name  string                 `query:"name"`
				Pages map[string]interface{} `query:",inline"`
				Sets  map[string]interface{} `query:"sets"`
			}{Pages: map[string]interface{}{"a": "1", "b": "2"}, Sets: map[string]interface{}{"a": "3", "b": "4"}},
			url.Values{"name": {""}, "a": {"1"}, "b": {"2"}, "sets[a]": {"3"}, "sets[b]": {"4"}},
		},
		{
			struct {
				Name   string          `query:"name"`
				Inline map[string]inin `query:",inline"`
				In     in              `query:",inline"`
				InRaw  in              `query:"in"`
				Out    outin           `query:",inline"`
				OutRaw outin           `query:"out"`
			}{
				Name:   "myName",
				Inline: map[string]inin{"inline": inin{}},
			},
			url.Values{
				"name":          {"myName"},
				"key":           {""},
				"val":           {""},
				"inline[key]":   {""},
				"inline[val]":   {""},
				"in[key]":       {""},
				"in[val]":       {""},
				"out[key]":      {""},
				"out[val]":      {""},
				"out[out][key]": {""},
				"out[out][val]": {""},
			},
		},
		{
			struct {
				Name      string `query:"name"`
				Ins       []inin `query:"ins"`
				InsIdx    []inin `query:"ins_idx,idx"`
				InsNumber []inin `query:"ins_num,numbered"`
			}{
				Name:      "myName",
				Ins:       []inin{{}, {}},
				InsIdx:    []inin{{}, {}},
				InsNumber: []inin{{}, {}},
			},
			url.Values{
				"name":            {"myName"},
				"ins[key]":        {"", ""},
				"ins[val]":        {"", ""},
				"ins_idx[0][val]": {""},
				"ins_idx[0][key]": {""},
				"ins_idx[1][val]": {""},
				"ins_idx[1][key]": {""},
				"ins_num0[val]":   {""},
				"ins_num0[key]":   {""},
				"ins_num1[val]":   {""},
				"ins_num1[key]":   {""},
			},
		},
		//
		// struct
		{
			struct {
				Name   string `query:"name"`
				Pages  in     `query:",inline"`
				Inline *inin  `query:",inline,omitempty"`
			}{Pages: in{Key: "1"}, Inline: nil},
			url.Values{"key": {"1"}, "name": {""}, "val": {""}},
		},
		{
			struct {
				Name    string `query:"name"`
				Pages   in     `query:",inline"`
				Inline  *inin  `query:",inline,omitempty"`
				Outline inin   `query:"outline"`
			}{
				Pages:   in{Key: "1"},
				Inline:  &inin{in{}},
				Outline: inin{in{}},
			},
			url.Values{"key": {"1", ""}, "name": {""}, "val": {"", ""}, "outline[key]": {""}, "outline[val]": {""}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_NestedTypes(t *testing.T) {
	type SubNested struct {
		Value string `query:"value"`
	}

	type Nested struct {
		A      SubNested  `query:"a"`
		B      *SubNested `query:"b"`
		C      string
		Inline *SubNested `query:",inline,omitempty"`
		Ptr    *SubNested `query:"ptr,omitempty"`
	}

	tests := []struct {
		input interface{}
		want  url.Values
	}{
		{
			struct {
				Nest Nested `query:"nest"`
			}{
				Nested{
					A: SubNested{
						Value: "v",
					},
				},
			},
			url.Values{
				"nest[a][value]": {"v"},
				"nest[b]":        {""},
				"nest[C]":        {""},
			},
		},
		{
			struct {
				Nest Nested `query:"nest"`
			}{
				Nested{
					Ptr: &SubNested{
						Value: "v",
					},
				},
			},
			url.Values{
				"nest[a][value]":   {""},
				"nest[b]":          {""},
				"nest[ptr][value]": {"v"},
				"nest[C]":          {""},
			},
		},
		{
			struct {
				Nest Nested `query:",inline"`
			}{
				Nested{
					C: "v",
				},
			},
			url.Values{
				"a[value]": {""},
				"b":        {""},
				"C":        {"v"},
			},
		},
		{
			struct {
				Nest Nested `query:"nest"`
			}{
				Nested{
					Inline: &SubNested{
						Value: "v",
					},
				},
			},
			url.Values{
				"nest[a][value]": {""},
				"nest[b]":        {""},
				"nest[C]":        {""},
				"nest[value]":    {"v"},
			},
		},
		{
			nil,
			url.Values{},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_OmitEmpty(t *testing.T) {
	str := ""

	tests := []struct {
		input interface{}
		want  url.Values
	}{
		{struct{ v string }{}, url.Values{}}, // non-exported field
		{
			struct {
				V string `query:",omitempty"`
			}{},
			url.Values{},
		},
		{
			struct {
				V string `query:"-"`
			}{},
			url.Values{},
		},
		{
			struct {
				V string `query:"omitempty"` // actually named omitempty
			}{},
			url.Values{"omitempty": {""}},
		},
		{
			// include value for a non-nil pointer to an empty value
			struct {
				V *string `query:",omitempty"`
			}{&str},
			url.Values{"V": {""}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestValues_EmbeddedStructs(t *testing.T) {
	type Inner struct {
		V string
	}
	type Outer struct {
		Inner
	}
	type OuterPtr struct {
		*Inner
	}
	type Mixed struct {
		Inner
		V string
	}
	type unexported struct {
		Inner
		V string
	}
	type Exported struct {
		unexported
	}

	tests := []struct {
		input interface{}
		want  url.Values
	}{
		{
			Outer{Inner{V: "a"}},
			url.Values{"V": {"a"}},
		},
		{
			OuterPtr{&Inner{V: "a"}},
			url.Values{"V": {"a"}},
		},
		{
			Mixed{Inner: Inner{V: "a"}, V: "b"},
			url.Values{"V": {"b", "a"}},
		},
		{
			// values from unexported embed are still included
			Exported{
				unexported{
					Inner: Inner{V: "bar"},
					V:     "foo",
				},
			},
			url.Values{"V": {"foo", "bar"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

// customEncodedStrings is a slice of strings with a custom URL encoding
type customEncodedStrings []string

// EncodeValues using key name of the form "{key}.N" where N increments with
// each value.  A value of "err" will return an error.
func (m customEncodedStrings) EncodeValues(key string, v *url.Values) error {
	for i, arg := range m {
		if arg == "err" {
			return errors.New("encoding error")
		}
		v.Set(fmt.Sprintf("%s.%d", key, i), arg)
	}
	return nil
}

func TestValues_CustomEncodingSlice(t *testing.T) {
	tests := []struct {
		input interface{}
		want  url.Values
	}{
		{
			struct {
				V customEncodedStrings `query:"v"`
			}{},
			url.Values{},
		},
		{
			struct {
				V customEncodedStrings `query:"v"`
			}{[]string{"a", "b"}},
			url.Values{"v.0": {"a"}, "v.1": {"b"}},
		},

		// pointers to custom encoded types
		{
			struct {
				V *customEncodedStrings `url:"v"`
			}{},
			url.Values{},
		},
		{
			struct {
				V *customEncodedStrings `url:"v"`
			}{(*customEncodedStrings)(&[]string{"a", "b"})},
			url.Values{"v.0": {"a"}, "v.1": {"b"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

// One of the few ways reflectValues will return an error is if a custom
// encoder returns an error.  Test all of the various ways that can happen.
func TestValues_CustomEncoding_Error(t *testing.T) {
	type st struct {
		V customEncodedStrings
	}
	tests := []struct {
		input interface{}
	}{
		{
			st{[]string{"err"}},
		},
		{ // struct field
			struct{ S st }{st{[]string{"err"}}},
		},
		{ // embedded struct
			struct{ st }{st{[]string{"err"}}},
		},
	}
	for _, tt := range tests {
		_, err := Values(tt.input)
		if err == nil {
			t.Errorf("Values(%q) did not return expected encoding error", tt.input)
		}
	}
}

// customEncodedInt is an int with a custom URL encoding
type customEncodedInt int

// EncodeValues encodes values with leading underscores
func (m customEncodedInt) EncodeValues(key string, v *url.Values) error {
	v.Set(key, fmt.Sprintf("_%d", m))
	return nil
}

func TestValues_CustomEncodingInt(t *testing.T) {
	var zero customEncodedInt = 0
	var one customEncodedInt = 1
	tests := []struct {
		input interface{}
		want  url.Values
	}{
		{
			struct {
				V customEncodedInt `url:"v"`
			}{},
			url.Values{"v": {"_0"}},
		},
		{
			struct {
				V customEncodedInt `url:"v,omitempty"`
			}{zero},
			url.Values{},
		},
		{
			struct {
				V customEncodedInt `url:"v"`
			}{one},
			url.Values{"v": {"_1"}},
		},

		// pointers to custom encoded types
		{
			struct {
				V *customEncodedInt `url:"v"`
			}{},
			url.Values{"v": {"_0"}},
		},
		{
			struct {
				V *customEncodedInt `url:"v,omitempty"`
			}{},
			url.Values{},
		},
		{
			struct {
				V *customEncodedInt `url:"v,omitempty"`
			}{&zero},
			url.Values{"v": {"_0"}},
		},
		{
			struct {
				V *customEncodedInt `url:"v"`
			}{&one},
			url.Values{"v": {"_1"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

// customEncodedInt is an int with a custom URL encoding defined on its pointer
// value.
type customEncodedIntPtr int

// EncodeValues encodes a 0 as false, 1 as true, and nil as unknown.  All other
// values cause an error.
func (m *customEncodedIntPtr) EncodeValues(key string, v *url.Values) error {
	if m == nil {
		v.Set(key, "undefined")
	} else {
		v.Set(key, fmt.Sprintf("_%d", *m))
	}
	return nil
}

// Test behavior when encoding is defined for a pointer of a custom type.
// Custom type should be able to encode values for nil pointers.
func TestValues_CustomEncodingPointer(t *testing.T) {
	var zero customEncodedIntPtr = 0
	var one customEncodedIntPtr = 1
	tests := []struct {
		input interface{}
		want  url.Values
	}{
		// non-pointer values do not get the custom encoding because
		// they don't implement the encoder interface.
		{
			struct {
				V customEncodedIntPtr `url:"v"`
			}{},
			url.Values{"v": {"0"}},
		},
		{
			struct {
				V customEncodedIntPtr `url:"v,omitempty"`
			}{},
			url.Values{},
		},
		{
			struct {
				V customEncodedIntPtr `url:"v"`
			}{one},
			url.Values{"v": {"1"}},
		},

		// pointers to custom encoded types.
		{
			struct {
				V *customEncodedIntPtr `url:"v"`
			}{},
			url.Values{"v": {"undefined"}},
		},
		{
			struct {
				V *customEncodedIntPtr `url:"v,omitempty"`
			}{},
			url.Values{},
		},
		{
			struct {
				V *customEncodedIntPtr `url:"v"`
			}{&zero},
			url.Values{"v": {"_0"}},
		},
		{
			struct {
				V *customEncodedIntPtr `url:"v,omitempty"`
			}{&zero},
			url.Values{"v": {"_0"}},
		},
		{
			struct {
				V *customEncodedIntPtr `url:"v"`
			}{&one},
			url.Values{"v": {"_1"}},
		},
	}

	for _, tt := range tests {
		testValue(t, tt.input, tt.want)
	}
}

func TestIsEmptyValue(t *testing.T) {
	str := "string"
	tests := []struct {
		value interface{}
		empty bool
	}{
		// slices, arrays, and maps
		{[]int{}, true},
		{[]int{0}, false},
		{[0]int{}, true},
		{[3]int{}, false},
		{[3]int{1}, false},
		{map[string]string{}, true},
		{map[string]string{"a": "b"}, false},

		// strings
		{"", true},
		{" ", false},
		{"a", false},

		// bool
		{true, false},
		{false, true},

		// ints of various types
		{(int)(0), true}, {(int)(1), false}, {(int)(-1), false},
		{(int8)(0), true}, {(int8)(1), false}, {(int8)(-1), false},
		{(int16)(0), true}, {(int16)(1), false}, {(int16)(-1), false},
		{(int32)(0), true}, {(int32)(1), false}, {(int32)(-1), false},
		{(int64)(0), true}, {(int64)(1), false}, {(int64)(-1), false},
		{(uint)(0), true}, {(uint)(1), false},
		{(uint8)(0), true}, {(uint8)(1), false},
		{(uint16)(0), true}, {(uint16)(1), false},
		{(uint32)(0), true}, {(uint32)(1), false},
		{(uint64)(0), true}, {(uint64)(1), false},

		// floats
		{(float32)(0), true}, {(float32)(0.0), true}, {(float32)(0.1), false},
		{(float64)(0), true}, {(float64)(0.0), true}, {(float64)(0.1), false},

		// pointers
		{(*int)(nil), true},
		{new([]int), false},
		{&str, false},

		// time
		{time.Time{}, true},
		{time.Now(), false},

		// unknown type - always false unless a nil pointer, which are always empty.
		{(*struct{ int })(nil), true},
		{struct{ int }{}, false},
		{struct{ int }{0}, false},
		{struct{ int }{1}, false},
	}

	for _, tt := range tests {
		got := isEmptyValue(reflect.ValueOf(tt.value))
		want := tt.empty
		if got != want {
			t.Errorf("isEmptyValue(%v) returned %t; want %t", tt.value, got, want)
		}
	}
}

func TestParseTag(t *testing.T) {
	name, opts := parseTag("field,foobar,foo", reflect.StructField{})
	if name != "field" {
		t.Fatalf("name = %q, want field", name)
	}
	for _, tt := range []struct {
		opt  string
		want bool
	}{
		{"foobar", true},
		{"foo", true},
		{"bar", false},
		{"field", false},
	} {
		if opts.contains(tt.opt) != tt.want {
			t.Errorf("Contains(%q) = %v", tt.opt, !tt.want)
		}
	}
}
