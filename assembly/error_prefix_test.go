package assembly

import (
	"strings"
	"testing"
)

const errorPrefix = "assembly: "

func TestSentinelErrorsHaveAssemblyPrefix(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrRuntimeNotInitialized", ErrRuntimeNotInitialized},
		{"ErrInvalidGateway", ErrInvalidGateway},
		{"ErrInvalidAPIKey", ErrInvalidAPIKey},
		{"ErrSidecarUnavailable", ErrSidecarUnavailable},
	}

	for _, sentinel := range sentinels {
		if !strings.HasPrefix(sentinel.err.Error(), errorPrefix) {
			t.Errorf("%s: expected prefix %q, got %q", sentinel.name, errorPrefix, sentinel.err.Error())
		}
	}
}

func TestPolicyViolationErrorHasAssemblyPrefix(t *testing.T) {
	cases := []struct {
		name string
		err  *PolicyViolationError
	}{
		{"empty", &PolicyViolationError{}},
		{"reason only", &PolicyViolationError{Reason: "blocked"}},
		{"tool only", &PolicyViolationError{ToolName: "web_search"}},
		{"both", &PolicyViolationError{ToolName: "web_search", Reason: "blocked"}},
	}

	for _, tc := range cases {
		if !strings.HasPrefix(tc.err.Error(), errorPrefix) {
			t.Errorf("%s: expected prefix %q, got %q", tc.name, errorPrefix, tc.err.Error())
		}
	}

	var nilErr *PolicyViolationError
	if !strings.HasPrefix(nilErr.Error(), errorPrefix) {
		t.Errorf("nil receiver: expected prefix %q, got %q", errorPrefix, nilErr.Error())
	}
}
