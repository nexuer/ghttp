package plain

import (
	encoding2 "encoding"
	"fmt"
	"reflect"

	"github.com/nexuer/ghttp/encoding"
)

const Name = "plain"

func init() {
	encoding.RegisterCodec(codec{})
}

type codec struct{}

func (codec) Marshal(v any) ([]byte, error) {
	switch v.(type) {
	case string:
		return []byte(v.(string)), nil
	case []byte:
		return v.([]byte), nil
	default:
		return []byte(fmt.Sprintf("%v", v)), nil
	}
}

func (codec) Unmarshal(data []byte, v any) error {
	switch val := v.(type) {
	case *string:
		*val = string(data)
	case *[]byte:
		*val = append([]byte(nil), data...)
	case encoding2.TextUnmarshaler:
		return val.UnmarshalText(data)
	default:
		return fmt.Errorf("supports only *string, *[]byte or encoding.TextUnmarshaler, got: %v", reflect.TypeOf(v))
	}
	return nil
}

func (codec) Name() string {
	return Name
}
