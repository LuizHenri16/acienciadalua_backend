package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// newHTTPClient builds a client tuned for concurrency: connection reuse per
// worker, no transparent decompression (keeps byte counts honest) and a hard
// per-request timeout.
func newHTTPClient(opts *Options) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          opts.Concurrency * 2,
		MaxIdleConnsPerHost:   opts.Concurrency,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
		// Report redirects as-is instead of following them, so the status
		// distribution reflects what the API actually answered.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// newRequest builds a request for baseURL+path. A nil body means no body and
// no Content-Type header.
func newRequest(ctx context.Context, method, baseURL, path string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, joinURL(baseURL, path), reader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
	}

	return req, nil
}

func joinURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

var fakeIPCounter atomic.Uint64

// nextFakeIP returns a unique address in 10.0.0.0/8. The API's RealIP
// middleware trusts X-Forwarded-For, so each request is rate limited as a
// different client.
func nextFakeIP() string {
	n := fakeIPCounter.Add(1)
	return fmt.Sprintf("10.%d.%d.%d", (n>>16)&0xff, (n>>8)&0xff, n&0xff)
}
