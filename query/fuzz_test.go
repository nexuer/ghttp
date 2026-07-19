package query

import "testing"

type fuzzValuesStruct struct {
	Value interface{}   `query:"value"`
	Items []interface{} `query:"item"`
}

func FuzzValues_NoPanic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("query"))
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Add([]byte{255, 0, 255, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 256 {
			data = data[:256]
		}

		input := map[string]interface{}{
			"root": fuzzNestedValue(data, 4),
		}
		values, err := Values(input)
		if err != nil {
			t.Fatalf("Values() returned error for generated nested value: %v", err)
		}
		_ = values.Encode()
	})
}

func fuzzNestedValue(data []byte, depth int) interface{} {
	if len(data) == 0 {
		return nil
	}
	if depth == 0 {
		return fuzzScalarValue(data)
	}

	rest := data[1:]
	split := len(rest) / 2
	left := rest[:split]
	right := rest[split:]

	switch data[0] % 7 {
	case 0:
		return map[string]interface{}{
			"value": fuzzNestedValue(rest, depth-1),
		}
	case 1:
		return []interface{}{
			fuzzNestedValue(left, depth-1),
			fuzzNestedValue(right, depth-1),
		}
	case 2:
		return [2]interface{}{
			fuzzNestedValue(left, depth-1),
			fuzzNestedValue(right, depth-1),
		}
	case 3:
		value := fuzzNestedValue(rest, depth-1)
		return &value
	case 4:
		return fuzzValuesStruct{
			Value: fuzzNestedValue(left, depth-1),
			Items: []interface{}{
				fuzzNestedValue(right, depth-1),
				nil,
			},
		}
	case 5:
		var value *string
		return value
	default:
		return fuzzScalarValue(data)
	}
}

func fuzzScalarValue(data []byte) interface{} {
	switch data[0] % 5 {
	case 0:
		return string(data[1:])
	case 1:
		return int8(data[0])
	case 2:
		return data[0]%2 == 0
	case 3:
		return append([]byte(nil), data[1:]...)
	default:
		return nil
	}
}
