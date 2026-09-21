# Task B: Application Backend

Implement the browser-facing API and outbound vendor call. Read the [shared contract and acceptance rules](01-development-approach.md), [Web API](../api/webapi.openapi.json), and [Vendor API](../test/mock/commissionquote/api/commissionquote.openapi.json).

## Scope

Own `cmd/app/` and `api/webapi.openapi.json`. Start after the integration owner prepares the root module.

Do not modify A's service or C's UI. Keep the backend thin: no commission calculation, quote ID generation, failure simulation, idempotency store, repository layer, or single-implementation client interface. Coordinate API changes with the integration owner.

## Implementation context

- Validate at the incoming HTTP boundary. Forward the payload and idempotency key with the private vendor API key, using an actual `http.Client`.
- Apply the shared timeout/cancellation and public error boundary. Check external response decoding and required fields, close response bodies, and return quote fields unchanged; do not recalculate vendor business rules.
- Handle the quote service's HTTP status and documented `error.code` internally, including invalid request, authentication failure, conflict, rate limit, unavailable service, and internal error. Record the failed operation and safe cause in logs; unknown codes, malformed error bodies, connection failures, invalid success payloads, and timeouts also produce the same public failure. Never forward upstream status, headers, code, message, or raw body. No per-error retry behavior is required.
- Missing required API key stops startup. Keep runtime wiring in `main.go`; business logic belongs in the other files.

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

After implementation, run from the repository root, with the API key supplied privately:

```sh
gofmt -w cmd/app
go test ./... -coverprofile=/tmp/commission-app-coverage.out
go tool cover -html=/tmp/commission-app-coverage.out
go build ./cmd/app
go run ./cmd/app
```

Production Go files target **greater than 90% statement coverage per file**, excluding `main.go` and test code. Aggregate covered/total statements per file; package averages are insufficient. Explain shortfalls without adding test-only production abstractions. This root test command excludes A's independent module.
