module github.com/ai-agent-assembly/go-sdk

go 1.26.0

// Pin the build/scan toolchain to go1.26.6 so govulncheck evaluates the stdlib
// against a patched one. AAASM-5776 measured four advisories REACHABLE from this
// SDK under go1.26.5 — GO-2026-6218, GO-2026-6090, GO-2026-5972 (Sidecar.Start ->
// asn1.Unmarshal) and GO-2026-5026 (probeHealthz -> http.Client.Do) — all fixed in
// go1.26.6, which scans clean. GO-2026-5856 (crypto/tls ECH privacy leak), the
// reason for the earlier go1.26.5 pin, was fixed in go1.26.5 and stays fixed here.
// The `go` line stays at 1.26.0 (the language/min-version floor); this only raises
// the toolchain actually used to build and scan.
toolchain go1.26.6

require (
	github.com/oklog/ulid/v2 v2.1.2
	go.opentelemetry.io/otel/trace v1.45.0
	golang.org/x/tools v0.49.0
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	golang.org/x/mod v0.39.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/telemetry v0.0.0-20260811182544-a038080d80e5 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/vuln v1.6.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)

tool golang.org/x/vuln/cmd/govulncheck
