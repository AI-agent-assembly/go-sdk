package assembly

// Config contains the user-supplied bootstrap settings.
type Config struct {
	Gateway        string
	APIKey         string
	SidecarAddress string
}

// InitAssembly initializes the SDK runtime.
func InitAssembly(cfg Config) error {
	_ = cfg
	return nil
}
