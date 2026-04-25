package assembly

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"google.golang.org/grpc"
)

func TestHTTPMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("uses default transport when next is nil", func(t *testing.T) {
		t.Parallel()

		middleware := HTTPMiddleware(nil)
		if middleware == nil {
			t.Fatal("expected middleware transport, got nil")
		}
	})

	t.Run("delegates to next transport", func(t *testing.T) {
		t.Parallel()

		called := false
		wantErr := errors.New("transport error")
		next := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			if req == nil {
				t.Fatal("expected request")
			}
			return nil, wantErr
		})

		middleware := HTTPMiddleware(next)
		_, err := middleware.RoundTrip(httptestRequest(t))
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected error %v, got %v", wantErr, err)
		}
		if !called {
			t.Fatal("expected next transport to be called")
		}
	})
}

func TestUnaryClientInterceptor(t *testing.T) {
	t.Parallel()

	interceptor := UnaryClientInterceptor()
	invoked := false

	err := interceptor(
		context.Background(),
		"/assembly.Test/Ping",
		"req",
		"resp",
		nil,
		func(
			ctx context.Context,
			method string,
			req, reply any,
			cc *grpc.ClientConn,
			opts ...grpc.CallOption,
		) error {
			invoked = true
			if ctx == nil {
				t.Fatal("expected context")
			}
			if method != "/assembly.Test/Ping" {
				t.Fatalf("unexpected method: %s", method)
			}
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !invoked {
		t.Fatal("expected invoker to be called")
	}
}

func TestStreamClientInterceptor(t *testing.T) {
	t.Parallel()

	interceptor := StreamClientInterceptor()
	streamed := false
	wantErr := errors.New("stream error")

	_, err := interceptor(
		context.Background(),
		&grpc.StreamDesc{ServerStreams: true},
		nil,
		"/assembly.Test/Stream",
		func(
			ctx context.Context,
			desc *grpc.StreamDesc,
			cc *grpc.ClientConn,
			method string,
			opts ...grpc.CallOption,
		) (grpc.ClientStream, error) {
			streamed = true
			if ctx == nil {
				t.Fatal("expected context")
			}
			if desc == nil {
				t.Fatal("expected stream descriptor")
			}
			if method != "/assembly.Test/Stream" {
				t.Fatalf("unexpected method: %s", method)
			}
			return nil, wantErr
		},
	)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if !streamed {
		t.Fatal("expected streamer to be called")
	}
}

func httptestRequest(t *testing.T) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	return req
}
