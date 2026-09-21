# Task A: Mock Quote Service

Implement the independent vendor service. Read the [shared contract and acceptance rules](01-development-approach.md) and [Vendor OpenAPI](../test/mock/commissionquote/api/mock-commissionquote.openapi.json); this document adds only Task A context.

## Scope

Own `test/mock/commissionquote/`. Initialize its `commissionquote/mock` Go module only if missing. Do not import application packages or implement the application's client/UI.

## Implementation context

- Read the selected profile once with standard `flag` and JSON packages. `-config` defaults to `config/dev.json`; paths are relative to the service working directory. Do not merge profiles or add failure-mode environment overrides. Restart after changes.
- Missing API key, unreadable/malformed configuration, unsupported/missing mode, or missing/invalid rate in random mode stops startup with a safe error. Select the mode before decoding its settings so loanAmount mode can ignore any supplied rate.
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

Run this independent module's checks from `test/mock/commissionquote/` after implementation; root Go tests do not include it. Functional tests are required, but the mock service has no high-coverage target. Supply the API key privately for startup:

```sh
gofmt -w cmd/vendor internal/vendor
go test -race ./...
go build ./cmd/vendor
go run ./cmd/vendor -config config/dev.json
```

Use `-config config/ci.json` to select the CI profile.
