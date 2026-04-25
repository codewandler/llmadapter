// Package unified defines llmadapter's provider-neutral request, response, event,
// tool, content, usage, cache, extension, and client primitives.
//
// This package is the primary public API for in-process consumers. Provider
// clients, the mux client, and gateway adapters all exchange data through
// unified.Request values and streams of unified.Event values.
package unified
