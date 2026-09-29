# BOOST10 Task: Mock

**Status:** Planned. Provide independent base-quote verification for the [shared BOOST10 draft](01-development-approach.md#boost10-broker-loyalty-bonus-planned). The application owns the incentive calculation; the mock supplies the original commission.

## Scope

Own `test/mock/quotevendor/` and this task document. Reuse `internal/httpapi/quote.go`, `handler_test.go`, and the existing vendor OpenAPI. Do not modify the application or web.

## Changes

- Preserve the vendor's three-field request and response contract, rates, authentication, validation, failure modes, and in-memory idempotency. No promo field, bonus calculation, new endpoint, delay, or new failure mode is required.
- Reuse existing calculation tests; add only missing deterministic base-commission cases needed by the shared rounding examples. The existing formula already supplies these values, so production changes may be unnecessary.
- Keep the vendor OpenAPI usable independently. If adding base-quote examples, do not add the application's bonus fields or change existing response meanings.
- After verification, add a concise README explanation that the vendor supplies the original commission and the application applies BOOST10. Record actual mock results separately from future end-to-end results.

## Independent acceptance

Use the real mock router/handler with synthetic credentials and either `failureMode: "random", failureRate: 0` for success or `loanAmount` for deterministic errors. Do not rely on or alter a developer's current CI profile. The table specifies base vendor responses only; the shared draft defines application bonus expectations.

| Loan amount / term / risk | Expected base rate / commission |
| --- | --- |
| `10000` / 36 / medium | `0.02` / `200` |
| `4001` / 36 / low | `0.01` / `40.01` |
| `4005` / 36 / low | `0.01` / `40.05` |
| `10003` / 36 / medium | `0.02` / `200.06` |
| `10000000` / 36 / high | `0.03` / `300000` |

Also run the existing missing/wrong API-key, invalid-input, sequential/concurrent replay, changed-input conflict, and six failure-trigger cases. A repeated successful base request must preserve its quote ID and original total. Failed responses remain failures with no quote or bonus fields. Test real HTTP behavior directly; no web or application process is needed.

## Verification and handoff

From the repository root:

```sh
cd test/mock/quotevendor
go test -race ./...
go vet ./...
go build -o /tmp/quotevendor-boost10 ./cmd
```

Format any changed Go files with `gofmt`. The mock is an independent Go module and has no production coverage threshold. Record inputs, deterministic configuration, expected/actual base responses, commands, and gaps here. Supply reproducible examples for integration; do not claim the bonus itself was implemented or verified by this task.
