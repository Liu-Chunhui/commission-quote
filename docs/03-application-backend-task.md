# Task B: Application Backend

Implement the browser-facing API and outbound vendor call. Read the [shared contract and acceptance rules](01-development-approach.md), [Web API](../api/webapi.openapi.json), and [Vendor API](../test/mock/commissionquote/api/commissionquote.openapi.json).

**Status:** Task B is implemented and has passed independent acceptance. Both `GET /health` and `POST /api/quotes` are wired through the running application. Real browser → application → mock-service integration and the combined challenge handover remain separate work; they have not been verified by the checks below.

## Scope

Own `cmd/app/`, `internal/app/`, `internal/config/`, `internal/httpapi/`, `internal/integration/commissionquote/`, `confg/`, and `api/webapi.openapi.json`. The root Go module is `commissionquote`.

Do not modify A's service or C's UI. Keep the backend thin: no commission calculation, quote ID generation, failure simulation, idempotency store, repository layer, or single-implementation client interface. Coordinate API changes with the integration owner.

## Implementation context

- Load application settings in `internal/config/config.go` using the standard library; keep its tests in `internal/config/config_test.go`. Select the JSON file with `--config`; follow the [shared runtime configuration](01-development-approach.md#runtime-configuration). Configuration errors are logged once by the loader before startup exits.
- Validate at the incoming HTTP boundary. Forward the payload and idempotency key with the private vendor API key, using an actual `http.Client`.
- Keep commission quote calls in `internal/integration/commissionquote/`. `NewQuoteClient(httpClient, baseURL, apiKey)` accepts an HTTP client configured by the caller with a three-second timeout and redirects disabled (`http.ErrUseLastResponse`); `GenerateQuote(ctx, key, input)` accepts already-validated input and returns a validated quote or a safe error. Startup loads `Config.APIKey` through the configuration loader's `apiKeyFile` setting; the client does not read credential files.
- Apply the shared timeout/cancellation and public error boundary. Check external response decoding and required fields, close response bodies, and return quote fields unchanged; do not recalculate vendor business rules.
- Handle the quote service's HTTP status and documented `error.code` internally, including invalid request, authentication failure, conflict, rate limit, unavailable service, and internal error. Record the failed operation and safe cause in logs; unknown codes, malformed error bodies, connection failures, invalid success payloads, and timeouts also produce the same public failure. Never forward upstream status, headers, code, message, or raw body. No per-error retry behavior is required.
- Missing required API key stops startup. Keep runtime wiring in `cmd/app/main.go`, routing and request logging in `internal/app/`, configuration in `internal/config/`, and HTTP handlers in `internal/httpapi/`. Keep tests beside the implementation.
- Health handling lives in `internal/httpapi/health.go`, quote validation/handling in `quote.go`, and JSON response writing in `http.go`.

## Acceptance

Test the real handler/client against fixed-response or delayed `httptest` vendor handlers. Do not start or reproduce A's service.

| Check | Pass condition |
| --- | --- |
| Shared input/header validation | Rejected requests never call the vendor |
| Outbound request | Method, path, payload, content type, private API key, and unchanged idempotency key match the vendor contract |
| Success and repeat requests | Vendor quote fields are returned unchanged; the same incoming key is forwarded on every attempt |
| Error isolation | Exercise every documented quote-service error, unknown codes/statuses, malformed error/success bodies, missing/wrong success fields, and connection failure; all return the identical public 500 payload while logs retain a safe internal cause |
| Mock trigger inputs | Both pass ordinary validation and are forwarded unchanged; outcomes come from fixed vendor responses |
| Timeout/cancellation | Deadline produces the same generic 500; browser cancellation cancels the outbound request; no detached work or automatic retry |

Use synthetic test credentials. A shorter timeout is acceptable through existing standard client construction; do not add a custom clock, interface, or test-only constructor parameter.

**Done:** these checks, the coverage target below, the [Go and logging rules](../AGENTS.md#backend), and the [common handoff](01-development-approach.md#common-acceptance-and-handoff) pass without A or C. Verify success/failure logs after each backend change.

## Commands

Use Go 1.26.4 or newer, as required by the root `go.mod`. Run from the repository root. Tests use temporary synthetic credentials; normal startup requires the private file referenced by `apiKeyFile` (both profiles point to `test/mock/data/API_KEY`):

```sh
make server test
go test -race ./...
go vet ./...
go tool cover -html=gen/coverage.out
make server build
make server dev
```

The build writes `bin/app`; tests write `gen/coverage.out`. Start the built server with `./bin/app --config confg/ci.json` for the CI profile. Omitting `--config` selects the dev profile.

Run `make clean` to remove `bin/` and `gen/` from both the repository root and `test/mock/commissionquote/`, legacy root build/coverage outputs (`app`, `coverage.out`, `coverage.html`), and the frontend's generated files and dependencies. It also stops this worktree's Vite processes; source files, configuration, and private key files are preserved.

Production Go files target **greater than 90% statement coverage per file**, excluding `main.go` and test code. Aggregate covered/total statements per file; package averages are insufficient. Explain shortfalls without adding test-only production abstractions. This root test command excludes A's independent module.

## Integration client acceptance

Run `go test ./internal/integration/commissionquote -count=1 -v` without a running vendor. The tests call the real client against controlled `httptest` HTTP servers:

| Input or controlled dependency | Expected outcome |
| --- | --- |
| `4001/36/low`, response `opaque-id/0.01/40.01` | Exact request fields and headers; unchanged quote; one call for each repeated attempt |
| All risk bands and mock trigger amounts | Forward normally and preserve supplied totals without recalculation |
| Each documented 400/401/409/429/500/503 error, unknown status/code, malformed or invalid response | Empty quote and `unable to generate a quote`; safe internal diagnostic only |
| Missing/wrong synthetic API key, connection failure, truncated body, or redirect | Fail without exposing credentials or following redirects |
| Delayed headers/body or canceled context | Timeout/cancellation reaches the outbound request; no automatic retry, including transport replay |
| Success and failure with request ID `request-42` | INFO start/completion with status and duration; exactly one ERROR on failure; no raw upstream detail or key in logs |

The downstream client is wired into `/api/quotes` through startup configuration. Real-service end-to-end integration remains a separate phase. The client and handler tests were derived with AI assistance from the supplied OpenAPI contracts and shared application rules.

## Web API acceptance and handoff

`GET /health` and `POST /api/quotes` are implemented. Startup loads the configured URL and key file, creates the HTTP client with a three-second timeout and redirects disabled, and passes it to the quote client. Provision the private file referenced by `confg/dev.json` before running `make server dev`.

Run `go test ./internal/httpapi -run TestQuote -count=1 -v` for independent endpoint acceptance. These tests exercise the real router, handler, and integration client with controlled HTTP dependencies:

| Input or dependency | Expected and verified result |
| --- | --- |
| Valid boundary values, all risk bands, `4001`, and both mock trigger amounts | 200; forward fields/key unchanged and return the supplied quote without recalculation; repeated requests each call the dependency |
| Missing/invalid key, wrong content type, malformed/trailing JSON, missing/null/wrong-type fields, invalid ranges or risk labels | 400 `INVALID_REQUEST`; no downstream calls |
| Documented downstream errors, unknown status/code, malformed or incomplete response, connection failure, redirect | Identical 500 `INTERNAL_ERROR` payload; no retries or credential forwarding to redirects |
| Delayed headers/body or incoming cancellation | Generic 500; outgoing request canceled |
| Success, validation failure, downstream failure, or response write failure | Correlated request logs with final status/timing; one error record per detected failure and no private details |

Verification on 2026-09-21 passed: `make server test`, `go test -race ./...`, `go vet ./...`, `make server build`, `make web build`, and all 16 browser tests (`make web test`, with intercepted API responses). Every executable production file under `internal/` has 100% statement coverage, including `internal/httpapi/http.go`; `type.go` contains only type declarations and `cmd/app/main.go` is exempt. The earlier built-process smoke check with temporary configuration and a controlled HTTP dependency also verified health, success, validation, generic failures, blocked redirects, the real three-second timeout, and UTC logs. These checks do not establish real mock-service or browser-to-backend end-to-end integration.
