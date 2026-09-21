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
