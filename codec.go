package ghttp

import (
	"net/http"
	"strings"

	"github.com/nexuer/ghttp/encoding"
	"github.com/nexuer/ghttp/encoding/json"
	_ "github.com/nexuer/ghttp/encoding/plain"
	"github.com/nexuer/ghttp/encoding/proto"
	_ "github.com/nexuer/ghttp/encoding/xml"
	"github.com/nexuer/ghttp/encoding/yaml"
)

// codecAliases maps HTTP content subtypes to differently named codecs.
// Registration must be completed before codecs are used concurrently.
var codecAliases = map[string]string{
	"x-protobuf": proto.Name,
	"x-yaml":     yaml.Name,
}

func codecForSubtype(subtype string) encoding.Codec {
	if alias, ok := codecAliases[subtype]; ok {
		subtype = alias
	}
	return encoding.GetCodec(subtype)
}

func registerCodecName(contentType string, name string) {
	if name == "" {
		return
	}
	codecAliases[subContentType(contentType)] = strings.ToLower(name)
}

// RegisterCodec registers a codec and associates it with a content type.
// It must be called before codecs are used concurrently.
func RegisterCodec(contentType string, codec encoding.Codec) {
	if codec == nil {
		return
	}
	encoding.RegisterCodec(codec)
	registerCodecName(contentType, codec.Name())
}

// CodecForContentType returns the codec registered for an HTTP content type.
func CodecForContentType(contentType string) encoding.Codec {
	return codecForSubtype(subContentType(contentType))
}

// CodecForRequest returns the codec selected from an HTTP request header.
// It falls back to JSON when the header is missing or unsupported.
func CodecForRequest(r *http.Request, name ...string) (encoding.Codec, bool) {
	headerName := "Content-Type"
	if len(name) > 0 && name[0] != "" {
		headerName = name[0]
	}
	for _, accept := range r.Header.Values(headerName) {
		codec := CodecForContentType(accept)
		if codec != nil {
			return codec, true
		}
	}
	return encoding.GetCodec(json.Name), false
}

// CodecForResponse returns the codec selected from an HTTP response header.
// It falls back to JSON when the header is missing or unsupported.
func CodecForResponse(r *http.Response, name ...string) (encoding.Codec, bool) {
	headerName := "Content-Type"
	if len(name) > 0 && name[0] != "" {
		headerName = name[0]
	}
	for _, accept := range r.Header.Values(headerName) {
		codec := CodecForContentType(accept)
		if codec != nil {
			return codec, true
		}
	}
	return encoding.GetCodec(json.Name), false
}
