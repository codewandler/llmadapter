// Package compatibility evaluates model/provider/API candidates against
// workload profiles such as agentic coding.
//
// The package does not resolve models by itself. Callers should feed it route
// candidates produced by adapterconfig so modeldb, aliases, provider endpoint
// construction, and routing metadata stay centralized.
package compatibility
