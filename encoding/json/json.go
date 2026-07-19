package json

import (
	"encoding/json"

	"github.com/nexuer/ghttp/encoding"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const Name = "json"

var (

	// MarshalOptions is a configurable JSON format marshaller.
	MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
	// UnmarshalOptions is a configurable JSON format parser.
	UnmarshalOptions = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

func init() {
	encoding.RegisterCodec(codec{})
}

// codec is a Codec implementation with json.
type codec struct{}

func (codec) Marshal(v interface{}) ([]byte, error) {
	switch m := v.(type) {
	case json.Marshaler:
		return m.MarshalJSON()
	case proto.Message:
		return MarshalOptions.Marshal(m)
	default:
		return json.Marshal(m)
	}
}

func (codec) Unmarshal(data []byte, v interface{}) error {
	switch m := v.(type) {
	case json.Unmarshaler:
		return m.UnmarshalJSON(data)
	case proto.Message:
		return UnmarshalOptions.Unmarshal(data, m)
	default:
		return json.Unmarshal(data, v)
	}
}

func (codec) Name() string {
	return Name
}
