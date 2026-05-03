# Use-Case Compatibility Execution Plan

This plan defines the concrete work needed to answer one product question:

```text
Which provider/model/API-kind combinations are approved for a workload such as agentic coding?
```

The answer must be derived from the existing llmadapter architecture:

```text
modeldb catalog + overlays
  -> configured or auto-detected provider endpoints
  -> modeldb runtime views for configured provider instances
  -> adapterconfig model resolution
  -> route capabilities and pricing metadata
  -> live/fixture compatibility evidence
  -> use-case approval result
```

Do not add a second model resolver. CLI, gateway, mux client, and tests must continue to use `adapterconfig` plus modeldb-backed route construction.

Current implementation status:

```text
Phases 1-3 are implemented for offline compatibility inspection.
Phase 4 is implemented for agentic_coding through docs/compatibility/agentic_coding.json.
Phase 5 live workload smoke tests are implemented for the first agentic_coding matrix.
Phase 6 library filtering is implemented through adapterconfig compatibility helpers.
Phase 7 strict workload-aware selection is implemented through modeldb runtime views plus live compatibility evidence.
Phase 8 artifact-to-doc generation is implemented through `compatibility-record` and generated matrix markers.
```

## Definitions

Provider endpoint evidence answers: what does this endpoint implementation support?

Use-case compatibility answers: is this specific model through this specific provider endpoint good enough for this workload?

Initial use cases:

| Use case | Goal |
| --- | --- |
| `agentic_coding` | Coding-agent runtime with tool use, caching, structured output, and usage accounting; reasoning is optional evidence for thinking-model filters. |
| `summarization` | Simple text generation/summarization where tools, reasoning, and cache behavior are not required. |

Initial agentic-coding features:

| Feature | Requirement |
| --- | --- |
| `streaming_text` | required |
| `tools` | required |
| `tool_continuation` | required |
| `structured_output` | required |
| `reasoning` | optional |
| `prompt_caching` | required |
| `usage` | required |
| `cache_accounting` | required |
| `pricing` | preferred |
| `gateway` | optional |

`prompt_caching` means llmadapter can encode useful cache controls for the provider/API shape. `cache_accounting` means the provider reports cache write/read counters in usage.

## Phase 1: Data Model

Goal: create a small compatibility vocabulary without changing routing behavior.

Files:

- `compatibility/doc.go`
- `compatibility/types.go`
- `compatibility/profiles.go`
- `compatibility/evaluate.go`
- `compatibility/evaluate_test.go`

Steps:

1. Add `compatibility.UseCase`.
2. Add constants for `agentic_coding` and `summarization`.
3. Add `compatibility.Feature`.
4. Add constants for the initial feature set listed above.
5. Add `compatibility.RequirementLevel`: `required`, `preferred`, `optional`, `not_required`.
6. Add `compatibility.EvidenceLevel`: `live`, `fixture`, `mapped`, `modeldb`, `provider_descriptor`, `manual`, `untested`.
7. Add `compatibility.Status`: `approved`, `degraded`, `failed`, `untested`, `unavailable`.
8. Add `compatibility.Profile` with feature requirements.
9. Add built-in profiles for `agentic_coding` and `summarization`.
10. Add an evaluator that combines requirements and feature evidence into one status.
11. Test required-feature failure, preferred-feature degradation, all-required approval, and all-untested behavior.

Done criteria:

- `go test ./compatibility` passes.
- No CLI, gateway, mux, or adapterconfig behavior changes yet.
- The package does not import provider packages.

Release checkpoint:

- Commit: `feat: add use-case compatibility model`
- Release: patch prerelease/release after changelog update.

## Phase 2: Candidate Shape

Goal: evaluate model route candidates produced by the existing adapterconfig path.

Files:

- `compatibility/candidate.go`
- `adapterconfig/model_resolution.go`
- `adapterconfig/model_resolution_test.go`
- `cmd/llmadapter/main_test.go`

Steps:

1. Add a compatibility candidate type that can be built from `adapterconfig.ModelResolutionCandidate`.
2. Include input model, public model, native model, provider instance name, provider type, provider API kind, API family, source API, modeldb service, capability source, route weight, provider priority, and router capabilities.
3. Map `router.CapabilitySet` into baseline feature evidence.
4. Treat capability metadata as `provider_descriptor`, `config_override`, or `modeldb` evidence based on existing capability provenance.
5. Keep live evidence out of this phase.
6. Add tests proving candidates are built from existing `ResolveModelCandidates` output.

Done criteria:

- The compatibility layer consumes adapterconfig output only.
- No direct modeldb lookup is duplicated inside compatibility.
- `go test ./adapterconfig ./compatibility ./cmd/llmadapter` passes.

Release checkpoint:

- Commit: `feat: evaluate route candidates for use cases`
- Release after changelog update.

## Phase 3: CLI Inspection

Goal: make use-case compatibility visible to users and consumers.

Files:

- `cmd/llmadapter/main.go`
- `cmd/llmadapter/main_test.go`
- `docs/CLI.md`

Steps:

1. Add `llmadapter compatibility --use-case agentic_coding`.
2. Add `--model <name>` to filter compatibility output to one model or alias.
3. Add `--json` for machine-readable output.
4. Add `--config` support using the same config loading as `resolve` and `infer`.
5. Default to auto-detected config when `--config` is omitted.
6. Print each candidate with status, missing required features, degraded preferred features, provider instance, provider type, source API, provider API, native model, modeldb service, and evidence levels.
7. Add `--use-case` to `resolve` so commands like `llmadapter resolve anthropic/claude-haiku-4-5-20251001 --use-case agentic_coding` rank/annotate candidates by workload suitability.
8. Do not change default routing semantics yet.
9. Document examples in `docs/CLI.md`.

Done criteria:

- `llmadapter compatibility --use-case agentic_coding --model anthropic/claude-haiku-4-5-20251001` explains candidates without making provider calls.
- JSON output is stable enough for downstream tools.
- `go test ./cmd/llmadapter` passes.

Release checkpoint:

- Commit: `feat: add compatibility CLI`
- Release after changelog update.

## Phase 4: Static Evidence Artifact

Goal: separate generated/live evidence from endpoint docs.

Files:

- `docs/compatibility/agentic_coding.json`
- `docs/compatibility/summarization.json`
- `docs/USE_CASE_MATRIX.md`
- `docs/PROVIDER_MATRIX.md`
- `README.md`

Steps:

1. Add a machine-readable evidence JSON schema.
2. Seed `agentic_coding.json` from current known endpoint smoke evidence.
3. Seed `summarization.json` from current text/usage evidence.
4. Add `docs/USE_CASE_MATRIX.md` generated or manually synchronized from the JSON artifacts.
5. Update `docs/PROVIDER_MATRIX.md` to state that it is endpoint evidence, not workload approval.
6. Link `docs/USE_CASE_MATRIX.md` from `README.md`.
7. Ensure each entry distinguishes `live`, `fixture`, `mapped`, and `untested`.

Done criteria:

- The provider matrix no longer needs to imply agentic-coding readiness.
- The use-case matrix explicitly shows approved, degraded, failed, unavailable, and untested combinations.
- Docs explain why cache control support and provider cache accounting are separate.

Release checkpoint:

- Commit: `docs: add use-case compatibility matrix`
- Release after changelog update.

## Phase 5: Live Agentic-Coding Smoke

Goal: prove agentic-coding approval with outside-in tests.

Files:

- `tests/e2e/usecase_agentic_coding_test.go`
- `tests/e2e/smoke_test.go`
- `docs/PROVIDER_MATRIX.md`
- `docs/USE_CASE_MATRIX.md`

Steps:

1. Add a table-driven `TestUseCaseAgenticCoding`.
2. Use the same `TEST_INTEGRATION=1` gate as existing e2e tests.
3. Skip provider/model rows when credentials or local auth files are unavailable.
4. Resolve each candidate through adapterconfig/auto mux, not direct provider construction.
5. Run a streaming text request.
6. Run a tool call request.
7. Run a tool-result continuation request.
8. Run a structured output request.
9. Run a reasoning/thinking request where the candidate requires reasoning.
10. Run a prompt-cache write/read request where cache controls are supported.
11. Assert usage is reported.
12. Assert cache write/read counters only for providers where cache accounting is expected.
13. Emit or update evidence JSON from test results only if an explicit update flag is set.

Initial candidate rows:

| Public model | Provider candidates |
| --- | --- |
| `gpt-5.5` | `openai_responses`, `codex_responses`, `openrouter_responses` |
| `gpt-5.4` | `openai_responses`, `codex_responses`, `openrouter_responses` |
| `haiku` | `claude`, `anthropic`, `openrouter_messages` |
| `sonnet` | `claude`, `anthropic`, `openrouter_messages` |
| `opus` | `claude`, `anthropic`, `openrouter_messages` |
| `minimax-latest` | `minimax_messages` |

Short names in this historical plan are modeldb/catalog or test-harness public model names, not llmadapter-owned built-in aliases.

Done criteria:

- One command verifies the agentic-coding matrix where credentials exist:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestUseCaseAgenticCoding -count=1 -v
```

- Skips are explicit and distinguish missing credentials from unsupported features.
- Failures identify the model/provider/API-kind combination and failed feature.

Release checkpoint:

- Commit: `test: add agentic coding compatibility smoke`
- Release after changelog update.

## Phase 6: Use-Case Filtering API

Goal: let consumers such as miniagent ask for approved choices without duplicating logic.

Files:

- `compatibility/filter.go`
- `adapterconfig/compatibility.go`
- `adapterconfig/compatibility_test.go`
- `docs/LIBRARY_USAGE.md`

Steps:

1. Add an API to evaluate all `ResolveModelCandidates` results for a use case.
2. Add an API to return only `approved` candidates.
3. Add an API to include `degraded` candidates when explicitly requested.
4. Keep default routing unchanged unless the caller opts into a use-case filter.
5. Add docs showing how miniagent-style consumers can list approved agentic-coding models.

Done criteria:

- A library user can call adapterconfig once and filter candidates by use case.
- The filtering API does not instantiate hidden providers or perform live calls.
- `go test ./adapterconfig ./compatibility` passes.

Release checkpoint:

- Commit: `feat: expose use-case compatibility filters`
- Release after changelog update.

## Phase 7: Runtime-View Selection

Goal: allow a caller to request workload-aware model selection without making it implicit global routing behavior.

Files:

- `adapterconfig/compatibility_evidence.go`
- `adapterconfig/usecase_selection.go`
- `cmd/llmadapter/main.go`
- `docs/CLI.md`
- `docs/LIBRARY_USAGE.md`

Steps:

1. Load live compatibility evidence artifacts.
2. Project configured provider instances into modeldb runtime views.
3. Resolve aliases and offerings through modeldb `RuntimeView`.
4. Join runtime-view candidates with compatibility evidence by provider instance, provider API, and native model.
5. Fail closed by default unless a row is approved for the requested use case.
6. Preserve current deterministic routing when no approved-only selection is requested.
7. Add CLI inspection through `resolve --use-case ... --approved-only`.

Done criteria:

- `agentsdk`/`miniagent` can ask llmadapter for an approved provider/model/API selection without hard-coding provider names.
- Existing users that do not request approved-only selection see unchanged routing.
- `go test ./adapterconfig ./cmd/llmadapter` passes.

Release checkpoint:

- Commit: `feat: select models by use-case evidence`
- Release after changelog update.

## Phase 8: Documentation And V1 Gate

Goal: make the feature adoption-ready.

Files:

- `README.md`
- `docs/GETTING_STARTED.md`
- `docs/CLI.md`
- `docs/LIBRARY_USAGE.md`
- `docs/PROVIDER_MATRIX.md`
- `docs/USE_CASE_MATRIX.md`
- `docs/ARCHITECTURE.md`
- `PLAN.md`
- `CHANGELOG.md`

Steps:

1. Document the difference between endpoint support and workload approval.
2. Add a README section: "Choosing models for agentic coding".
3. Show CLI examples for compatibility inspection.
4. Show Go examples for compatibility filtering.
5. Update architecture docs with the compatibility layer.
6. Update PLAN current status.
7. Add generated-section markers for the compatibility result table.
8. Add `compatibility-record` so the JSON artifact rewrites the markdown table.
9. Run local test, vet, and build.
10. Run available live agentic-coding smoke tests.
11. Update CHANGELOG.
12. Cut the release.

Done criteria:

- A new user can answer "which model should I use for agentic coding?" from README/docs/CLI.
- A library consumer can programmatically get the same answer.
- The project can promote this as a v1 readiness feature.

Release checkpoint:

- Commit: `docs: document workload compatibility`
- Release after changelog update.

## Stop Conditions

Stop and ask for clarification if any of these happen:

- modeldb does not expose enough metadata to identify model owner/service for the candidate set.
- A provider advertises a required feature but live tests prove the feature does not work for the promoted model.
- Structured output semantics differ enough that `structured_output` needs to be split into multiple features before approval.
- MiniMax latest model naming cannot be resolved reliably from modeldb/overlays.
- Bedrock becomes mandatory before a Bedrock provider endpoint exists.

## Mandatory Verification Before Each Release

Run local verification:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

Run live compatibility verification when credentials are available:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestUseCaseAgenticCoding -count=1 -v
```

Before tagging:

```text
1. Update CHANGELOG.md.
2. Commit the phase.
3. Create the git tag.
4. Push commit and tag.
5. Create GitHub release notes from the phase summary and changelog entry.
```
