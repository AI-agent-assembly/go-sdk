// Package assembly provides the Go SDK for Agent Assembly governance.
//
// It enables AI agent tool calls to be intercepted and checked against a
// governance policy gateway before they run. The SDK manages sidecar
// connectivity, context propagation (agent ID, trace ID, run ID), and HTTP/gRPC
// middleware for outbound interception.
//
// It keeps no audit trail of its own. A governed call's outcome is offered to
// [GovernanceClient.RecordResult], and the client this SDK ships hands it to the
// runtime over the native event channel — a handoff, not a retention guarantee:
// the send is unacknowledged and the dispatch is not joined before shutdown, so
// this SDK cannot tell you the record survived, and does not claim it did. What
// happens downstream of the handoff is tracked as AAASM-5783 and is unfixed. With
// no runtime connected there is no channel at all and the call produces no audit
// evidence. Enforcement is unaffected either way. [Init] warns in that case and
// [Assembly.AuditSink] reports which case a run is in — see
// [AuditSinkDisposition] (AAASM-5750).
//
// # Quick Start
//
//	a, err := assembly.Init(ctx,
//	    assembly.WithGatewayURL("https://gateway.example.com"),
//	    assembly.WithAPIKey("my-key"),
//	    // Point at a running gateway's gRPC address so Init can reach the
//	    // registration path; without it (or WithSidecarBinary) Init returns
//	    // ErrSidecarUnavailable.
//	    assembly.WithSidecarAddress("127.0.0.1:50051"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer a.Close()
//
// Gateway registration (the Ed25519 possession-proof handshake) runs only under
// the opt-in native cgo binding — build with `-tags aa_ffi_go` and
// CGO_ENABLED=1. The default pure-Go build has no native transport, so it does
// not self-register; [WithSidecarAddress] makes the path reachable, the native
// binding makes it enforce-grade.
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
// [GovernanceClient.RecordResult] after execution, on the denied path as well as
// the executed one.
//
// Where that record goes is a property of the client, not of the wrapper. The
// client this SDK ships hands it to a connected runtime and has nowhere to send
// it without one; for a client you supply, this SDK makes no claim either way.
// No path here reaches ADR 0033 §6 *Observed*. Init warns when no record can be
// sent and [Assembly.AuditSink] reports which case a run is in — see
// [AuditSinkDisposition] (AAASM-5750).
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
//
// # Audit Events
//
// [AuditEvent] is the Go-side mirror of the gateway's audit-trail event
// shape, including the hierarchical [CallStackNode] tree that records
// LLM / tool / result steps for inline rendering in the dashboard's
// Live Ops view. Use [CallStackNodeKindLLM], [CallStackNodeKindTool],
// and [CallStackNodeKindResult] as the canonical Kind values.
package assembly
