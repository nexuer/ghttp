package ghttp

import (
	"net/http"
	"testing"
)

var (
	benchmarkCallOptionsResult *callOptions
	benchmarkRequestHook       = func(*http.Request) error { return nil }
	benchmarkResponseHook      = func(*http.Response) error { return nil }
)

func BenchmarkResolveCallOptions(b *testing.B) {
	scenarios := []struct {
		name string
		opts []CallOption
	}{
		{
			name: "content_type",
			opts: []CallOption{ContentType("application/json")},
		},
		{
			name: "typical",
			opts: []CallOption{
				ContentType("application/json"),
				Query(map[string]any{"page": 1}),
				BearerToken("token"),
				Before(benchmarkRequestHook),
				After(benchmarkResponseHook),
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkCallOptionsResult = resolveCallOptions(scenario.opts...)
			}
		})
	}
}
