module github.com/ai-agent-assembly/go-sdk

go 1.26.0

// Pin the build/scan toolchain to go1.26.5 so govulncheck evaluates crypto/tls
// against the patched stdlib — GO-2026-5856 (ECH privacy leak) is fixed in
// go1.26.5. The `go` line stays at 1.26.0 (the language/min-version floor); this
// only raises the toolchain actually used to build and scan.
toolchain go1.26.5

require (
	github.com/oklog/ulid/v2 v2.1.2
	go.opentelemetry.io/otel/trace v1.44.0
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/telemetry v0.0.0-20260710170516-c325552849a7 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	golang.org/x/vuln v1.6.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)

tool golang.org/x/vuln/cmd/govulncheck
