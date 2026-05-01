package assembly_test

import (
	"context"
	"fmt"

	"github.com/agent-assembly/go-sdk/assembly"
)

// demoTool is a minimal Tool implementation for demonstration purposes.
type demoTool struct{}

func (demoTool) Name() string                                     { return "calculator" }
func (demoTool) Description() string                              { return "evaluates expressions" }
func (demoTool) Call(_ context.Context, _ string) (string, error) { return "42", nil }

// This example demonstrates the primary Init → WrapTools flow.
// In production, Init connects to a running governance sidecar.
func ExampleInit() {
	ctx := context.Background()

	a, err := assembly.Init(ctx,
		assembly.WithGatewayURL("https://gateway.example.com"),
		assembly.WithAPIKey("my-key"),
	)
	if err != nil {
		fmt.Println("init requires a running sidecar:", err)
		return
	}
	defer func() { _ = a.Close() }()

	tools := []assembly.Tool{demoTool{}}
	_ = assembly.WrapTools(tools, nil,
		assembly.WithFailClosed(true),
	)
}
