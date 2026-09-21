# Task A: Mock Quote Vendor

Implement the independent vendor service. Read the [shared contract and acceptance rules](01-development-approach.md) and [Vendor OpenAPI](../test/mock/quotevendor/api/commissionquote.openapi.json); this document adds only Task A context.

## Scope

Own `test/mock/quotevendor/`. Initialize its `quotevendor` Go module only if missing. Do not import application packages or implement the application's client/UI.

## Implementation context

- Read the selected profile once with standard `flag` and JSON packages. `-config` defaults to `config/dev.json`; `-config` paths are relative to the service working directory. Do not merge profiles or add environment overrides. Read the required `port` (integer 1–65535) from this file. Resolve `apiKeyFile` relative to the profile directory, or use an absolute mounted path. Read the key once and trim surrounding whitespace. Restart after configuration or key changes.
- Missing/invalid port, missing `apiKeyFile`, unreadable/empty key file, unreadable/malformed configuration, unsupported/missing mode, or missing/invalid rate in random mode stops startup with a safe error. Select the mode before decoding its settings so loanAmount mode can ignore any supplied rate.
- Handler order: authenticate → validate → check/bind key or replay → simulate failure → calculate and store success → respond.
- A standard mutex around the in-memory map is sufficient to make key checking, binding, simulation/calculation, and successful-result storage atomic. Generate an opaque ID only for a new successful quote.

## Acceptance

Exercise the real HTTP handler directly; use the shared validation cases and these service-specific checks:

| Check | Pass condition |
| --- | --- |
| Calculation | Shared quote examples and all risk bands produce the expected rates/totals |
| Authentication, including replay | Missing/wrong API key is rejected before validation/simulation |
| Idempotency | Sequential/concurrent duplicates, including reordered JSON fields, return one quote; changed amount/term/risk under the bound key conflicts, even after failure |
| Key lifecycle | Invalid input reserves no key; failed responses can be retried; different caller-supplied keys create separate quotes; restart clears stored results |
| Failure profiles | Both profiles match shared behavior; random rates 0/1 are deterministic; trigger amounts have no special meaning in random mode; unlisted amounts succeed in loanAmount mode |
| Precedence | Invalid authentication, input, or key reuse wins over failure simulation |
| Configuration | Default/explicit file selection works; invalid startup settings fail; loanAmount mode accepts omitted, null, string, or out-of-range rate values |
| Invalid idempotency headers | Missing or invalid headers match the vendor specification |

**Done:** these checks, the [Go and logging rules](../AGENTS.md#backend), and the [common handoff](01-development-approach.md#common-acceptance-and-handoff) pass without B or C. Verify success/failure logs after each backend change. Startup/restart checks may be manual commands; no test-only production seam is needed.

## Commands

Use Go 1.24 or newer for this independent module. Run its checks from `test/mock/quotevendor/`; root Go tests do not include it. Functional tests are required, but the mock service has no high-coverage target. Provision the private key file referenced by the profile before startup:

```sh
gofmt -w cmd internal/app internal/httpapi internal/config
go test -race ./...
go build -o /tmp/quotevendor ./cmd
go run ./cmd -config config/dev.json
```

Use `-config config/ci.json` to select the CI profile.

From the repository root, `make quote dev` starts the service with the dev profile and `make quote build` writes `test/mock/quotevendor/bin/quote`. Run the built executable from the mock module directory so the default profile resolves correctly. `make clean` also removes this service's `bin/` and `gen/` directories. There is no `make quote test`; run the Go tests from the mock module as shown above.

`make dev up` starts all three components using the existing shared key referenced by the dev profiles. See the [combined startup instructions](../test/mock/quotevendor/README.md#start-the-complete-app).

## Task A handoff — 2026-09-21

The independent service is implemented. Startup and manual HTTP steps are in [README](../test/mock/quotevendor/README.md#run-the-mock-separately); its standalone specification is `test/mock/quotevendor/api/commissionquote.openapi.json`.

| Reproducible check | Expected | Actual |
| --- | --- | --- |
| `go test -race ./...` | Configuration, all risk bands, validation, authentication, sequential/concurrent replay, conflicts, key lifecycle, failure modes, health, and request-log tests pass | Passed |
| `go vet ./...`; `go build -o /tmp/quotevendor ./cmd` | No diagnostics; executable builds | Passed |
| Default startup; explicit dev/CI and temporary random 0/1 profiles | Correct profile behavior; unauthenticated health remains 200 `ok` | Passed through live HTTP; CI outcome was not asserted probabilistically |
| 10000/36/medium; repeat; 24 concurrent duplicates | 0.02 and 200; identical quote for each reused key | Passed through live HTTP |
| Changed term under an existing key; restart and repeat original request | 409 before simulation; restart generates a new quote ID | Passed through live HTTP |
| Missing/empty key file; unreadable/malformed config; random rate 2 | Nonzero startup exit and one safe error | Passed |
| Follow live success and failure logs | Request IDs, ordered lifecycle, final status, duration, one ERROR for each rejected request, no API key | Passed |

The HTTP tests use the actual service handler and controlled failure modes, without application or frontend dependencies. Repeated deterministic-failure tests verify retained binding and retry acceptance; they do not demonstrate a random failure followed by success on the same key. Failure responses are not cached in the implementation. No production delay endpoint was added; application timeout acceptance belongs to Task B. Browser and real-service integration remain outside Task A.
