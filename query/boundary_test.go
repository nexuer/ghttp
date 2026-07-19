package query

import (
	"net/url"
	"testing"
	"time"
)

func TestValues_InvalidInput(t *testing.T) {
	_, err := Values(1)
	if err == nil {
		t.Fatal("Values(1) did not return an error")
	}

	want := "query: Values() unsupported kind input. Got int"
	if err.Error() != want {
		t.Fatalf("Values(1) error = %q; want %q", err, want)
	}
}

func TestValues_InvalidQueryString(t *testing.T) {
	_, err := Values("%zz")
	if err == nil {
		t.Fatal("Values(%zz) did not return an error")
	}
}

func TestValues_ByteSlice(t *testing.T) {
	testValue(t, []byte("?a=1&a=2"), url.Values{"a": {"1", "2"}})
}

func TestValues_URLValuesAlias(t *testing.T) {
	input := url.Values{"a": {"1"}}
	got, err := Values(input)
	if err != nil {
		t.Fatalf("Values() returned error: %v", err)
	}

	got.Add("a", "2")
	if values := input["a"]; len(values) != 2 || values[1] != "2" {
		t.Fatalf("Values(url.Values) returned a clone; input = %v", input)
	}
}

func TestValues_QueryTagPrecedesURLTag(t *testing.T) {
	input := struct {
		Value string `query:"query_value" url:"url_value"`
	}{Value: "value"}

	testValue(t, input, url.Values{"query_value": {"value"}})
}

func TestValues_MapTypedNilPointer(t *testing.T) {
	var value *string

	testValue(t, map[string]*string{"value": value}, url.Values{"value": {""}})
	testValue(t, map[string]interface{}{"value": value}, url.Values{"value": {""}})
}

func TestValues_NestedSlices(t *testing.T) {
	structInput := struct {
		Values [][]int `query:"value"`
	}{
		Values: [][]int{{1, 2}, {0}},
	}
	testValue(t, structInput, url.Values{"value": {"1", "2", "0"}})

	mapInput := map[string][][]int{
		"value": {{1}, {2, 0}},
	}
	testValue(t, mapInput, url.Values{"value": {"1", "2", "0"}})
}

func TestValues_ZeroTimeField(t *testing.T) {
	testValue(t, struct {
		Value time.Time `query:"value"`
	}{}, url.Values{"value": {""}})

	testValue(t, struct {
		Value time.Time `query:"value,omitempty"`
	}{}, url.Values{})
}

func TestValues_ConflictingSliceOptions(t *testing.T) {
	t.Run("join option precedence", func(t *testing.T) {
		input := struct {
			Values []string `query:"value,comma,space,semicolon,brackets,del:|" del:"!"`
		}{
			Values: []string{"a", "b"},
		}
		testValue(t, input, url.Values{"value": {"a,b"}})
	})

	t.Run("element name precedence", func(t *testing.T) {
		input := struct {
			Values []string `query:"value,brackets,numbered,idx"`
		}{
			Values: []string{"a", "b"},
		}
		testValue(t, input, url.Values{
			"value[]0": {"a"},
			"value[]1": {"b"},
		})
	})

	t.Run("inline delimiter precedes separate tag", func(t *testing.T) {
		input := struct {
			Values []string `query:"value,del:|" del:"!"`
		}{
			Values: []string{"a", "b"},
		}
		testValue(t, input, url.Values{"value": {"a|b"}})
	})
}
