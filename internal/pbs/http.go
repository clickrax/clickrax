package pbs

import (
	"net/http"
	"time"
)

// NewHTTPClient returns an HTTP client with PBS TLS fingerprint pinning.
func NewHTTPClient(fingerprint string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig(fingerprint),
		},
	}
}
