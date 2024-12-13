package ghttp

import "testing"

func TestSubContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{
			contentType: "application/json",
			want:        "json",
		},
		{
			contentType: "application/xml",
			want:        "xml",
		},
		{
			contentType: "application/x-www-form-urlencoded",
			want:        "x-www-form-urlencoded",
		},
		{
			contentType: "multipart/form-data",
			want:        "form-data",
		},
		{
			contentType: "application/vnd.api+json",
			want:        "json",
		},
		{
			contentType: "multipart/byteranges",
			want:        "byteranges",
		},
		{
			contentType: "application/json; charset=utf-8",
			want:        "json",
		},
		{
			contentType: "application/vnd.docker.distribution.manifest.v2+json; charset=utf-8",
			want:        "json",
		},
		{
			contentType: "text/plain; charset=utf-8",
			want:        "plain",
		},
	}

	for _, v := range tests {
		target := subContentType(v.contentType)
		if target != v.want {
			t.Logf("SubContentType() failed: target=%s want=%s", target, v.want)
		}
	}
}

func BenchmarkSubContentType(b *testing.B) {

	for i := 0; i < b.N; i++ {
		subContentType("application/vnd.docker.distribution.manifest.v2+json; charset=utf-8")
	}
}
