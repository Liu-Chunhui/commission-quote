# BOOST10 Task: Web

**Status:** Planned. Implement only the web portion of the [shared BOOST10 draft](01-development-approach.md#boost10-broker-loyalty-bonus-planned). Read its proposed business decisions, examples, and idempotency behavior first.

## Scope

Own `web/` and this task document. Reuse `web/src/App.tsx`, `web/src/quotes.ts`, and `web/tests/quotes.spec.ts`. The service owner maintains the Web OpenAPI; do not edit either Go service or wait for them to run.

## Changes

- Add a labelled `Promo code (optional)` text input. Update the existing “All fields are required” note. Follow the shared normalization/validation rule; omit the field from JSON when it normalizes to empty.
- Extend the application request and response types. Parse all returned money using `decimal.js`; validate the two new money fields at the response boundary using the existing approach. Do not calculate the bonus in the browser or silently substitute missing breakdown values.
- Display `Original commission`, `Bonus amount`, and `Final commission` from `totalCommission`, `bonusAmount`, and `finalCommission`. Show all three for no-promo quotes, including AUD 0.00 bonus. Retain quote ID and the original commission rate.
- Keep UUIDv5 generation based on the three loan fields. Promo-only edits preserve the key, clear the displayed result, and take effect on the next submission.
- Preserve loading, duplicate-submission prevention, error recovery, labels, keyboard access, and narrow-screen layout. The optional field participates in the existing disabled fieldset while loading.

## Independent acceptance

Intercept `/api/quotes` in Playwright. Use the shared draft JSON as a local test fixture until the service's OpenAPI update is merged; update existing success fixtures to contain both new fields so this task passes against the current checkout. Keep test fixtures inside `web/`; no production fallback or separate mock server.

| Check | Expected outcome |
| --- | --- |
| Blank / BOOST10 / normalized code | Correct optional request value and the corresponding shared example displayed |
| Unsupported promo | Clear validation feedback; no request; original loan validation still applies |
| Server-supplied breakdown | Display the returned amounts exactly, formatted to two decimals, without local recalculation |
| Promo added/removed, retry, reload | Same key for the same loan; correct new breakdown; no stale or compounded bonus |
| Delayed request | Progress visible, all controls disabled, one request despite repeat clicks |
| 400, generic 500, network failure, invalid JSON, missing/wrong-type bonus/final fields | Clear stale results, show the appropriate error, and permit recovery |
| Keyboard and narrow viewport | Optional field and three result rows remain accessible and readable |

## Verification and handoff

From the repository root, using the existing dependency/browser setup:

```sh
make web test
make web build
```

Neither Go service is required for these intercepted browser tests. Record the fixture/input, expected and actual UI/request behavior, commands, browser checks, and remaining gaps here. Keep live-stack integration for the shared post-merge check; do not label planned checks as passed.
