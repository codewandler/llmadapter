# llmadapter Documentation

This directory contains the detailed user, operator, library, and provider-development documentation for `llmadapter`.

The root [README.md](https://github.com/codewandler/llmadapter/blob/main/README.md) is the public landing page. This file is the documentation index.

When GitHub Pages is enabled for this repository, the docs workflow publishes this directory and uses this file as the site index.

## Start Here

- [Getting Started](GETTING_STARTED.md): fresh checkout, credentials, CLI inference, config inspection, gateway, Docker, and live smoke tests.
- [CLI](CLI.md): command reference for `providers`, `routes`, `models`, `resolve`, `infer`, `proxy`, `serve`, `smoke`, `compatibility-record`, and `conformance`.
- [Configuration](CONFIGURATION.md): JSON config, provider instances, endpoint routes, dynamic models, modeldb, capabilities, pricing, and fallback behavior.
- [Library Usage](LIBRARY_USAGE.md): Go client patterns, direct provider clients, mux client construction, tool loops, prompt caching, diagnostics, quotas, and provider extensions.

## Provider And Compatibility Reference

- [Provider Matrix](PROVIDER_MATRIX.md): supported provider endpoint types, API kinds/families, continuation behavior, transport behavior, feature coverage, and live smoke commands.
- [Use Case Matrix](USE_CASE_MATRIX.md): workload-specific compatibility evidence, including agentic-coding approval rows.
- [API Surface](API_SURFACE.md): stable public packages, extension packages, internal packages, and v1 package-boundary rules.

## Operating And Debugging

- [Troubleshooting](TROUBLESHOOTING.md): WebSocket closures, context-window surprises, transport-mode debugging, and provider error triage.
- [Architecture](ARCHITECTURE.md): practical architecture review of request flow, endpoint codecs, routing, mux/gateway behavior, continuation boundaries, transports, and known shortcomings.

## Extending llmadapter

- [Provider Development](PROVIDER_DEVELOPMENT.md): how to add provider endpoints, model API kinds/families, update registry/config behavior, and add shared smoke coverage.

## Research And Internal Notes

These documents are useful for maintainers, but are not the primary user path:

- [Claude Code Wire Diff](CLAUDE_CODE_WIRE_DIFF.md): observed Claude Code wire behavior and Claude-compatible provider parity notes.
- [Use Case Compatibility Plan](USE_CASE_COMPATIBILITY_PLAN.md): implementation plan for workload compatibility evidence and approval flows.
- [DESIGN.md](https://github.com/codewandler/llmadapter/blob/main/DESIGN.md): internal long-form target design.
- [PLAN.md](https://github.com/codewandler/llmadapter/blob/main/PLAN.md): implementation history and roadmap notes.
- [review.md](https://github.com/codewandler/llmadapter/blob/main/review.md): latest repository review snapshot.

## Examples

- [examples/README.md](https://github.com/codewandler/llmadapter/blob/main/examples/README.md): shipped config examples and adaptation notes.
- [examples/llmadapter.example.json](https://github.com/codewandler/llmadapter/blob/main/examples/llmadapter.example.json): multi-provider gateway/mux config.
- [examples/modeldb.overlay.example.json](https://github.com/codewandler/llmadapter/blob/main/examples/modeldb.overlay.example.json): modeldb overlay example used by the main config.

## Verification

Local checks:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

Live provider matrix:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -count=1 -v
```
