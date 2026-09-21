# Project Guidelines

These requirements apply to all work in this repository.

## Challenge Scope

- This repository is for the **Commission Quote App** challenge described in `Code Challenge - Commission Quote App.pdf`.
- Respect the challenge's four-hour timebox. Prioritize a working solution, readable code, core tests, clear edge-case handling, and simple run instructions over production infrastructure.
- Build a loan-details web form and a mock Commission Quote API that represents the unavailable external vendor.
- The PDF allows any familiar tech stack. Go and React with TypeScript are retained repository choices, not requirements imposed by the challenge.
- Do not add persistence, authentication for staff, quote history, or other features unless requested. The challenge does not require a database.
- Treat submission instructions in the PDF as delivery context, not authorization to publish a repository or send files or messages.

## Project Layout

- Follow [Standard Go Project Layout](https://github.com/golang-standards/project-layout/blob/master/README.md).
- Place executable entry points in `cmd/`, the frontend in `web/`, and documentation in `docs/`. Keep the application backend implementation and its tests together in `cmd/app/` using `package main`; no separate application package directory is needed. Keep `main.go` limited to startup wiring, with request handling and vendor calls in separate files in the same directory.
- Keep the four planning/task documents, human-readable API contract, and design notes in the root `docs/`. Keep service run instructions and the README in `test/mock/commissionquote/README.md`. Place machine-readable API specifications in the owning service's `api/` directory, following Standard Go Project Layout; the application specification belongs in `api/webapi.openapi.json`, and the mock specification belongs in `test/mock/commissionquote/api/`. Keep the root `AGENTS.md` as the tool-discovered repository instruction file.
- Keep the commission quote as an independently runnable service under `test/mock/commissionquote/`, with its own Go module, `api/` specification, `cmd/main.go` entry point, and `internal/app/router.go` routing, `internal/httpapi/` handlers, and `internal/config/` configuration loading. Apply the Go project layout relative to this independent service root. Keep its task document in `docs/02-commission-quote-task.md` alongside the other planning documents. It communicates with the application only over HTTP; neither service imports the other's code.
- Treat the mock quote service and application backend as separate tasks with independent implementation, tests, and acceptance. The backend task must not implement or maintain the mock service. Connecting the two running services belongs to the final integration phase.
- Create directories only when needed. Use `pkg/` only for libraries intended for external consumers.

## Backend

- Prioritize readability and clear business logic over brevity, cleverness, or abstraction. Keep control flow straightforward and names explicit.
- Validate untrusted data at system boundaries; do not repeat the same checks across internal layers once the data has been validated.
- Avoid speculative defensive checks for states prevented by types, constructors, or established invariants. Add a check only for a concrete failure case relevant to the application.
- Retain necessary input validation, API-key authentication, and handling of network, timeout, decoding, and other operational errors.
- Use Go and follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
- Use `github.com/go-chi/chi/v5` for HTTP routing in both the application backend and the independent mock quote service.
- Every microservice must register `GET /health`. The commission quote service registers `router.Get("/health", httpapi.Health)` in `internal/app/router.go`. Follow the shared health-check contract in `docs/01-development-approach.md`.
- Format Go code with `gofmt`.
- Place package-level `const` and `var` declarations at the top of each file, after `package` and imports, before type declarations and functions.
- Place constructors immediately below their corresponding type declarations. Constructors are exempt from the function ordering rule below.
- Within each Go file, place other exported (public) functions and methods first, followed by unexported (private) methods, then unexported standalone helper functions at the bottom. Sort each group alphabetically by function or method name.
- Place local `var` declarations at the beginning of their function, including anonymous functions.
- Separate independent `if` blocks with a blank line for readability. Keep an error check directly adjacent to the operation it checks.
- Prefix unexported package-level constants and variables with `_`, except error variables, which use `err` as specified by the Uber Go Style Guide.

### Logging

- Use standard-library `log/slog` for structured logs to stderr. Use UTC for all timestamps, including the automatic log timestamp and time-valued log attributes.
- Log errors at `ERROR` level where they are raised or first detected. If a library or remote service returns an error, log it at the calling boundary with the failed operation and a safe explanation of the cause.
- Log each error once within a service. Higher layers may wrap, return, or map an already-logged error; do not log it again merely because it was propagated. Request-completion logs remain separate lifecycle events.
- Assign a `request_id` when a request enters the service and carry it through the request context. Include it in handler, business-operation, and outbound vendor-call logs so concurrent requests can be followed independently. It identifies one HTTP attempt, not the idempotency key shared by retries.
- Log request start/completion and outbound vendor-call start/completion at `INFO`. Include `service`, `request_id`, and `operation`; add HTTP method/path, response status, and `duration_ms` where applicable. Error records also include `error_code` when available. Log relevant decisions such as an idempotency replay or simulated vendor failure so the outcome is understandable.
- Keep messages concise and explain what happened. Do not log every function call, full request/response bodies, or all headers. Never log credentials, API keys, or other private secrets; sanitize error details before logging.
- During backend verification, follow a successful request and a failed request through the logs. Confirm the sequence, failed operation/cause, final HTTP status, and timing are clear, and that error propagation does not produce duplicate error records. No custom logging framework or abstraction is required.

## API Design

- Preserve the vendor contract's exact JSON field names:
  - Request: `loanAmount`, `loanTermInMonths`, `riskBand`.
  - Response: `quoteId`, `commissionRate`, `totalCommission`.
- The commission quote API must require a valid `api-key` header and reject both missing and invalid keys.
- Require `idempotency-key` on both quote endpoints. The browser deterministically generates UUIDv5 from the validated `loanAmount|loanTermInMonths|riskBand` using the fixed namespace in the shared contract, and the application forwards it unchanged. Identical input produces the same key, including after success or page reload. Follow the shared idempotency contract: the mock binds a key to validated input, replays successful quotes, rejects changed input with an internal 409 (the application returns generic 500 to the browser), and allows retries after failures. Keep this state only in mock-process memory; no database or automatic retries.
- Configure mock failure simulation only in the commission quote's own `test/mock/commissionquote/config/dev.json` or `config/ci.json`: `failureMode: "random"` requires numeric `failureRate` from 0 through 1; `failureMode: "loanAmount"` uses the contract's fixed loan-amount triggers and ignores `failureRate` entirely, whether omitted or present. The dev profile uses loanAmount mode; the CI profile uses random mode with rate 0.1. Load the file selected by `-config`, defaulting to `config/dev.json` relative to the mock service root. The modes are mutually exclusive; authentication and input validation always apply. Set random mode's rate to 0 when no simulated failures are wanted. The application backend and frontend do not manage these settings. Read all commission quote runtime settings from the selected JSON file, including numeric `port`; do not use environment-variable overrides.
- Keep the vendor API key on the server. The browser calls the application backend, which calls the commission quote API with the key. Never embed the key in frontend code, browser requests, or client-exposed environment variables.
- Read the key from the file referenced by `apiKeyFile` in the selected server-side JSON profile. Resolve relative key paths against the profile directory; absolute mount paths are also supported. Local profiles reference `test/mock/data/API_KEY`, which must be ignored by Git. In production, the deployment platform mounts a secret-manager value before application startup; do not add a secret-manager client to the mock or store credentials in JSON.
- Use an explicit timeout for vendor requests and handle vendor failures and timeouts with clear application errors. Web API error codes/messages must not reveal vendor/provider names, codes, raw errors, URLs, credentials, or connection details. Handle quote-service statuses and error codes internally, but return the same HTTP 500 `INTERNAL_ERROR` payload for every quote-service failure, network/decoding failure, and timeout. Only browser input/header validation uses 400 `INVALID_REQUEST`. The frontend uses generic failure handling; keep sanitized diagnostic context only in server logs.
- Validate untrusted input at API boundaries, including invalid numbers and unsupported risk bands.
- Use `components.schemas.QuoteRequest` in `test/mock/commissionquote/api/commissionquote.openapi.json` as the authoritative field-validation definition for all tasks. Keep the human-readable rules and common acceptance cases in `docs/01-development-approach.md` synchronized with it. Require integer loan amounts from AUD 4000 through 10000000, integer terms from 12 through 360 months, and risk bands low/medium/high. Apply one generic rule set; do not classify loans or introduce product-specific validation. Validate at external boundaries and avoid repeating checks in internal layers.
- Maintain the browser-facing application specification in `api/webapi.openapi.json`; Tasks B and C use it for `/api/quotes`, the idempotency header, and application responses/errors. Keep its request/quote schemas and UUIDv5 definition synchronized with the vendor specification; each specification must be usable independently without external file references.
- Maintain the commission quote's standalone API specification in `test/mock/commissionquote/api/commissionquote.openapi.json`. Keep its schemas, authentication, responses, examples, and mock-mode behavior synchronized with the shared contract; Task A and vendor-client tests in Task B must follow it.
- The PDF does not specify routes, field types, allowed risk bands, commission formulas, rate units, monetary rounding, quote ID format, or failure probability. Document simple choices in `test/mock/commissionquote/README.md` as assumptions; do not present them as vendor requirements.

## Frontend

- Use React with TypeScript.
- Capture `loanAmount`, `loanTermInMonths`, and `riskBand`, with a **Generate Quote** button.
- Display the returned quote on success and clear loading and error states during requests and failures.
- Prevent duplicate submissions while a request is in progress.
- Use labelled, keyboard-accessible inputs and readable validation messages. Additional visual polish is optional.

## Tests and Verification

- Before merging any split task, independently verify its required outcomes against the shared API contract using deterministic mock responses or controlled test inputs. No task's acceptance may depend on another task being implemented, merged, or running. Test the real component under review; mock only its external dependencies. The commission quote itself is verified directly through its HTTP API using deterministic configuration.
- Task handoff must include reproducible commands or browser steps, mock inputs/responses, expected outcomes, and actual results. Passing this independent acceptance check makes a task ready to merge; real-service end-to-end verification belongs to the post-merge integration phase.
- After every backend change, review the changed code against all applicable rules in this file before considering the work complete. Explicitly check readability, unnecessary or repeated checks, Go declaration and function ordering, naming, formatting, API contracts, logging levels/context and duplicate errors, and secret handling; passing tests alone is not sufficient.
- Format changed Go files with `gofmt`, run the relevant tests, and check per-file coverage after production backend code changes. Fix rule violations introduced by the change before delivery, and briefly report verification results and any unresolved gaps.
- Aim for Go statement coverage greater than 90% in each production backend source file, including implementation files in `cmd/app/`, except `main.go`; 100% coverage is not required.
- The high-coverage target applies only to production code. Test code, fixtures, test utilities, and mock services under `test/`, including `test/mock/`, are exempt even if they contain their own `internal/` or `cmd/` directories. Retain meaningful functional tests for these components without a coverage threshold.
- Measure coverage with `go test ./... -coverprofile=coverage.out` and inspect coverage per file, not just the package or overall percentage. Report any files below the target and explain the remaining gaps.
- Run production backend tests with coverage from the repository root, and run mock service tests separately from `test/mock/commissionquote/` without requiring coverage; the root `./...` pattern does not include the nested mock module.
- Use meaningful unit or integration tests to reach the coverage target. Keep readability first; do not add unnecessary checks or production abstractions solely to increase coverage.
- Cover successful quote generation, invalid input, missing or invalid API keys, and vendor error and timeout handling.
- Keep automated tests repeatable; assertions must not depend on a random vendor outcome.
- Prefer Go's standard testing tools and `httptest` for HTTP checks. Do not add production abstractions solely for tests.
- Verify the browser flow for success, loading, validation, and failure states, and run the relevant backend tests and frontend build before claiming completion.

## README and Handover

- Use `test/mock/commissionquote/README.md` for the challenge's README deliverable.
- Provide step-by-step instructions for prerequisites, environment setup, starting each application component, and running tests.
- Explain the request flow, mock calculation assumptions, failure simulation, and relevant trade-offs concisely.
- Include an honest **AI Usage** section describing how AI was actually used during the challenge.
- Keep the code easy to explain and extend in the follow-up live-coding session. Do not pre-build speculative future requirements.

## Documentation

- Keep documentation concise, logically ordered, and easy to understand. Put shared task details in `docs/01-development-approach.md`; task documents contain only their own scope, implementation context, acceptance cases, and commands. Keep task-specific commands, Go test/module details, and coverage instructions out of the development approach. Link to shared rules instead of copying them into each task or repeating them across behavior, tests, and completion checklists.
- Keep machine-readable contracts in each service's `api/` directory and synchronize them with the shared document. Describe planned checks as requirements; report completion only from actual verification.

## Repository Language

- Write all repository documentation, code comments, and commit messages in English.

## Git Commit and Push

- Whenever the user requests a Git commit, prepare a short title and a brief description of the actual changes, and include both in the commit message.
- Keep the description simple and clear: use plain English and one or two sentences or a few short bullets.
- When the user requests a push, provide a concise summary of the changes being pushed.

# Codex Behavioral Guidelines

These instructions are intended to reduce common LLM coding mistakes. Merge them with project-specific instructions as needed.

Tradeoff: These guidelines bias toward caution and correctness over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

Do not assume. Do not hide confusion. Surface tradeoffs.

Before implementing:

- State important assumptions explicitly.
- If the request is unclear or has multiple reasonable interpretations, ask a concise clarifying question instead of silently choosing.
- If there is a simpler approach, mention it.
- Push back when warranted.
- If something is confusing, stop, name what is unclear, and ask.

## 2. Simplicity First

Write the minimum code that solves the problem. Do not add speculative complexity.

- Do not add features beyond what was requested.
- Do not create abstractions for single-use code.
- Do not add parameters or abstractions to production code solely for testing. If a test would require such changes, it may be omitted; use an appropriate build, lint, or manual check instead.
- Do not add flexibility, configurability, or extension points unless requested or clearly required.
- Do not add defensive error handling for impossible or irrelevant scenarios.
- If the solution becomes much larger than the problem suggests, simplify it.

Ask: would a senior engineer consider this overcomplicated? If yes, rewrite it smaller.

## 3. Surgical Changes

Touch only what is necessary. Clean up only the mess created by your own change.

When editing existing code:

- Do not “improve” adjacent code, comments, formatting, or naming unless required.
- Do not refactor unrelated code.
- Match the existing style, even if you would normally choose a different style.
- If you notice unrelated dead code or technical debt, mention it instead of deleting it.

When your change creates unused code:

- Remove imports, variables, functions, files, or tests made unused by your own edits.
- Do not remove pre-existing unused code unless explicitly asked.

Every changed line should trace directly to the user’s request.

## 4. Goal-Driven Execution

Turn tasks into verifiable goals and keep working until they are verified.

Examples:

- “Add validation” means: write or update tests for invalid inputs, then make them pass.
- “Fix the bug” means: reproduce the bug with a test or command, then fix it and verify it no longer fails.
- “Refactor X” means: preserve behavior and verify tests before and after when practical.

For multi-step tasks, use a short plan:

1. Define the change.
   Verify with: relevant test, build, lint, or manual check.

2. Implement the smallest working solution.
   Verify with: focused tests or command output.

3. Clean up only affected code.
   Verify with: final test/build/lint pass.

Strong success criteria allow independent progress. Weak criteria such as “make it work” require clarification.

## Success Signal

These guidelines are working when:

- Diffs are smaller.
- Fewer unrelated files are touched.
- Clarifying questions happen before mistakes.
- Tests or other verification steps support the final answer.
- No credentials or personal secrets are leaked.
