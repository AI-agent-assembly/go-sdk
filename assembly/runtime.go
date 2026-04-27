package assembly

import "context"

// Assembly is the runtime entrypoint for governance-enabled execution.
type Assembly struct {
	opts             runtimeOptions
	sidecar          SidecarClient
	sidecarConnector func(context.Context, string) (SidecarClient, error)
}

// NewAssembly builds an Assembly runtime from functional options.
func NewAssembly(options ...Option) *Assembly {
	opts := defaultRuntimeOptions()
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}

	return &Assembly{
		opts:             opts,
		sidecarConnector: sidecarConnector,
	}
}

// Init boots the runtime and prepares governance integrations.
func (a *Assembly) Init(ctx context.Context) error {
	if err := validateRuntimeOptions(a.opts); err != nil {
		return err
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
	a.sidecar = nil
	return nil
}
