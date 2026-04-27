package assembly

import (
	"testing"
	"time"
)

func TestDefaultRuntimeOptions(t *testing.T) {
	t.Parallel()

	opts := defaultRuntimeOptions()
	if opts.failClosed {
		t.Fatal("expected failClosed default to false")
	}
	if opts.timeout != defaultGatewayTimeout {
		t.Fatalf("expected default timeout %v, got %v", defaultGatewayTimeout, opts.timeout)
	}
}

func TestOptionsMutateRuntimeOptions(t *testing.T) {
	t.Parallel()

	opts := defaultRuntimeOptions()
	WithGatewayURL("https://gateway.example.com")(&opts)
	WithAPIKey("test-key")(&opts)
	WithFailClosed(true)(&opts)
	WithTimeout(3 * time.Second)(&opts)

	if opts.gatewayURL != "https://gateway.example.com" {
		t.Fatalf("expected gateway url to be set, got %q", opts.gatewayURL)
	}
	if opts.apiKey != "test-key" {
		t.Fatalf("expected api key to be set, got %q", opts.apiKey)
	}
	if !opts.failClosed {
		t.Fatal("expected failClosed to be true")
	}
	if opts.timeout != 3*time.Second {
		t.Fatalf("expected timeout to be 3s, got %v", opts.timeout)
	}
}
