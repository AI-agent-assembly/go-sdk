package assembly

// WrapTools wraps all tools with AssemblyTool governance interception.
func WrapTools(toolList []Tool, client GovernanceClient, options ...Option) []Tool {
	runtimeOpts := defaultRuntimeOptions()
	for _, option := range options {
		if option != nil {
			option(&runtimeOpts)
		}
	}

	wrapped := make([]Tool, len(toolList))
	for index, tool := range toolList {
		wrapped[index] = NewAssemblyTool(tool, client, runtimeOpts)
	}

	return wrapped
}
