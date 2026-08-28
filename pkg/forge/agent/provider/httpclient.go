package provider

import (
	"net"
	"net/http"
	"time"
)

// NewHTTPClient returns an http.Client for model-provider calls.
//
// http.DefaultClient has no timeout at all, so a stalled connection hangs the
// caller forever — and with no context on the Provider interface, nothing
// upstream can cancel it. Two layers of protection here:
//
//   - connection-level timeouts (dial, TLS handshake) fail fast when a host
//     is unreachable or the connection stalls before it is established;
//   - total is a per-attempt backstop covering the entire request including
//     reading the body. It must stay generous: a legitimate generation —
//     a long streamed response, or a slow local model — can take minutes,
//     and total applies to streaming reads too.
func NewHTTPClient(total time.Duration) *http.Client {
	return &http.Client{
		Timeout: total,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}
