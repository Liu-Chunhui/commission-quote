# Development Approach and Shared Contract

Read this document first, then your task: [A: commission quote](02-commission-quote-task.md), [B: Application Backend](03-application-backend-task.md), or [C: Frontend](04-frontend-task.md). Shared decisions live here; task documents contain only their own scope, implementation context, and acceptance cases. Coding rules live in [AGENTS.md](../AGENTS.md).

This document defines shared requirements. Each task document records its implementation and verified handoff separately; passing independent task checks does not establish end-to-end integration. Inspect existing files before creating them.

**Planned extension:** [BOOST10 broker loyalty bonus](#boost10-broker-loyalty-bonus-planned) defines the new work split into [web](05-boost10-web-task.md), [service](06-boost10-service-task.md), and [mock](07-boost10-mock-task.md). Its proposed contract is a documentation draft, not implemented or verified behavior. Existing acceptance results refer to the original application.

## Development plan

Define the APIs and task boundaries first, develop the three parts in parallel, then integrate.

| Time | Work | Exit condition |
| --- | --- | --- |
| 0:00–0:20 | Agree contracts and scopes; initialize the root Go module if missing | Each task can start independently |
| 0:20–2:30 | A builds the vendor; B builds the application API; C builds the UI | Each task passes independent acceptance |
| 2:30–3:20 | Connect the three components | End-to-end success and failure flows work |
| 3:20–4:00 | Fix defects, review coverage, and finish handoff | Checks and run instructions are reproducible |

Use Go for both services and React with TypeScript for the UI. Services use separate Go modules and communicate only over HTTP; no shared Go package or Go workspace. The integration owner maintains the root `commissionquote` module, shared documents/configuration, and the final `test/mock/quotevendor/README.md`. Coordinate contract changes through that owner.

The challenge requires the form, quote results, vendor API-key protection, occasional random failures, error handling, tests, and setup/AI-usage notes. The stack, routes, numeric limits, formula, idempotency, and error mappings below are project decisions. No database, staff login, quote history, automatic retries, or deployment infrastructure is required. The mock's in-memory idempotency state is the only quote cache.

## API contracts and flow

| Caller → receiver | Endpoint | Local address | Required headers |
| --- | --- | --- | --- |
| Browser → application | `POST /api/quotes` | `http://localhost:8080` | `Content-Type: application/json`, `idempotency-key` |
| Application → vendor | `POST /quotes` | `http://localhost:8090` | Same headers, plus server-side `api-key` |

The browser calls only the application. The application validates input, forwards the key, calls the vendor, and returns the quote or the generic public failure response. The vendor authenticates before validating input. Keep its API key out of browser code, requests, and logs.

- [Web API specification](../api/webapi.openapi.json): browser-facing contract for B and C.
- [Vendor specification](../test/mock/quotevendor/api/commissionquote.openapi.json): vendor contract for A and B.

The vendor's `QuoteRequest` schema is the shared field definition. Keep both specifications' request/quote schemas and UUIDv5 definition identical. Each specification remains self-contained; exact response messages and full JSON examples belong there.

### Health checks

Every microservice exposes `GET /health`, registered in its own `internal/app/router.go` with the handler in `internal/httpapi/health.go`. Return HTTP 200 with `Content-Type: text/plain; charset=utf-8` and body `ok`. No request body, API key, or idempotency key is required. This is a local liveness check: do not call downstream services or apply quote validation, idempotency, or simulated failures. Each service's independent acceptance must verify this endpoint without credentials, including when its quote dependency is unavailable or mock failures are enabled.

## Request validation contract

```json
{
  "loanAmount": "10000",
  "loanTermInMonths": 36,
  "riskBand": "medium"
}
```

| Field | Rule |
| --- | --- |
| `loanAmount` | Required decimal string for whole AUD dollars, 4000–10000000 inclusive |
| `loanTermInMonths` | Required integer, 12–360 months inclusive |
| `riskBand` | Required string: exactly `low`, `medium`, or `high` |

Require one JSON object. Reject missing/null fields, wrong types (including JSON numbers for money and strings for the term), fractional amounts/terms, out-of-range values, malformed JSON, and trailing JSON values. Ignore extra fields. Do not round, clamp, normalize risk labels, infer risk, or supply defaults. Apply one generic rule set without loan categories or cross-field eligibility rules.

Each service validates at its HTTP boundary; internal code uses the validated values. Frontend validation provides usability and does not replace server validation. Invalid application input must not call the vendor.

### Validation acceptance cases

| Field | Accept | Reject |
| --- | --- | --- |
| `loanAmount` | `"4000"`, `"10000"`, `"10000000"` | `4000`, `"3999"`, `"10000001"`, `"4000.5"`, `"04000"`, `"4e3"` |
| `loanTermInMonths` | `12`, `36`, `360` | `11`, `361`, `12.5` |
| `riskBand` | `low`, `medium`, `high` | Empty or unsupported values |

Keep other fields valid; also test required fields, wrong types, and invalid bodies. APIs return 400; the UI blocks invalid submission. Direct HTTP tests use valid headers and a fresh key per unrelated case, with simulated failures disabled. UI tests derive keys only after validation. No scientific-notation or numeric-format equivalence tests are required.

## Quote response

```json
{
  "quoteId": "quote-example-standard",
  "commissionRate": "0.02",
  "totalCommission": "200"
}
```

Use `shopspring/decimal` in both Go services and `decimal.js` in the frontend for loan amounts, commission rates, and commission totals. JSON carries exact decimal strings; do not convert monetary values through binary floats. Loan amount strings contain canonical whole dollars without signs, leading zeroes, decimal points, or exponents. Commission strings have at most two decimal places and no exponent notation; display two decimal places without converting to JavaScript `number`.

`quoteId` is non-empty and opaque. `commissionRate` is a fraction, so `0.02` means 2%. `totalCommission` is AUD with at most two decimal places. Only the vendor calculates commission or generates quote IDs.

Mock rates: low = 1%, medium = 2%, high = 3%. Calculate AUD commission using decimal multiplication: `loanAmount * commissionRate`; no rounding is needed for whole-dollar amounts and these rates. Term is validated but does not affect this formula. Examples: 10000/medium → 200; 750000/medium → 15000; 4001/low → 40.01.

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
| JSON `host` | A, B | Dev profiles bind to `localhost`; CI profiles bind to `0.0.0.0` for container networking |
| Application JSON `port` | B | Integer from 1 through 65535; both `confg/dev.json` and `confg/ci.json` use 8080 |
| Application JSON `dependencies.commissionquote.baseUrl` | B | Required absolute HTTP(S) base URL; dev uses `http://localhost:8090`, CI uses Compose DNS `http://quotevendor:8090`; the client appends `/quotes` |
| Mock JSON `port` | A | Integer from 1 through 65535; both mock profiles use 8090; no `VENDOR_ADDR` override |
| Application JSON `apiKeyFile` | B | Required key file; both profiles use `../test/mock/data/API_KEY`, sharing the mock service's private key |

The application loads the file selected by `--config`, defaulting to `confg/dev.json` relative to the working directory. Missing/unreadable files, malformed JSON, and missing/invalid ports or base URLs stop startup. Base URLs must not contain credentials, query strings, or fragments. The file is the source of the application port and downstream base URL; `APP_ADDR` and `VENDOR_BASE_URL` are not used.

The application resolves relative `apiKeyFile` paths against the selected profile directory and supports absolute mount paths. It loads the key into `Config.APIKey`, trims surrounding whitespace, and rejects missing, unreadable, or empty key files. Credentials are not read from JSON values or environment variables, included in serialized configuration, or logged.

The dev frontend runs on port 5173 and proxies `/api` through Vite. In CI, Nginx serves the built frontend on host port 8088 and proxies `/api` to `server:8080`; only this web port is published, bound to host loopback. Mock failure profiles belong only to A:

| Profile | Failure behavior |
| --- | --- |
| [config/dev.json](../test/mock/quotevendor/config/dev.json) | `loanAmount`: prefix `100` + HTTP status; 100400/100401/100409/100429/100500/100503 → 400/401/409/429/500/503; all others succeed |
| [config/ci.json](../test/mock/quotevendor/config/ci.json) | `random`: fail with probability `failureRate: 0.1`, selecting uniformly from 400/401/409/429/500/503; otherwise success; amount triggers disabled |

All commission quote runtime settings come from the selected JSON profile; there are no environment-variable overrides. Both profiles reference `../../data/API_KEY` using `apiKeyFile`, resolved relative to the selected profile directory. Absolute file paths are supported for production mounts. Keep the local `test/mock/data/API_KEY` out of Git. The deployment platform supplies the secret-manager value as a file before startup; the service reads it once, trims surrounding whitespace, and rejects a missing or empty key. It does not read an `API_KEY` environment variable or contact a secret manager. Restart after key rotation.

Modes are mutually exclusive and run after authentication, validation, and idempotency checks. Successful replays bypass simulation. Random mode requires a numeric rate from 0 through 1; loanAmount mode ignores the rate entirely, including its type/range. No real rate limiter is required. All six trigger amounts are valid decimal-string inputs. Simulated 401/409 responses do not bypass actual authentication or idempotency checks.

## Common acceptance and handoff

A task is ready to merge when its own acceptance cases pass against the real component with controlled dependencies. A tests its real handler; B uses fixed/delayed `httptest` vendor servers; C intercepts application responses. No task waits for another implementation. Test outcomes must be deterministic: use loanAmount mode or random rates 0/1, not probabilistic assertions against the CI profile.

All tasks must build, satisfy their API contract, and provide reproducible commands/steps, inputs or mock responses, expected versus actual results, limitations, and an honest AI-usage summary. Keep mocks in test/development tooling, never as a production fallback. Record actual checks; a build alone does not establish browser behavior.

## Final integration

Development integration has been verified for successful quotes, replay after reload, the dev trigger failures, recovery, and browser credential isolation. Reproduction steps and remaining verification limits are in the [combined README](../test/mock/quotevendor/README.md#verify-the-complete-app). The checklist below also includes CI/random-mode checks beyond that dev verification.

After independent acceptance and merging:

1. Start all components with the shared configuration. Use dev mode and an ordinary amount for success; verify browser key secrecy and invalid-input handling.
2. Repeat unchanged input after success/reload: same key and quote. To test conflicts, manually reuse the old key with changed input: the quote service returns 409 internally and the Web API returns generic 500. The UI normally derives a different key.
3. Exercise all six dev trigger amounts and a temporary random config with rate 1. Verify the same generic public failure, UI recovery, and distinct internal causes in logs. Confirm timeout handling with B's slow-vendor test and C's mocked timeout response; no production delay endpoint is needed.
4. Smoke-check the CI profile, restore dev for local work, run both Go suites and the frontend build, and record browser checks and coverage.
5. Finish `test/mock/quotevendor/README.md` with prerequisites, environment setup, startup/tests, assumptions, limitations, and AI usage. Verify a clean start and disclose unresolved gaps. Publishing or sending the submission requires separate user authorization.

## BOOST10 broker loyalty bonus (planned)

### Requirement and draft decisions

Add an optional promo code for accredited broker partners. `BOOST10` adds 10% of the original commission to the payout. Show the original commission, bonus amount, and final commission separately.

The requirement does not define calculation ownership, code normalization, invalid-code behavior, rounding, or accreditation checks. This draft proposes:

- The application service calculates the bonus after receiving a successful, validated vendor quote. The vendor remains the source of the original commission; the web displays the returned breakdown.
- Trim surrounding ASCII whitespace and uppercase ASCII letters in the promo code at the web and application boundaries. Omitted, empty, or whitespace-only strings mean no promo. The only supported non-empty normalized value is `BOOST10`; reject other values with 400 `INVALID_REQUEST` and message `Promo code must be BOOST10 or left blank.` Reject null and non-string values with the same error. Invalid promo input makes no vendor call.
- Treat broker accreditation as a precondition supplied by the existing business workflow. This feature does not prove accreditation; do not introduce a checkbox, broker database, login, or accreditation lookup. If the app must enforce eligibility itself, that requires a separate identity/eligibility contract before release.
- Apply the bonus once per quote calculation. Round the bonus to two decimal places using decimal half-up rounding, then add that rounded amount to the original commission. No bonus cap, stacking, expiry, or promo configuration is introduced.

These are proposed decisions, not additional requirements supplied by the business. Resolve any requested changes in this shared section before the three implementations diverge.

### Application contract

Extend only `POST /api/quotes` with optional `promoCode`. Existing loan validation, headers, routes, authentication boundaries, and generic downstream-error handling remain applicable.

```json
{
  "loanAmount": "10000",
  "loanTermInMonths": 36,
  "riskBand": "medium",
  "promoCode": "BOOST10"
}
```

Successful application responses always contain the existing three fields plus `bonusAmount` and `finalCommission`, including when no promo is supplied:

```json
{
  "quoteId": "quote-example-standard",
  "commissionRate": "0.02",
  "totalCommission": "200",
  "bonusAmount": "20",
  "finalCommission": "220"
}
```

| Field | Meaning |
| --- | --- |
| `quoteId` | Unchanged opaque vendor quote ID |
| `commissionRate` | Unchanged base commission rate; do not increase it by 10 percentage points |
| `totalCommission` | Original vendor commission, preserving its existing meaning |
| `bonusAmount` | Rounded 10% bonus for BOOST10; otherwise `"0"` |
| `finalCommission` | Original commission plus the rounded bonus |

All monetary fields remain decimal strings with at most two decimal places. Display AUD with two decimal places. Use the existing decimal libraries and exact string constants: `bonusAmount = roundHalfUp(totalCommission * "0.10", 2)` for BOOST10, otherwise zero; `finalCommission = totalCommission + bonusAmount`. Calculate from the vendor total, never from the loan amount, a previously boosted result, or a recalculated vendor rate.

The vendor request/response remains the existing three-field contract. The application sends only loan amount, term, and risk to the vendor; it does not forward `promoCode` or require bonus fields in the vendor response. Its public DTO therefore differs from the vendor DTO. The existing rule that both APIs have identical complete schemas and the original Task B prohibition on all commission calculation are superseded only for this application-owned bonus. Base loan fields and vendor quote fields stay synchronized.

The service task owns the Web OpenAPI update, including all success examples, promo validation, and the distinction from the vendor schema. Until then, these draft examples define the proposed extension; existing OpenAPI files still describe the original application. Web work can use local test fixtures from this section without waiting for that update. Deploy the completed web and service changes together.

### Idempotency and failure behavior

Keep the existing UUIDv5 name `loanAmount|loanTermInMonths|riskBand`, namespace, and example key. It identifies the base vendor quote. `promoCode` is not part of that key or the vendor's input binding.

Changing only the promo keeps the same key and base quote ID. The application calculates the breakdown from the incoming normalized promo and the original vendor quote on every successful request; it stores no payout state. Adding, removing, retrying, or reloading BOOST10 must never compound the bonus. A different loan amount, term, or risk still changes the key; manually reusing a key for changed loan fields still produces the existing vendor conflict and generic application 500.

Invalid promo input follows application 400 validation behavior. All vendor failures, timeouts, and malformed responses remain generic 500 failures with no partial breakdown. Existing mock failure modes and successful replay behavior remain unchanged.

### Shared acceptance examples

All rows use term 36. Values below are decimal-string API values; trailing `.00` is optional on the wire.

| Loan amount / risk | Promo input | Original | Bonus | Final |
| --- | --- | --- | --- | --- |
| `10000` / medium | Omitted, empty, or ASCII whitespace | `200` | `0` | `200` |
| `10000` / medium | `BOOST10` | `200` | `20` | `220` |
| `10000` / medium | ` boost10 ` | `200` | `20` | `220` |
| `4001` / low | `BOOST10` | `40.01` | `4` | `44.01` |
| `4005` / low | `BOOST10` | `40.05` | `4.01` | `44.06` |
| `10003` / medium | `BOOST10` | `200.06` | `20.01` | `220.07` |
| `10000000` / high | `BOOST10` | `300000` | `30000` | `330000` |

Also check unsupported `BOOST20`, null/non-string promo values, same-input replay, promo removal, loading, validation, failure recovery, and stale-result clearing. A promo is an optional input; the three loan fields remain required.

### Parallel ownership and integration

| Part | Owned files | Independent dependency |
| --- | --- | --- |
| [Web](05-boost10-web-task.md) | `web/` and its task document | Intercepted `/api/quotes` responses using the above breakdown |
| [Service](06-boost10-service-task.md) | Root application Go code/tests, `api/webapi.openapi.json`, and its task document | Controlled `httptest` vendor responses with the original three fields |
| [Mock](07-boost10-mock-task.md) | `test/mock/quotevendor/` and its task document | Real mock handler with deterministic configuration; no application or web |

Give each part a separate branch/worktree. Each owner changes only its scope, records actual acceptance results, and reports contract questions here. Mock work is primarily verification: reuse existing behavior and add only missing base-quote cases; no promo engine or new simulation mode is needed.

The integration owner maintains this shared section and reconciles the original task documents and repository guidance with the completed extension. After independent acceptance and merging, verify the real browser → service → mock flow for no promo, BOOST10, rounding, promo removal/reload, and a dev failure trigger with BOOST10. Confirm unchanged key/base quote ID on promo-only changes and no credentials in the browser. Update the combined README and actual handoff results after those checks; this draft records no implementation acceptance.
