# BOOST10 Task: Service

**Status:** Planned. Implement the application-owned bonus in the [shared BOOST10 draft](01-development-approach.md#boost10-broker-loyalty-bonus-planned). Its proposed application contract extends the original Task B pass-through behavior.

## Scope

Own the root application's affected Go code/tests, `api/webapi.openapi.json`, and this task document. Start with `internal/httpapi/quote.go` and its tests; inspect the existing vendor client/types before changing them. Do not modify `web/` or the nested mock module.

## Changes

- Parse and validate the optional promo at the incoming HTTP boundary using the shared rule. Invalid input returns 400 before any vendor call; avoid normalizing or validating it repeatedly in internal layers.
- Keep promo state in the application request/response layer. Preserve the vendor's three-field request and response types in `internal/integration/commissionquote/`; forward only loan details and the unchanged idempotency key.
- After a valid vendor success, calculate the bonus once using `shopspring/decimal` and the shared half-up cent rule. Return the unchanged vendor fields plus required `bonusAmount` and `finalCommission`. Use a small local calculation; no promo registry, configuration, persistence, or new client abstraction is needed.
- Preserve vendor quote validation, timeout/cancellation, generic public errors, and existing request/logging context. Failed vendor requests return no breakdown. Replays always start from the original vendor total.
- Update Web OpenAPI request/response schemas, required response fields, every success example, validation descriptions, and promo-only idempotency semantics. Remove descriptions claiming the full application schema matches the vendor schema. Shared base fields retain their existing definitions.

## Independent acceptance

Exercise the real application router/handler and vendor client against controlled `httptest` HTTP servers. The vendor stub returns only `quoteId`, `commissionRate`, and `totalCommission`; it must not calculate a bonus. No mock service or frontend is required.

| Check | Expected outcome |
| --- | --- |
| Shared no-promo, normalized BOOST10, cent-rounding, and maximum examples | Exact decimal original, bonus, and final values |
| Missing/empty/whitespace promo; null, number, boolean, array, object, unsupported code | Valid absence gets zero bonus; invalid values return 400 with zero vendor calls |
| Controlled vendor total different from the mock formula | Bonus uses the supplied vendor total; no recalculation from loan/risk |
| Outbound request | Only three loan fields; unchanged key, private API key, method/path, and JSON content type |
| Same base quote with promo added, repeated, and removed | Same base ID/total; bonus applied once or reset to zero; no payout cache |
| All existing downstream failure and timeout cases with BOOST10 | Identical generic 500, no partial quote, no retry, and no leaked details |
| Existing no-promo clients | Existing three values retain their meaning; response also includes zero bonus and matching final commission |

Update existing exact-response assertions for the new application fields. Keep vendor-client response assertions unchanged. Review changed backend code against `AGENTS.md`, including declaration/function ordering, readability, exact money, logging, and secrets.

## Verification and handoff

From the repository root:

```sh
make server test
go test -race ./...
go vet ./...
make server build
```

Format changed Go files with `gofmt`. Inspect `gen/coverage.out` per production file under `internal/`; target greater than 90% and explain gaps. Root tests exclude the nested mock module. Follow one successful promo request and one failed request through logs, checking safe context and duplicate errors. Record controlled responses, commands, actual results, and remaining gaps here. Final frontend build/browser and real-service checks belong to integration after the other tasks merge.
