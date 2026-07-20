package ghttp

import (
	"net/http"
	"testing"
)

func TestNewAcceptNegotiator(t *testing.T) {
	tests := []struct {
		name   string
		offers []string
	}{
		{name: "no offers"},
		{name: "invalid offer", offers: []string{"not-a-media-type"}},
		{name: "wildcard offer", offers: []string{"application/*"}},
		{name: "unsupported offer", offers: []string{"image/avif"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewAcceptNegotiator(tt.offers...); err == nil {
				t.Fatal("NewAcceptNegotiator() error = nil")
			}
		})
	}
}

func TestAcceptNegotiator(t *testing.T) {
	jsonXML, err := NewAcceptNegotiator("application/json", "application/xml")
	if err != nil {
		t.Fatal(err)
	}
	xmlJSON, err := NewAcceptNegotiator("application/xml", "application/json")
	if err != nil {
		t.Fatal(err)
	}
	jsonOnly, err := NewAcceptNegotiator("application/json")
	if err != nil {
		t.Fatal(err)
	}
	vendorJSON, err := NewAcceptNegotiator("application/vnd.api+json")
	if err != nil {
		t.Fatal(err)
	}

	browserAccept := "text/html,application/xhtml+xml,application/xml;q=0.9," +
		"image/avif,image/webp,image/apng,*/*;q=0.8," +
		"application/signed-exchange;v=b3;q=0.7"
	tests := []struct {
		name        string
		negotiator  *AcceptNegotiator
		values      []string
		wantCodec   string
		wantType    string
		wantMatched bool
	}{
		{
			name:        "missing accepts default offer",
			negotiator:  jsonXML,
			wantCodec:   "json",
			wantType:    "application/json",
			wantMatched: true,
		},
		{
			name:        "empty accepts default offer",
			negotiator:  jsonXML,
			values:      []string{" "},
			wantCodec:   "json",
			wantType:    "application/json",
			wantMatched: true,
		},
		{
			name:        "exact",
			negotiator:  jsonXML,
			values:      []string{"application/xml"},
			wantCodec:   "xml",
			wantType:    "application/xml",
			wantMatched: true,
		},
		{
			name:        "case insensitive exact",
			negotiator:  jsonXML,
			values:      []string{"Application/JSON"},
			wantCodec:   "json",
			wantType:    "application/json",
			wantMatched: true,
		},
		{
			name:        "browser header selects weighted offer",
			negotiator:  jsonXML,
			values:      []string{browserAccept},
			wantCodec:   "xml",
			wantType:    "application/xml",
			wantMatched: true,
		},
		{
			name:        "browser header cannot select unoffered xml",
			negotiator:  jsonOnly,
			values:      []string{browserAccept},
			wantCodec:   "json",
			wantType:    "application/json",
			wantMatched: true,
		},
		{
			name:        "registered but unoffered xml does not match",
			negotiator:  jsonOnly,
			values:      []string{"application/xml"},
			wantMatched: false,
		},
		{
			name:        "type wildcard",
			negotiator:  jsonXML,
			values:      []string{"application/*"},
			wantCodec:   "json",
			wantType:    "application/json",
			wantMatched: true,
		},
		{
			name:        "global wildcard uses server preference",
			negotiator:  xmlJSON,
			values:      []string{"*/*"},
			wantCodec:   "xml",
			wantType:    "application/xml",
			wantMatched: true,
		},
		{
			name:        "quality selects json",
			negotiator:  jsonXML,
			values:      []string{"application/xml;q=0.5, application/json;q=0.9"},
			wantCodec:   "json",
			wantType:    "application/json",
			wantMatched: true,
		},
		{
			name:        "specific rejection overrides wildcard",
			negotiator:  jsonXML,
			values:      []string{"application/json;q=0, */*;q=1"},
			wantCodec:   "xml",
			wantType:    "application/xml",
			wantMatched: true,
		},
		{
			name:        "only offer specifically rejected",
			negotiator:  jsonOnly,
			values:      []string{"application/json;q=0, */*;q=1"},
			wantMatched: false,
		},
		{
			name:        "multiple field values",
			negotiator:  jsonXML,
			values:      []string{"image/webp", "application/xml;q=0.8"},
			wantCodec:   "xml",
			wantType:    "application/xml",
			wantMatched: true,
		},
		{
			name:        "quoted comma",
			negotiator:  jsonOnly,
			values:      []string{`application/json;note="a,b";q=0.9`},
			wantCodec:   "json",
			wantType:    "application/json",
			wantMatched: true,
		},
		{
			name:        "structured suffix offer",
			negotiator:  vendorJSON,
			values:      []string{"application/vnd.api+json"},
			wantCodec:   "json",
			wantType:    "application/vnd.api+json",
			wantMatched: true,
		},
		{
			name:        "invalid quality",
			negotiator:  jsonOnly,
			values:      []string{"application/json;q=1.1"},
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codec, contentType, matched := tt.negotiator.NegotiateAccept(tt.values...)
			if matched != tt.wantMatched {
				t.Fatalf("NegotiateAccept() matched = %t; want %t", matched, tt.wantMatched)
			}
			if !matched {
				if codec != nil || contentType != "" {
					t.Fatalf("NegotiateAccept() = (%v, %q, false); want (nil, empty, false)", codec, contentType)
				}
				return
			}
			if codec == nil || codec.Name() != tt.wantCodec || contentType != tt.wantType {
				t.Fatalf("NegotiateAccept() = (%v, %q, true); want (%s, %q, true)",
					codec, contentType, tt.wantCodec, tt.wantType)
			}
		})
	}
}

func TestAcceptNegotiatorRequest(t *testing.T) {
	negotiator, err := NewAcceptNegotiator("application/json", "application/xml")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{Header: http.Header{
		"Accept": {"application/xml"},
	}}

	codec, contentType, ok := negotiator.Negotiate(req)
	if !ok || codec == nil || codec.Name() != "xml" || contentType != "application/xml" {
		t.Fatalf("Negotiate() = (%v, %q, %t); want (xml, application/xml, true)",
			codec, contentType, ok)
	}

	codec, contentType, ok = negotiator.Negotiate(nil)
	if ok || codec != nil || contentType != "" {
		t.Fatalf("Negotiate(nil) = (%v, %q, %t); want (nil, empty, false)",
			codec, contentType, ok)
	}
}

func BenchmarkAcceptNegotiator(b *testing.B) {
	negotiator, err := NewAcceptNegotiator("application/json", "application/xml")
	if err != nil {
		b.Fatal(err)
	}
	browserAccept := "text/html,application/xhtml+xml,application/xml;q=0.9," +
		"image/avif,image/webp,image/apng,*/*;q=0.8," +
		"application/signed-exchange;v=b3;q=0.7"

	benchmarks := []struct {
		name   string
		accept string
	}{
		{name: "exact", accept: "application/json"},
		{name: "wildcard", accept: "*/*"},
		{name: "browser", accept: browserAccept},
	}
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				codec, contentType, ok := negotiator.NegotiateAccept(benchmark.accept)
				if !ok || codec == nil || contentType == "" {
					b.Fatal("NegotiateAccept() failed")
				}
			}
		})
	}
}
