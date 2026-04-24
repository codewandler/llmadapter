package adapterconfig

// DefaultModelDBAliases returns llmadapter's opinionated provider-local model
// aliases. Callers can append or override these through Config.ModelDB.Aliases
// or AutoOptions.ModelDBAliases.
func DefaultModelDBAliases() []ModelDBAliasConfig {
	return []ModelDBAliasConfig{
		{Name: "haiku", ServiceID: "anthropic", WireModelID: "claude-haiku-4-5-20251001"},
		{Name: "sonnet", ServiceID: "anthropic", WireModelID: "claude-sonnet-4-6"},
		{Name: "opus", ServiceID: "anthropic", WireModelID: "claude-opus-4-6"},
		{Name: "haiku", ServiceID: "openrouter", WireModelID: "anthropic/claude-haiku-4.5"},
		{Name: "sonnet", ServiceID: "openrouter", WireModelID: "anthropic/claude-sonnet-4.6"},
		{Name: "opus", ServiceID: "openrouter", WireModelID: "anthropic/claude-opus-4.6"},
	}
}
