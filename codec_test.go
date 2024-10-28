package ghttp

import "testing"

func TestCodecForString(t *testing.T) {
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
		target := CodecForString(v.contentType)
		if target == nil {
			if v.want != "" {
				t.Errorf("CodecForString(%q) got target = %q, want nil", v.contentType, v.want)
			}
		} else {
			if target.Name() != v.want {
				t.Errorf("CodecForString(%q) failed: target=%s want=%s", v.contentType, target.Name(), v.want)
			}
		}

	}
}
