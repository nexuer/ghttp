package ghttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nexuer/ghttp/encoding"
)

// Debugger observes a single HTTP request lifecycle. Begin may return a derived
// request, which must be the request passed to both the transport and End.
type Debugger interface {
	Begin(request *http.Request) *http.Request
	End(request *http.Request, response *http.Response, err error)
}

type Debug struct {
	Writer        io.Writer
	Trace         bool
	TraceCallback func(w io.Writer, info TraceInfo)
}

func (d *Debug) writer() io.Writer {
	if d.Writer == nil {
		return os.Stderr
	}
	return d.Writer
}

type debugStateKey struct{}

func (d *Debug) Begin(req *http.Request) *http.Request {
	if !d.Trace {
		return req
	}

	state := &traceInfo{startTime: time.Now()}
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			state.dnsStartTime = time.Now()
			state.host = info.Host
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			state.dnsDoneTime = time.Now()
			state.dnsDoneInfo = &dnsInfo
		},
		GetConn: func(hostPort string) {
			state.getConnTime = time.Now()
			state.getConnHostPort = hostPort
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			state.gotConnTime = time.Now()
			state.gotConnInfo = &connInfo
		},
		TLSHandshakeStart: func() {
			state.tlsHandshakeStartTime = time.Now()
		},
		TLSHandshakeDone: func(connectionState tls.ConnectionState, err error) {
			state.tlsHandshakeDoneTime = time.Now()
			state.tlsConnectionState = &connectionState
		},
		GotFirstResponseByte: func() {
			state.gotFirstResponseByteTime = time.Now()
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			state.wroteRequestTime = time.Now()
		},
	}

	ctx := context.WithValue(req.Context(), debugStateKey{}, state)
	ctx = httptrace.WithClientTrace(ctx, trace)
	return req.WithContext(ctx)
}

func (d *Debug) statTraceInfo(ctx context.Context, state *traceInfo) TraceInfo {
	if !d.Trace || state == nil {
		return TraceInfo{}
	}
	return TraceInfo{
		ctx:                  ctx,
		DNSDuration:          state.dnsDoneTime.Sub(state.dnsStartTime),
		ConnectDuration:      state.gotConnTime.Sub(state.getConnTime),
		TLSHandshakeDuration: state.tlsHandshakeDoneTime.Sub(state.tlsHandshakeStartTime),
		RequestDuration:      state.wroteRequestTime.Sub(state.gotConnTime),
		WaitResponseDuration: state.gotFirstResponseByteTime.Sub(state.wroteRequestTime),

		ResponseDuration: state.responseDoneTime.Sub(state.gotFirstResponseByteTime),
		TotalDuration:    state.responseDoneTime.Sub(state.startTime),
	}
}

func (d *Debug) End(request *http.Request, response *http.Response, err error) {
	writer := d.writer()

	// print request and response
	path := request.URL.String()
	if path == "" {
		path = "/"
	}

	if d.Trace {
		state, _ := request.Context().Value(debugStateKey{}).(*traceInfo)
		if state != nil {
			state.responseDoneTime = time.Now()
		}
		if d.TraceCallback != nil {
			d.TraceCallback(writer, d.statTraceInfo(request.Context(), state))
		}
		if state != nil {
			if state.host == "" {
				state.host = request.URL.Host
			}
			state.write(writer)
		}
	}

	write(writer, "* using %s", request.Proto)
	write(writer, "> %s %s %s", request.Method, path, request.Proto)
	// write request header
	for k, v := range request.Header {
		write(writer, "> %s: %s", k, strings.Join(v, ","))
	}

	// request body
	if request.GetBody != nil {
		if reqBodyReader, err := request.GetBody(); err == nil {
			reqBody, readErr := io.ReadAll(reqBodyReader)
			_ = reqBodyReader.Close()
			if readErr == nil {
				codec, _ := CodecForRequest(request)
				reqBodyBs, _ := formatIndent(codec, reqBody)
				if len(reqBodyBs) > 0 {
					write(writer, "")
					write(writer, "%s", string(reqBodyBs))
				}
			}
		}
	} else {
		write(writer, ">")
	}

	if response != nil {
		write(writer, "")
		// response
		write(writer, "< %s %s", response.Proto, response.Status)
		for k, v := range response.Header {
			write(writer, "< %s: %s", k, strings.Join(v, ","))
		}
		// response body
		if response.Body != nil && response.Body != http.NoBody {
			originalBody := response.Body
			if responseBody, err := io.ReadAll(originalBody); err == nil {
				_ = originalBody.Close()
				response.Body = io.NopCloser(bytes.NewReader(responseBody))
				codec, _ := CodecForResponse(response)
				resBodyBs, _ := formatIndent(codec, responseBody)
				if len(resBodyBs) > 0 {
					write(writer, "")
					write(writer, "%s", string(resBodyBs))
				} else {
					write(writer, "")
					write(writer, "%s", string(responseBody))
				}
			}
		}
	}

	if err != nil {
		write(writer, "")
		write(writer, "** ERROR: %s", err)
	}
}

type TraceInfo struct {
	ctx context.Context

	DNSDuration          time.Duration `json:"DNSDuration,omitempty" yaml:"DNSDuration" xml:"DNSDuration"`
	ConnectDuration      time.Duration `json:"connectDuration,omitempty" yaml:"connectDuration" xml:"connectDuration"`
	TLSHandshakeDuration time.Duration `json:"TLSHandshakeDuration,omitempty" yaml:"TLSHandshakeDuration" xml:"TLSHandshakeDuration"`
	RequestDuration      time.Duration `json:"requestDuration,omitempty" yaml:"requestDuration" xml:"requestDuration"`
	WaitResponseDuration time.Duration `json:"waitResponseDuration,omitempty" yaml:"waitResponseDuration" xml:"waitResponseDuration"`
	ResponseDuration     time.Duration `json:"responseDuration,omitempty" yaml:"responseDuration" xml:"responseDuration"`
	TotalDuration        time.Duration `json:"totalDuration,omitempty" yaml:"totalDuration" xml:"totalDuration"`
}

func (t TraceInfo) Context() context.Context {
	return t.ctx
}

func (t TraceInfo) String() string {
	return string(t.Table())
}

func (t TraceInfo) Table() []byte {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 30, 0, 3, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(w, "--------------------------------------------")
	_, _ = fmt.Fprintln(w, "Trace\tValue\t")
	_, _ = fmt.Fprintln(w, "--------------------------------------------")
	_, _ = fmt.Fprintf(w, "DNSDuration\t%s\t\n", t.DNSDuration)
	_, _ = fmt.Fprintf(w, "ConnectDuration\t%s\t\n", t.ConnectDuration)
	_, _ = fmt.Fprintf(w, "TLSHandshakeDuration\t%s\t\n", t.TLSHandshakeDuration)
	_, _ = fmt.Fprintf(w, "RequestDuration\t%s\t\n", t.RequestDuration)
	_, _ = fmt.Fprintf(w, "WaitResponseDuration\t%s\t\n", t.WaitResponseDuration)
	_, _ = fmt.Fprintf(w, "TotalDuration\t%s\t\n", t.TotalDuration)
	_, _ = fmt.Fprintln(w, "--------------------------------------------")

	_ = w.Flush()
	return buf.Bytes()
}

type traceInfo struct {
	host               string
	dnsDoneInfo        *httptrace.DNSDoneInfo
	getConnHostPort    string
	gotConnInfo        *httptrace.GotConnInfo
	tlsConnectionState *tls.ConnectionState

	dnsStartTime             time.Time
	dnsDoneTime              time.Time
	getConnTime              time.Time
	gotConnTime              time.Time
	tlsHandshakeStartTime    time.Time
	tlsHandshakeDoneTime     time.Time
	gotFirstResponseByteTime time.Time
	wroteRequestTime         time.Time

	startTime        time.Time
	responseDoneTime time.Time
}

func (t traceInfo) write(w io.Writer) {
	// print trace
	if t.dnsDoneInfo != nil {
		write(w, "* Host %s was resolved.", t.getConnHostPort)
		for _, ipAddr := range t.dnsDoneInfo.Addrs {
			if len(ipAddr.IP) == net.IPv4len {
				write(w, "* IPv4: %s", ipAddr.IP)
			}
			if len(ipAddr.IP) == net.IPv6len {
				write(w, "* IPv6: %s", ipAddr.IP)
			}
		}
	}

	if t.gotConnInfo != nil {
		remoteAddr := t.gotConnInfo.Conn.RemoteAddr()
		write(w, "*   Trying %s...", remoteAddr)
		ip, port, _ := net.SplitHostPort(remoteAddr.String())
		write(w, "* Connected to %s (%s) port %s", t.host, ip, port)
	}

	if t.tlsConnectionState != nil {
		write(w, "* SSL connection using %s / %s",
			tls.VersionName(t.tlsConnectionState.Version),
			tls.CipherSuiteName(t.tlsConnectionState.CipherSuite),
		)
		write(w, "* ALPN: server accepted %s", t.tlsConnectionState.NegotiatedProtocol)
		if len(t.tlsConnectionState.VerifiedChains) > 0 && len(t.tlsConnectionState.VerifiedChains[0]) > 0 {
			cer := t.tlsConnectionState.VerifiedChains[0][0]
			write(w, `* Server certificate:
*   subject: CN=%s
*   notBefore: %s
*   notAfter: %s
*   issuer: C=%s; ST=%s; L=%s; O=%s; CN=%s
*   SSL certificate verify ok.`, cer.Subject.CommonName, cer.NotBefore, cer.NotAfter,
				getFirst(cer.Issuer.Country), getFirst(cer.Issuer.Province), getFirst(cer.Issuer.Locality),
				getFirst(cer.Issuer.Organization), cer.Issuer.CommonName)
		}
	}

}

func getFirst(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

func write(w io.Writer, format string, args ...any) {
	if format != "" {
		_, _ = fmt.Fprintf(w, format, args...)
	}
	_, _ = fmt.Fprintf(w, "\n")
}

func formatIndent(codec encoding.Codec, data []byte) (result []byte, err error) {
	if len(data) == 0 || codec == nil {
		return result, nil
	}

	var anyData any
	if err = codec.Unmarshal(data, &anyData); err != nil {
		return data, err
	}

	switch codec.Name() {
	case "json":
		result, err = json.MarshalIndent(anyData, "", "    ")
	default:
		result, err = codec.Marshal(anyData)
	}

	return
}
