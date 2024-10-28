package proto

import (
	"fmt"

	"github.com/nexuer/ghttp/encoding"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

const Name = "proto"

func init() {
	encoding.RegisterCodec(codec{})
}

// codec is a Codec implementation with protobuf.
type codec struct{}

func (codec) Marshal(v any) ([]byte, error) {
	vv := messageV2Of(v)
	if vv == nil {
		return nil, fmt.Errorf("failed to marshal, message is %T, want proto.Message", v)
	}

	return proto.Marshal(vv)
}
func (codec) Unmarshal(data []byte, v any) error {
	vv := messageV2Of(v)
	if vv == nil {
		return fmt.Errorf("failed to unmarshal, message is %T, want proto.Message", v)
	}

	return proto.Unmarshal(data, vv)
}

func messageV2Of(v any) proto.Message {
	switch v := v.(type) {
	case protoadapt.MessageV1:
		return protoadapt.MessageV2Of(v)
	case protoadapt.MessageV2:
		return v
	}

	return nil
}

func (codec) Name() string {
	return Name
}
