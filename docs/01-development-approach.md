# Development Approach and Shared Contract

Read this document first, then your task: [A: Mock Vendor](02-mock-vendor-task.md), [B: Application Backend](03-application-backend-task.md), or [C: Frontend](04-frontend-task.md). Shared decisions live here; task documents contain only their own scope, implementation context, and acceptance cases. Coding rules live in [AGENTS.md](../AGENTS.md).

These documents define requirements, not completed implementation. Inspect existing files before creating them.

## Development plan

Define the APIs and task boundaries first, develop the three parts in parallel, then integrate.

| Time | Work | Exit condition |
| --- | --- | --- |
| 0:00–0:20 | Agree contracts and scopes; initialize the root Go module if missing | Each task can start independently |
| 0:20–2:30 | A builds the vendor; B builds the application API; C builds the UI | Each task passes independent acceptance |
| 2:30–3:20 | Connect the three components | End-to-end success and failure flows work |
| 3:20–4:00 | Fix defects, review coverage, and finish handoff | Checks and run instructions are reproducible |

Use Go for both services and React with TypeScript for the UI. Services use separate Go modules and communicate only over HTTP; no shared Go package or Go workspace. The integration owner maintains the root `commissionquote` module, shared documents/configuration, and the final `docs/README.md`. Coordinate contract changes through that owner.

The challenge requires the form, quote results, vendor API-key protection, occasional random failures, error handling, tests, and setup/AI-usage notes. The stack, routes, numeric limits, formula, idempotency, and error mappings below are project decisions. No database, staff login, quote history, automatic retries, or deployment infrastructure is required. The mock's in-memory idempotency state is the only quote cache.

## API contracts and flow

| Caller → receiver | Endpoint | Local address | Required headers |
| --- | --- | --- | --- |
| Browser → application | `POST /api/quotes` | `http://localhost:8080` | `Content-Type: application/json`, `idempotency-key` |
| Application → vendor | `POST /quotes` | `http://localhost:8090` | Same headers, plus server-side `api-key` |

The browser calls only the application. The application validates input, forwards the key, calls the vendor, and returns the quote or the generic public failure response. The vendor authenticates before validating input. Keep its API key out of browser code, requests, and logs.

- [Web API specification](../api/webapi.openapi.json): browser-facing contract for B and C.
- [Vendor specification](../test/mock/commissionquote/api/mock-commissionquote.openapi.json): vendor contract for A and B.

The vendor's `QuoteRequest` schema is the shared field definition. Keep both specifications' request/quote schemas and UUIDv5 definition identical. Each specification remains self-contained; exact response messages and full JSON examples belong there.

### Health checks

Every microservice exposes `GET /health`, registered with `router.Get("/health", health)`. Return HTTP 200 with `Content-Type: text/plain; charset=utf-8` and body `ok`. No request body, API key, or idempotency key is required. This is a local liveness check: do not call downstream services or apply quote validation, idempotency, or simulated failures. Each service's independent acceptance must verify this endpoint without credentials, including when its quote dependency is unavailable or mock failures are enabled.

## Request validation contract

```json
{
  "loanAmount": 10000,
  "loanTermInMonths": 36,
  "riskBand": "medium"
}
```

| Field | Rule |
| --- | --- |
| `loanAmount` | Required integer, AUD 4000–10000000 inclusive |
| `loanTermInMonths` | Required integer, 12–360 months inclusive |
| `riskBand` | Required string: exactly `low`, `medium`, or `high` |

Require one JSON object. Reject missing/null fields, wrong types, numeric strings, fractional amounts/terms, out-of-range values, malformed JSON, and trailing JSON values. Ignore extra fields. Do not round, clamp, normalize risk labels, infer risk, or supply defaults. Apply one generic rule set without loan categories or cross-field eligibility rules.

Each service validates at its HTTP boundary; internal code uses the validated values. Frontend validation provides usability and does not replace server validation. Invalid application input must not call the vendor.

### Validation acceptance cases

| Field | Accept | Reject |
| --- | --- | --- |
| `loanAmount` | `4000`, `10000`, `10000000` | `3999`, `10000001`, `4000.5` |
| `loanTermInMonths` | `12`, `36`, `360` | `11`, `361`, `12.5` |
| `riskBand` | `low`, `medium`, `high` | Empty or unsupported values |

Keep other fields valid; also test required fields, wrong types, and invalid bodies. APIs return 400; the UI blocks invalid submission. Direct HTTP tests use valid headers and a fresh key per unrelated case, with simulated failures disabled. UI tests derive keys only after validation. No scientific-notation or numeric-format equivalence tests are required.

## Quote response

```json
{
  "quoteId": "quote-example-standard",
  "commissionRate": 0.02,
  "totalCommission": 200
}
```

`quoteId` is non-empty and opaque. `commissionRate` is a fraction, so `0.02` means 2%. `totalCommission` is AUD with at most two decimal places. Only the vendor calculates commission or generates quote IDs.

Mock rates: low = 1%, medium = 2%, high = 3%. Calculate integer commission cents as `loanAmount * ratePercent`, then return dollars; no rounding is needed. Term is validated but does not affect this formula. Examples: 10000/medium → 200; 750000/medium → 15000; 4001/low → 40.01.

## Idempotency: both endpoints

Require a case-sensitive, opaque `idempotency-key` value of 1–255 printable ASCII characters without spaces (`^[!-~]{1,255}$`). Missing/invalid values return 400. Header names are case-insensitive. The key is not a credential.

- Frontend derives the key below; the application validates its format and forwards it unchanged. Neither service verifies UUID derivation, and the application stores no idempotency state.
- After authentication and header/body validation, the vendor binds the key to the three validated fields. Compare values, not raw JSON; field order and ignored extra fields do not matter. Changed input under a bound key returns 409 to the application before simulation; the application exposes only the generic 500 response.
- Same key/input replays the stored successful response, including `quoteId`, without simulation or recalculation. Otherwise apply simulation, calculate on success, and store before responding. Failures retain the input binding but no response, allowing retries. Invalid authentication/header/body reserves no key.
- Concurrent duplicates must produce at most one successful quote. State lasts for one mock process; restarting clears it. No TTL, persistence, or cross-instance guarantee is required.

### Frontend key generation

Use [UUIDv5](https://www.rfc-editor.org/rfc/rfc9562.html#section-5.5):

```text
namespace = eae5612f-0c86-500d-ab8a-3f9eb483991f
name = loanAmount + "|" + loanTermInMonths + "|" + riskBand
idempotency-key = UUIDv5(namespace, name)
```

This is algorithm notation, not a library's argument order. The public namespace is UUIDv5(DNS namespace, "commissionquote"). Build the UTF-8 name from validated decimal integers and lowercase risk, without spaces, timestamps, or randomness; emit a lowercase hyphenated UUID.

`10000|36|medium` must produce `32fafb2a-70b4-5114-9f2d-52be7286eaf5`. Identical input gives the same key after errors, success, reloads, and across browser sessions; changing any field changes it. Recompute without key storage. Repeating a successful submission therefore reuses the quote while the vendor retains it.

## Error boundary

Both APIs use `{"error":{"code":"...","message":"..."}}`, but their error contracts are separate. The application handles quote-service statuses/codes internally and never forwards them or their messages to the frontend.

| Web API outcome | Response |
| --- | --- |
| Invalid browser input/header | 400 `INVALID_REQUEST`, with a useful validation message; no quote-service call |
| Any quote-service failure, connection/decoding error, timeout, or unexpected local failure | 500 `INTERNAL_ERROR`: `Unable to generate a quote. Please try again later.` |

The frontend uses one generic failure flow, without vendor-specific status/code branches. Internal causes belong in sanitized server logs, never the public response. The application uses a three-second outbound timeout and propagates cancellation; no automatic retries. Mock trigger amounts pass ordinary input validation; any resulting service error uses the same generic failure response. Their behavior still depends on the selected mock mode.

## Runtime configuration

| Setting | Owner | Default or requirement |
| --- | --- | --- |
| Application JSON `port` | B | Integer from 1 through 65535; both `confg/dev.json` and `confg/ci.json` use 8080; binds to `localhost` |
| `VENDOR_BASE_URL` | B | `http://localhost:8090`; append `/quotes` |
| `VENDOR_ADDR` | A | `localhost:8090` |
| `VENDOR_API_KEY` | A and B | Required environment variable; same private value in both processes |

The application loads the file selected by `--config`, defaulting to `confg/dev.json` relative to the working directory. Missing/unreadable files, malformed JSON, and missing/invalid ports stop startup. The file is the only source of the application port; `APP_ADDR` is no longer used.

The frontend runs on port 5173 and proxies `/api` to the application. Mock failure profiles belong only to A:

| Profile | Failure behavior |
| --- | --- |
| [config/dev.json](../test/mock/commissionquote/config/dev.json) | `loanAmount`: exact amount 100400 → 400; 100429 → 429; all others succeed |
| [config/ci.json](../test/mock/commissionquote/config/ci.json) | `random`: 503 with `failureRate: 0.1`; otherwise success; amount triggers disabled |

Modes are mutually exclusive and run after authentication, validation, and idempotency checks. Successful replays bypass simulation. Random mode requires a numeric rate from 0 through 1; loanAmount mode ignores the rate entirely, including its type/range. No real rate limiter is required. Both trigger amounts are valid inputs.

## Common acceptance and handoff

A task is ready to merge when its own acceptance cases pass against the real component with controlled dependencies. A tests its real handler; B uses fixed/delayed `httptest` vendor servers; C intercepts application responses. No task waits for another implementation. Test outcomes must be deterministic: use loanAmount mode or random rates 0/1, not probabilistic assertions against the CI profile.

All tasks must build, satisfy their API contract, and provide reproducible commands/steps, inputs or mock responses, expected versus actual results, limitations, and an honest AI-usage summary. Keep mocks in test/development tooling, never as a production fallback. Record actual checks; a build alone does not establish browser behavior.

## Final integration

After independent acceptance and merging:

1. Start all components with the shared configuration. Use dev mode and an ordinary amount for success; verify browser key secrecy and invalid-input handling.
2. Repeat unchanged input after success/reload: same key and quote. To test conflicts, manually reuse the old key with changed input: the quote service returns 409 internally and the Web API returns generic 500. The UI normally derives a different key.
3. Exercise both dev trigger amounts and a temporary random config with rate 1. Verify the same generic public failure, UI recovery, and distinct internal causes in logs. Confirm timeout handling with B's slow-vendor test and C's mocked timeout response; no production delay endpoint is needed.
4. Smoke-check the CI profile, restore dev for local work, run both Go suites and the frontend build, and record browser checks and coverage.
5. Finish `docs/README.md` with prerequisites, environment setup, startup/tests, assumptions, limitations, and AI usage. Verify a clean start and disclose unresolved gaps. Publishing or sending the submission requires separate user authorization.
