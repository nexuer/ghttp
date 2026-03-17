package ghttp

import "testing"

func TestEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"纯域名", "example.com", "http://example.com"},
		{"带端口", "127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"已有http", "http://api.test.com", "http://api.test.com"},
		{"已有https", "https://secure.com", "https://secure.com"},
		{"带空格", "  google.com  ", "http://google.com"},
		{"空字符串", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Endpoint(tt.input)
			if result != tt.expected {
				t.Errorf("Endpoint(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

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
