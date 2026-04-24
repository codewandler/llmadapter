# Agent Notes

Keep changes small, buildable, and covered by tests.

## Required Checks

Before committing implementation changes, run:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

For provider behavior changes, also run targeted live e2e tests when credentials are available:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream|TestSmokeToolUse|TestSmokeToolResultContinuation|TestGatewaySmoke' -count=1 -v
```

## Architecture Rules

- Route to provider endpoints, not just providers.
- Keep `ProviderName`, `APIKind`, `ApiFamily`, `Client`, and `CapabilitySet` explicit.
- Treat default capabilities as endpoint-family defaults; use config overrides or tests before advertising model-specific features.
- Treat model catalog data as metadata for configured provider endpoints, not as hidden provider/client construction.
- Keep pricing/cost enrichment and stateful conversation/session handling outside the stateless gateway/router path.
- Do not collapse OpenRouter-style multi-surface providers into one API kind.
- Preserve unsupported/lossy fields via warnings or namespaced `unified.Request.Extensions` rather than silently pretending support.
- Prefer stream-first provider clients; endpoint codecs can collect when they need non-streaming responses.

## Editing Rules

- Update `PLAN.md` when status, gaps, or verification commands change.
- Update `README.md` when public setup, env vars, or gateway config changes.
- Add focused unit tests for codecs/decoders and shared e2e smoke coverage for new provider clients.
- Keep dependencies minimal; prefer stdlib unless there is a clear reason.
