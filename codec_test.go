package ghttp

import (
	"net/http"
	"testing"
)

type namedTestCodec struct {
	name string
}

func (c namedTestCodec) Marshal(v interface{}) ([]byte, error) {
	return nil, nil
}

func (c namedTestCodec) Unmarshal(data []byte, v interface{}) error {
	return nil
}

func (c namedTestCodec) Name() string {
	return c.name
}

func TestCodecForContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		// zero values
		{
			contentType: "",
			want:        "",
		},

		// json
		{
			contentType: "application/json",
			want:        "json",
		},
		{
			contentType: "application/vnd.api+json",
			want:        "json",
		},
		{
			contentType: "application/json; charset=utf-8",
			want:        "json",
		},
		{
			contentType: "application/vnd.docker.distribution.manifest.v2+json; charset=utf-8",
			want:        "json",
		},

		// xml
		{
			contentType: "application/xml",
			want:        "xml",
		},
		{
			contentType: "text/xml",
			want:        "xml",
		},

		// yaml
		{
			contentType: "application/x-yaml",
			want:        "yaml",
		},
		{
			contentType: "text/yaml",
			want:        "yaml",
		},

		// proto
		{
			contentType: "application/x-protobuf",
			want:        "proto",
		},
	}

	for _, v := range tests {
		target := CodecForContentType(v.contentType)
		if target == nil {
			if v.want != "" {
				t.Errorf("CodecForContentType(%q) got target = %q, want nil", v.contentType, v.want)
			}
		} else {
			if target.Name() != v.want {
				t.Errorf("CodecForContentType(%q) failed: target=%s want=%s", v.contentType, target.Name(), v.want)
			}
		}

	}
}

func TestRegisterCodecContentTypeAlias(t *testing.T) {
	codec := namedTestCodec{name: "Test-Codec"}
	RegisterCodec("application/vnd.ghttp-test", codec)

	got := CodecForContentType("application/vnd.ghttp-test")
	if got == nil {
		t.Fatal("CodecForContentType() returned nil")
	}
	if got.Name() != codec.Name() {
		t.Fatalf("CodecForContentType() codec = %q; want %q", got.Name(), codec.Name())
	}
}

func TestCodecForRequest(t *testing.T) {
	req := &http.Request{Header: http.Header{
		"Accept": {"application/xml"},
	}}

	codec, ok := CodecForRequest(req, "accept")
	if !ok || codec == nil || codec.Name() != "xml" {
		t.Fatalf("CodecForRequest() = (%v, %t); want xml, true", codec, ok)
	}

	codec, ok = CodecForRequest(&http.Request{Header: make(http.Header)})
	if ok || codec == nil || codec.Name() != "json" {
		t.Fatalf("CodecForRequest() fallback = (%v, %t); want json, false", codec, ok)
	}
}

func TestCodecForResponse(t *testing.T) {
	resp := &http.Response{Header: http.Header{
		"Content-Type": {"application/x-yaml"},
	}}

	codec, ok := CodecForResponse(resp, "content-type")
	if !ok || codec == nil || codec.Name() != "yaml" {
		t.Fatalf("CodecForResponse() = (%v, %t); want yaml, true", codec, ok)
	}

	codec, ok = CodecForResponse(&http.Response{Header: make(http.Header)})
	if ok || codec == nil || codec.Name() != "json" {
		t.Fatalf("CodecForResponse() fallback = (%v, %t); want json, false", codec, ok)
	}
}
