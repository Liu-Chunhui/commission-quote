# Task C: Frontend

Build the form and quote experience. Read the [shared contract and acceptance rules](01-development-approach.md) and [Web API](../api/webapi.openapi.json).

## Scope

Own `web/`: React with TypeScript, Vite configuration, package manifest/lockfile, application code, and CSS. Use native form controls, browser `fetch`, and basic CSS. No form library, global state manager, or production mock fallback is needed. Do not modify either Go service.

## UI behavior

- Start with empty amount/term inputs and an empty required risk selection. Label amount as AUD and term as months. Use the shared min/max constraints, `step=1`, and required fields; check empty input before numeric conversion.
- After validation, generate the [shared UUIDv5 key](01-development-approach.md#frontend-key-generation) and submit typed JSON. Configure the local `/api` proxy using the shared runtime settings.
- Display the returned quote ID, rate as a percentage, and AUD total with two decimal places. Do not calculate commission locally.
- Support keyboard navigation, visible focus, associated labels, an announced status/error region, and narrow screens.

| State | Behavior |
| --- | --- |
| Invalid input | Identify the field and block submission |
| Loading | Show progress, disable repeat submission, and clear stale result/error content |
| Success | Display the returned quote, including replays, and restore submission |
| API error | Show input-validation feedback or the single generic failure message and restore the form; no quote-service error branches |
| Network/unreadable response | Show a useful generic error and allow manual retry |

## Acceptance

Exercise the real UI and fetch lifecycle using controlled/intercepted Web API responses. Manual browser checks are acceptable; use existing test tools where available.

| Check | Pass condition |
| --- | --- |
| Shared validation cases | Invalid forms send nothing; valid requests contain integer amount/term and the selected risk |
| Quote display | Shared success examples format correctly without recalculation |
| Idempotency | Match the shared UUIDv5 example; changing any field changes the key; unchanged input retains it after errors, success, or reload |
| Delayed response and repeat click | Loading remains visible and only one request is sent |
| API errors and recovery | Validation errors, generic 500, network failure, and invalid JSON restore controls without a stale quote; a later success clears the error |
| Mock trigger inputs | Both submit normally; when the intercepted API returns 500, show the same generic failure |
| Browser network | Only the application endpoint is called; no vendor API key is present |
| Accessibility/layout | Keyboard flow, labels, focus, announcements, and narrow viewport remain usable |

**Done:** these checks, the production build, and the [common acceptance and handoff](01-development-approach.md#common-acceptance-and-handoff) pass without A or B. Include reproducible interception/mock setup and browser steps in the handoff.

## Commands

After implementation, run from `web/`:

```sh
npm install
npm run build
npm run dev
```

Commit the lockfile so clean installs can use `npm ci`.

## Implementation and handoff

The single-column UI lives in `web/src/App.tsx`; `web/src/quotes.ts` owns the fetch boundary and UUIDv5 generation using browser Web Crypto. Native form constraints run before numeric conversion. Editing input clears the previous result; submitting disables the form until the request completes. No browser storage or commission calculation is used.

### Visual design and assets

The internal lending-tool styling follows the public [Bendigo Bank business lending page](https://www.bendigobank.com.au/business/loans-and-finance/), inspected on 2026-09-21: Muli typography, burgundy `#870e40` / `#58003a`, red `#de313b` actions, and light neutral surfaces. This is a public-site reference, not a verified internal design system. The single-column flow, native controls, labels, validation, and API behavior are preserved.

Assets are served locally from `web/public/brand/`; no external font or image requests are needed at runtime:

- Logo: [Bendigo Bank's official asset](https://www.bendigobank.com.au/globalassets/globalresources/brand-logos/bendigobank-logo.png), supplied by the site as WebP and saved without visual modification as `bendigo-bank.webp`.
- Fonts: official [regular Muli](https://www.bendigobank.com.au/static/assets/fonts/muli/muli.woff2) and [bold Muli](https://www.bendigobank.com.au/static/assets/fonts/muli/muli-bold.woff2). The [upstream SIL Open Font License](https://github.com/googlefonts/mulish/blob/main/OFL.txt) is included as `OFL.txt`; the Bendigo logo remains a Bendigo Bank brand asset.

### Run locally

Use Node.js 22.12+ and npm. From the repository root:

```sh
cd web
npm ci
npm run dev
```

The root Makefile also provides these commands (GNU Make and `lsof` required):

| Command | Behavior |
| --- | --- |
| `make web dev` | Start only the frontend in the foreground; Ctrl+C stops it |
| `make web build` | Type-check and build the frontend |
| `make web test` | Run the browser acceptance suite; install Chromium as described below on first use |
| `make clean` | Stop this worktree's Vite processes and remove `web/node_modules`, `web/dist`, `web/test-results`, `web/playwright-report`, backend `bin/` and `gen/`, and legacy root `app`, `coverage.out`, and `coverage.html` |

The web commands install locked dependencies automatically when missing or when the manifests change. Cleanup preserves source files, the lockfile, other worktrees, and shared npm/Playwright caches.

Open `http://localhost:5173`. Vite proxies `/api` to `http://localhost:8080`; normal manual quote generation needs the application backend there. The frontend needs no environment variables or credentials. Web Crypto requires localhost or HTTPS.

### Independent acceptance

From `web/`:

```sh
npx playwright install chromium
npm test
npm run build
```

Playwright starts Vite automatically, or reuses an existing local server on port 5173. `tests/quotes.spec.ts` intercepts `**/api/quotes` using [Playwright network routing](https://playwright.dev/docs/network) and reads success/error examples directly from `api/webapi.openapi.json`. Neither Go service is needed. It holds a response open for the loading check, aborts a request for network failure, and returns malformed JSON or an invalid quote for unreadable responses. These mocks are test-only.

For an interactive reproduction, run `npm test -- --debug --grep "submits typed input"`. Step through entering `10000`, `36`, and `medium`, then clicking **Generate Quote**. Expect the standard contract quote, `2%`, `AUD 200.00`, and the contract UUIDv5 header. Other named cases in the same file reproduce the remaining acceptance flows; use `--grep "recovers from"` or `--grep "response is pending"` to inspect errors or loading.

### Verified results (2026-09-21)

| Check | Expected and observed |
| --- | --- |
| Browser suite | 16 Chromium tests passed against intercepted responses: validation, boundaries/risk options, all success examples, server-supplied totals, idempotency across retries/reloads/new sessions, duplicate prevention, error recovery, trigger inputs, and keyboard/mobile flow |
| Request boundary | Typed JSON and the required key sent only to `/api/quotes`; no `api-key` header |
| Visual checks | Desktop 1280px and mobile 360px layouts inspected; result readable, no horizontal overflow, no framework overlay or console errors in the successful flow |
| Accessibility | Associated labels, focus outline/order, keyboard submission, invalid-field focus, and status/alert regions verified; actual screen-reader speech was not tested |
| Build/install | `npm ci` and `npm run build` passed |

Browser plugin was not available; verification used Playwright Chromium. Real-service integration, other browser engines, and screen-reader testing remain outside this independent handoff. No backend code was changed or tested.

### AI Usage

Codex read the shared/task documents and API contract, implemented the React/TypeScript UI and test tooling, and ran the build and browser checks. Browser tests exposed an invalid-field selector bug, which was fixed before the passing run. The user selected the single-column layout and directed implementation against the supplied contract. No AI service is used by the application at runtime.
