package assembly

import "net/http"

// HTTPMiddleware wraps outbound HTTP transport.
func HTTPMiddleware(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}

	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return next.RoundTrip(req)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
