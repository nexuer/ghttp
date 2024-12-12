package ghttp

import (
	"net/http"
	"sync"

	"github.com/nexuer/ghttp/encoding"
	"github.com/nexuer/ghttp/encoding/json"
	"github.com/nexuer/ghttp/encoding/plain"
	"github.com/nexuer/ghttp/encoding/proto"
	"github.com/nexuer/ghttp/encoding/xml"
	"github.com/nexuer/ghttp/encoding/yaml"
)

var defaultContentType = &contentType{
	subType: map[string]string{
		// default: json
		"*": json.Name,

		"json":       json.Name,
		"x-protobuf": proto.Name,
		"xml":        xml.Name,
		"x-yaml":     yaml.Name,
		"yaml":       yaml.Name,
		"plain":      plain.Name,
	},
}

type contentType struct {
	subType map[string]string
	mu      sync.RWMutex
}

func (c *contentType) set(name string, cname string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subType[name] = cname
}

func (c *contentType) get(name string) encoding.Codec {
	return encoding.GetCodec(c.subType[name])
}

func RegisterCodecName(contentType string, name string) {
	if name == "" {
		return
	}
	defaultContentType.set(subContentType(contentType), name)
}

func RegisterCodec(contentType string, codec encoding.Codec) {
	if codec == nil {
		return
	}
	encoding.RegisterCodec(codec)
	defaultContentType.set(subContentType(contentType), codec.Name())
}

// CodecForString get encoding.Codec via string
func CodecForString(contentType string) encoding.Codec {
	return defaultContentType.get(subContentType(contentType))
}

// CodecForRequest get encoding.Codec via http.Request
func CodecForRequest(r *http.Request, name ...string) (encoding.Codec, bool) {
	headerName := "Content-Type"
	if len(name) > 0 && name[0] != "" {
		headerName = name[0]
	}
	for _, accept := range r.Header[headerName] {
		codec := CodecForString(accept)
		if codec != nil {
			return codec, true
		}
	}
	return encoding.GetCodec(json.Name), false
}

// CodecForResponse get encoding.Codec via http.Response
func CodecForResponse(r *http.Response, name ...string) (encoding.Codec, bool) {
	headerName := "Content-Type"
	if len(name) > 0 && name[0] != "" {
		headerName = name[0]
	}
	for _, accept := range r.Header[headerName] {
		codec := CodecForString(accept)
		if codec != nil {
			return codec, true
		}
	}
	return encoding.GetCodec(json.Name), false
}
