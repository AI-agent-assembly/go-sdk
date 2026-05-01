package assembly

import (
	"context"

	"github.com/agent-assembly/go-sdk/internal/ffi"
)

// Assembly is the runtime entrypoint for governance-enabled execution.
type Assembly struct {
	opts             runtimeOptions
	sidecar          SidecarClient
	sidecarConnector func(context.Context, string) (SidecarClient, error)
	ffiClient        *ffi.Client
	managedSidecar   *Sidecar
}

// newAssembly builds an Assembly runtime from functional options.
func newAssembly(options ...Option) *Assembly {
	opts := defaultRuntimeOptions()
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}

	return &Assembly{
		opts:             opts,
		sidecarConnector: sidecarConnector,
		ffiClient:        ffi.NewDefaultClient(),
	}
}

// boot boots the runtime and prepares governance integrations.
func (a *Assembly) boot(ctx context.Context) error {
	if err := validateRuntimeOptions(a.opts); err != nil {
		return err
	}

	if a.opts.sidecarBinary != "" {
		sc := NewSidecar(a.opts.sidecarBinary, a.opts.sidecarAddress)
		if err := sc.Start(ctx); err != nil {
			return err
		}
		if err := sc.Healthy(ctx); err != nil {
			_ = sc.Stop()
			return err
		}
		a.managedSidecar = sc
	}

	if a.opts.sidecarAddress != "" && a.ffiClient != nil {
		if err := a.ffiClient.Connect(a.opts.sidecarAddress); err == nil {
			return nil
		}
	}

	sidecar, err := a.sidecarConnector(ctx, a.opts.sidecarAddress)
	if err != nil {
		return err
	}

	a.sidecar = sidecar
	return nil
}

// Close shuts down runtime resources.
func (a *Assembly) Close() error {
	if a.managedSidecar != nil {
		if err := a.managedSidecar.Stop(); err != nil {
			return err
		}
		a.managedSidecar = nil
	}
	a.sidecar = nil
	return nil
}
