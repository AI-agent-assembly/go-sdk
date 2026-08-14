package ffi

import (
	"sync"
	"unsafe"
)

// AAASM-4794: this file ships a test-double binding in the regular (non-test)
// build. That looks fixable with a `_test.go` suffix or a `//go:build test`
// constraint, but neither is safe here: assembly/*_test.go (a different
// package) imports NewCapturingClient* and friends, and Go only exposes a
// package's _test.go-gated symbols to that package's own test binary, not to
// importers. Gating this file breaks `go vet ./...`/`go test ./...` for the
// assembly package (12 undefined-symbol errors, verified) unless every
// go test/vet invocation in the Makefile and CI also adds `-tags test` —
// which nothing here does today. Doing that is out of scope for this fix;
// left as a note rather than a change that trades a hygiene nit for a
// broken build.

// Registration records the arguments of a single capturingBinding.register call
// so boot tests can assert the agent was registered via the native aa_register
// path (AAASM-3404) rather than only via a SendEvent("register", ...) audit
// event.
type Registration struct {
	AgentID         string
	Name            string
	Framework       string
	GatewayEndpoint string
	TeamID          string
	ParentAgentID   string
}

type capturingBinding struct {
	// mu guards Events and recordingBinding.Queries. Both are written from
	// whatever goroutine crosses the boundary, and since AAASM-5750 that
	// includes AssemblyTool.recordOutcome's — the audit record is dispatched
	// on its own goroutine, so a test that reads the crossings races the send
	// that produced them. The race was already reachable through
	// forwardingGovernanceClient before the shipped client had a record path;
	// wiring one made it the normal case rather than the exotic one.
	mu            sync.Mutex
	handle        *byte
	Events        []string
	Registrations []Registration
	// ConnectAgentID / ConnectSDKVersion record the last connect arguments so
	// tests can assert the Go-module version was forwarded into the handshake
	// (AAASM-3683).
	ConnectAgentID    string
	ConnectSDKVersion string
	// registerStatus is the status code register returns; statusOK by default.
	// Set it to a failure code to exercise the advisory register-failure path.
	registerStatus int32
	// sendEventStatus is the status code sendEvent returns; statusOK by default.
	// Set it to a failure code to exercise the boot SendEvent-failure path.
	sendEventStatus int32
	// Disconnects counts disconnect calls so a runtime test can assert Close
	// tore the native FFI session down (AAASM-4832), or that boot did so on the
	// SendEvent-failure path (AAASM-4843).
	Disconnects int
}

func (b *capturingBinding) connect(_, agentID, sdkVersion string) (unsafe.Pointer, int32) {
	b.ConnectAgentID = agentID
	b.ConnectSDKVersion = sdkVersion
	b.handle = new(byte)
	return unsafe.Pointer(b.handle), statusOK
}

func (b *capturingBinding) sendEvent(_ unsafe.Pointer, _, details string) int32 {
	b.mu.Lock()
	b.Events = append(b.Events, details)
	status := b.sendEventStatus
	b.mu.Unlock()

	if status != statusOK {
		return status
	}
	return statusOK
}

// snapshotEvents returns a copy of the events recorded so far.
//
// A copy, not the slice: handing back the live one moves the race to the
// caller's range loop, which is exactly where the detector found it.
func (b *capturingBinding) snapshotEvents() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]string(nil), b.Events...)
}

func (b *capturingBinding) disconnect(_ unsafe.Pointer) int32 {
	b.Disconnects++
	return statusOK
}

// register records the registration and returns a deterministic policy id so the
// advisory boot-path register call succeeds in tests. When registerStatus is set
// to a failure code it records the attempt but returns that status with no policy
// id, exercising the advisory register-failure path.
func (b *capturingBinding) register(_ unsafe.Pointer, agentID, name, framework, gatewayEndpoint, teamID, parentAgentID string) (string, int32) {
	b.Registrations = append(b.Registrations, Registration{
		AgentID:         agentID,
		Name:            name,
		Framework:       framework,
		GatewayEndpoint: gatewayEndpoint,
		TeamID:          teamID,
		ParentAgentID:   parentAgentID,
	})
	if b.registerStatus != statusOK {
		return "", b.registerStatus
	}
	return "policy-" + agentID, statusOK
}

// NewCapturingClient returns an FFI client backed by an in-memory binding that
// records every SendEvent's details payload. The second return value points to
// the captured slice — callers inspect it to assert on emitted events.
func NewCapturingClient() (*Client, *[]string) {
	b := &capturingBinding{}
	return NewClient(b), &b.Events
}

// NewCapturingClientRecordingDisconnect returns an FFI client whose connect and
// register succeed and whose binding counts disconnect calls. The returned *int
// lets a runtime test assert that Close teared down the native FFI session
// exactly once (AAASM-4832).
func NewCapturingClientRecordingDisconnect() (*Client, *int) {
	b := &capturingBinding{}
	return NewClient(b), &b.Disconnects
}

// NewCapturingClientFailingSendEvent returns an FFI client whose connect and
// register succeed but whose SendEvent fails, and whose binding counts disconnect
// calls. Boot opens the native session (connect ok) and then hits the
// SendEvent-failure path; the returned *int lets a runtime test assert boot tore
// the session down (Disconnect) rather than leaking it (AAASM-4843).
func NewCapturingClientFailingSendEvent() (*Client, *int) {
	b := &capturingBinding{sendEventStatus: statusIPCError}
	return NewClient(b), &b.Disconnects
}

// NewCapturingClientWithRegistrations is like NewCapturingClient but also exposes
// the captured native registrations so boot tests can assert aa_register was
// called on Init (AAASM-3404).
func NewCapturingClientWithRegistrations() (*Client, *[]string, *[]Registration) {
	b := &capturingBinding{}
	return NewClient(b), &b.Events, &b.Registrations
}

// ConnectArgs exposes the arguments the last connect call received so a boot test
// can assert that Init forwarded the agent id and the Go-module SDK version into
// the runtime handshake (AAASM-3683). The fields track the capturingBinding's
// recorded connect arguments by pointer, so they reflect the value at the time
// of assertion.
type ConnectArgs struct {
	AgentID    *string
	SDKVersion *string
}

// NewCapturingClientWithConnectArgs is like NewCapturingClient but also exposes
// the arguments the binding's connect received, so a boot test can assert that
// Init forwarded the agent id and the Go-module SDK version into the handshake
// (AAASM-3683).
func NewCapturingClientWithConnectArgs() (*Client, ConnectArgs) {
	b := &capturingBinding{}
	return NewClient(b), ConnectArgs{AgentID: &b.ConnectAgentID, SDKVersion: &b.ConnectSDKVersion}
}

// NewCapturingClientFailingRegister returns a capturing client whose register
// fails with GATEWAY_UNREACHABLE so the boot path's advisory register-failure
// branch (log + proceed unregistered) can be exercised. The captured
// registrations and events are still exposed for assertions.
func NewCapturingClientFailingRegister() (*Client, *[]string, *[]Registration) {
	b := &capturingBinding{registerStatus: statusGatewayUnreachable}
	return NewClient(b), &b.Events, &b.Registrations
}

// NewCapturingClientWithRegisterStatus is like NewCapturingClientFailingRegister
// but lets the caller pick the failure status code so the advisory boot path can
// be exercised for both REGISTER_FAILED and GATEWAY_UNREACHABLE (AAASM-3404):
// aa_register is fail-closed at the native boundary, but both outcomes are
// advisory at the SDK layer. Pass statusOK to get a succeeding register.
func NewCapturingClientWithRegisterStatus(status int32) (*Client, *[]string, *[]Registration) {
	b := &capturingBinding{registerStatus: status}
	return NewClient(b), &b.Events, &b.Registrations
}

// RegisterFailedStatus is the native status code aa_register returns when the
// gateway rejects the registration (e.g. an invalid did:key). Exported so the
// assembly boot tests can drive the advisory REGISTER_FAILED branch without
// reaching into the unexported status constants.
const RegisterFailedStatus = statusRegisterFailed

// GatewayUnreachableStatus is the native status code aa_register returns when it
// cannot reach the gateway gRPC endpoint. Exported for the same reason as
// RegisterFailedStatus.
const GatewayUnreachableStatus = statusGatewayUnreachable

// denyingRegisteringBinding records native registrations like capturingBinding
// and additionally answers policy queries (policyQuerier) with a fixed decision.
// It lets a boot test drive the full Init -> aa_register -> WrapTools ->
// aa_query_policy path so a runtime DENY blocks a wrapped tool end to end.
type denyingRegisteringBinding struct {
	capturingBinding
	queryDecision int32
	queryReason   string
}

func (b *denyingRegisteringBinding) queryPolicy(_ unsafe.Pointer, _, _, _, _ string) (int32, string, int32) {
	return b.queryDecision, b.queryReason, statusOK
}

// NewCapturingClientDenying returns a capturing client whose register succeeds
// and whose policy queries return DENY with the given reason, so a boot test can
// assert that a reachable runtime's DENY blocks a wrapped tool after Init. The
// captured registrations are exposed so the test can also confirm aa_register ran.
func NewCapturingClientDenying(reason string) (*Client, *[]Registration) {
	b := &denyingRegisteringBinding{queryDecision: DecisionDeny, queryReason: reason}
	return NewClient(b), &b.Registrations
}

// NewCapturingClientAllowing returns a capturing client whose register succeeds
// and whose policy queries return ALLOW, so a boot test can assert that a
// reachable runtime lets a permitted wrapped tool run after Init.
func NewCapturingClientAllowing() (*Client, *[]Registration) {
	b := &denyingRegisteringBinding{queryDecision: DecisionAllow}
	return NewClient(b), &b.Registrations
}

// PolicyQuery records one queryPolicy crossing of the native boundary.
type PolicyQuery struct {
	AgentID    string
	ActionType string
	ToolName   string
	ArgsJSON   string
}

// recordingBinding captures every crossing of the native boundary in one place:
// the policy queries AND the events. capturingBinding already records events but
// discards the queryPolicy arguments, so a test can see that a check crossed but
// not what it carried — which is exactly what an audit measurement needs on both
// sides (AAASM-5731).
type recordingBinding struct {
	capturingBinding
	Queries       []PolicyQuery
	queryDecision int32
	queryReason   string
}

func (b *recordingBinding) queryPolicy(_ unsafe.Pointer, agentID, actionType, toolName, argsJSON string) (int32, string, int32) {
	b.mu.Lock()
	b.Queries = append(b.Queries, PolicyQuery{
		AgentID:    agentID,
		ActionType: actionType,
		ToolName:   toolName,
		ArgsJSON:   argsJSON,
	})
	b.mu.Unlock()

	return b.queryDecision, b.queryReason, statusOK
}

// snapshotQueries returns a copy of the policy queries recorded so far.
func (b *recordingBinding) snapshotQueries() []PolicyQuery {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]PolicyQuery(nil), b.Queries...)
}

// NativeCrossings exposes everything that crossed the native FFI boundary during
// a run: the policy queries and the event payloads.
//
// binding.sendEvent is the SDK's ONLY outbound event channel (see the binding
// interface in client.go), so "nothing was recorded" is decidable here and
// nowhere else. The Queries side is the positive control that makes an empty
// Events slice admissible: without it, zero events is indistinguishable from a
// probe that never ran (AAASM-5731).
//
// The two fields are snapshot FUNCTIONS rather than slice pointers because the
// audit record is dispatched on its own goroutine (AAASM-5750): a test reading a
// live slice races the send it is trying to observe, which `go test -race`
// reports as a genuine data race. Each call returns a copy taken under the
// binding's lock.
type NativeCrossings struct {
	Queries func() []PolicyQuery
	Events  func() []string
}

// NewRecordingClient returns a capturing client whose register succeeds, whose
// policy queries return the supplied decision, and which exposes every crossing
// of the native boundary — queries and events alike.
func NewRecordingClient(decision int32, reason string) (*Client, NativeCrossings) {
	b := &recordingBinding{queryDecision: decision, queryReason: reason}
	return NewClient(b), NativeCrossings{Queries: b.snapshotQueries, Events: b.snapshotEvents}
}
