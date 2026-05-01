// Package assembly provides the Go SDK for Agent Assembly governance.
//
// It enables AI agent tool calls to be intercepted, checked against a
// governance policy gateway, and recorded for audit. The SDK manages
// sidecar connectivity, context propagation (agent ID, trace ID, run ID),
// and HTTP/gRPC middleware for outbound interception.
//
// # Quick Start
//
//	a, err := assembly.Init(ctx,
//	    assembly.WithGatewayURL("https://gateway.example.com"),
//	    assembly.WithAPIKey("my-key"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer a.Close()
//
// # Wrapping Tools
//
// Use [WrapTools] to add governance interception to a slice of tools:
//
//	wrapped := assembly.WrapTools(tools, governanceClient,
//	    assembly.WithFailClosed(true),
//	)
//
// Each wrapped tool calls [GovernanceClient.Check] before execution and
// [GovernanceClient.RecordResult] after execution.
//
// # Context Propagation
//
// Use [WithAgentID], [WithTraceID], and [WithRunID] to attach governance
// metadata to a context. These values are automatically forwarded to the
// governance gateway on every policy check.
//
// # Interceptors
//
// [HTTPMiddleware] and [UnaryClientInterceptor] / [StreamClientInterceptor]
// provide transport-level interception for outbound HTTP and gRPC calls.
package assembly
