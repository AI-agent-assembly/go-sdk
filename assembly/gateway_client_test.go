package assembly

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGatewayClientCheckCancelledContextFailsFast(t *testing.T) {
	t.Parallel()

	called := false
	client := NewGatewayClient(gatewayTransportStub{check: func(context.Context, CheckRequest) (Decision, error) {
		called = true
		return Decision{}, nil
	}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Check(ctx, CheckRequest{ToolName: "calculator"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if called {
		t.Fatal("expected transport check not to be called on cancelled context")
	}
}

func TestGatewayClientCheckAppliesDefaultTimeoutWhenMissingDeadline(t *testing.T) {
	t.Parallel()

	observedDeadline := time.Time{}
	client := NewGatewayClient(gatewayTransportStub{check: func(ctx context.Context, _ CheckRequest) (Decision, error) {
		dl, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected deadline to be set by GatewayClient.Check")
		}
		observedDeadline = dl
		return Decision{}, nil
	}}, WithTimeout(timeout))

	_, err := client.Check(context.Background(), CheckRequest{ToolName: "calculator"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if observedDeadline.IsZero() {
		t.Fatal("expected observed deadline to be recorded")
	}

	remaining := time.Until(observedDeadline)
	if remaining <= 0 {
		t.Fatalf("expected positive remaining timeout, got %v", remaining)
	}
	if remaining > defaultGatewayTimeout {
		t.Fatalf("expected remaining timeout <= %v, got %v", defaultGatewayTimeout, remaining)
	}
}

type gatewayTransportStub struct {
	check func(ctx context.Context, request CheckRequest) (Decision, error)
}

func (s gatewayTransportStub) Check(ctx context.Context, request CheckRequest) (Decision, error) {
	if s.check == nil {
		return Decision{}, nil
	}
	return s.check(ctx, request)
}
