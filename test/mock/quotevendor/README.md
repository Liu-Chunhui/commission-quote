# Commission Quote App

The React frontend, Go application backend, and independent Go mock quote vendor (`quotevendor`) are connected over HTTP. See the [shared contract](../../../docs/01-development-approach.md), [backend acceptance](../../../docs/03-application-backend-task.md), and [standalone vendor API](api/commissionquote.openapi.json).

## Start the complete app in containers

Start Docker Desktop, provide the shared test key in `test/mock/data/API_KEY`, and run from the repository root:

```sh
make ci up
```

Open **http://localhost:8088**. Compose builds all three images, waits for healthy quotevendor → server → web startup, and keeps the containers running in the background. Compose mounts each Go service's JSON profile read-only and passes its path through `--config`; Dockerfiles contain no configuration files. Profiles default to `ci.json`; select alternatives with `SERVER_CONFIG` and `QUOTEVENDOR_CONFIG` as shown in the [root README](../../../README.md#run-the-web-application). The existing key is supplied through read-only Compose secret mounts. Only Nginx's web port is published, on host loopback; it forwards `/api` to the backend over the Compose network. Go and Node.js run inside build containers, so no local dependency installation is needed for this command.

Use 10000 / 36 / Medium for a 2% rate and AUD 200.00 commission. The CI mock intentionally simulates a failure with probability 0.1, uniformly choosing HTTP 400/401/409/429/500/503; the browser shows a generic error and permits a manual retry. Successful quotes replay until the mock restarts. For deterministic checks, select a copy of the mock CI profile with `failureRate` set to 0 (success) or 1 (failure). After editing a mounted profile, reload with `docker compose -f test/compose.yaml up --force-recreate --wait`, including the same configuration-file variables; no image rebuild is needed.

```sh
docker compose -f test/compose.yaml ps
docker compose -f test/compose.yaml logs server quotevendor
make ci down
```

Down removes this stack's containers, network, volumes, and locally built images, preserving the private key and shared Docker build cache. There is no database, migration step, or persistent data volume; quote state stays in mock-process memory.

Verified on 2026-09-22: all three containers became healthy; Chromium calculated AUD 200.00 and replayed the quote after reload with failure rate 0, then displayed the generic error with rate 1. These overrides were applied only inside the test container; the committed CI profile remains 0.1. Logs showed UTC timestamps and one error per service for the failed request. `make ci down` removed the containers, network, and project images while preserving the key. Backend tests/vet, mock race tests/vet, all 16 frontend tests, and container builds passed; each executable production file under `internal/` has 100% statement coverage (`main.go` is exempt).

## Start the complete app

Prerequisites: Go 1.26.4+, Node.js 22.12+, npm, Make, and Bash (`lsof` is used by `make clean`). See [required tools and setup](../../../README.md#required-tools) for installation instructions and the macOS setup script. Run from the repository root:

```sh
make dev up
```

The Makefile launches `make quote dev` (8090), `make server dev` (8080), and `make web dev` (5173). The mock selects `test/mock/quotevendor/config/dev.json`; the backend selects `confg/dev.json`. Each component handles its own startup and logs to the terminal. Ctrl+C stops all three process groups.

Provide the shared key in Git-ignored `test/mock/data/API_KEY` before startup; both dev profiles already reference it. The services read the existing file selected by `apiKeyFile`. The Makefile starts and stops the stack directly and never generates or replaces credentials. The key stays server-side.

Open **http://localhost:5173** and enter:

| Loan amount (AUD) | Loan term (months) | Risk band | Expected result |
| --- | --- | --- | --- |
| 10000 | 36 | Medium | 2% commission, AUD 200.00, and a nonempty quote ID |

Click **Generate Quote**. The browser sends `POST /api/quotes` through Vite to the application; the application calls mock `POST /quotes` with its private API key. Repeating unchanged input, including after a page reload, reuses the mock's quote while that process remains running. Amounts 100400, 100401, 100409, 100429, 100500, and 100503 exercise the six dev failures and display the same generic error; return to 10000 to recover.

## Verify the complete app

With `make dev up` running, use a second terminal at the repository root:

```sh
npm --prefix web exec -- playwright install chromium
npm --prefix web run test:integration
```

The integration check uses real browser requests through both running services. It verifies the example, replay after reload, all six dev failures, recovery, and the absence of the private key header from browser requests. It prints the path to a success screenshot saved in the system temporary directory.

For a container check, select a copy of the mock CI profile with `failureMode: "loanAmount"` using `QUOTEVENDOR_CONFIG`, then run `APP_URL=http://localhost:8088 npm --prefix web run test:integration`. This also checks exact decimal-string transmission and the AUD 200.06 result for 10003 / 36 / medium. Restore the default CI profile after testing.

Decimal migration verified on 2026-09-22: both Go suites passed with race detection and vet, production `internal/` files retained 100% statement coverage, all 22 Playwright tests passed, and frontend/container builds passed. The real container browser flow passed success, exact cents, replay, all six deterministic failures, and recovery. Precision regressions cover values beyond JavaScript's safe integer range and fractions too small for binary floats to distinguish. Random mode was checked with rate 1 across 30 retries: every response matched a documented status/code pair and became a generic application error. Tests do not require a particular random sequence or distribution.

Verified on 2026-09-21: these flows passed in Chromium. The successful quote was also submitted and visibly confirmed in Chrome, with no console errors or warnings. Ctrl+C released ports 8090, 8080, and 5173; the stack restarted successfully. CI/random-mode end-to-end simulation and other browser engines are not covered by this dev check.

Independent checks and builds, from the repository root:

```sh
make server test
make server build
make quote build
make web test
make web build
```

Run mock tests separately with `go test -race ./...` from `test/mock/quotevendor/`. `make clean` removes generated files and dependencies, including root and mock `bin/` and `gen/`, while preserving the private key. Stop `make dev up` before cleaning. Cleanup uses `lsof` to stop this worktree's Vite processes.

## Run the mock separately

Prerequisite: Go 1.24 or newer. From the repository root, enter the module and start the service. Run the remaining commands from this module directory:

```sh
cd test/mock/quotevendor
go run ./cmd
```

Before standalone startup, provision the private key in `../data/API_KEY` (`test/mock/data/API_KEY` from the repository root; ignored by Git). Both mock JSON profiles contain `"apiKeyFile": "../../data/API_KEY"`; the application profiles reference the same file. Both mock profiles set `"port": 8090`. Change this JSON setting to select another port. JSON `host` selects the listening interface: `localhost` in dev, `0.0.0.0` in CI for container networking. There are no environment-variable overrides. No database or application backend is needed for standalone mock checks.

The default profile is `config/dev.json`, relative to the working directory. To select the CI profile:

```sh
go run ./cmd -config config/ci.json
```

Configuration and the key file are read once; restart after edits or key rotation. Relative `apiKeyFile` paths resolve against the selected JSON file directory, not the shell working directory. Absolute paths are supported. Surrounding whitespace in the key file is trimmed. In production, the deployment platform mounts the value from its secret manager before starting the app; configure `apiKeyFile` to that mounted path. This service reads the file and does not implement secret-manager access or mounting. Development mode uses the `100` prefix plus a documented HTTP error status: 100400/100401/100409/100429/100500/100503 trigger 400/401/409/429/500/503. CI mode fails with probability 0.1, uniformly selecting one of these six errors, with amount triggers disabled. For deterministic random-mode checks, copy a profile, preserve its port, point `apiKeyFile` to the private file, and set `failureMode: "random"` with `failureRate` set to 0 or 1. Authentication and validation always run first.

## Verify the mock separately

```sh
gofmt -l cmd internal/app internal/httpapi internal/config
go test -race ./...
go vet ./...
go build -o /tmp/quotevendor ./cmd
curl --fail --silent http://localhost:8090/health
```

Expected: no formatting output, passing tests/vet/build, and health body `ok` without credentials. The module is independent; root `go test ./...` does not include it. The mock is exempt from the production backend coverage threshold. If a restricted environment prevents Go cache writes, set `GOCACHE` to a writable temporary directory.

For a manual quote from the mock module directory, this optional Python 3 command reads the same file without putting its value in command arguments or printing it:

```sh
python3 - <<'PY_QUOTE'
import http.client
import json
from pathlib import Path

profile_path = Path("config/dev.json")
profile = json.loads(profile_path.read_text())
key_path = profile_path.parent / profile["apiKeyFile"]
connection = http.client.HTTPConnection("localhost", profile["port"], timeout=3)
try:
    connection.request("POST", "/quotes", json.dumps({
        "loanAmount": "10000", "loanTermInMonths": 36, "riskBand": "medium"
    }), {
        "Content-Type": "application/json",
        "api-key": key_path.read_text().strip(),
        "idempotency-key": "32fafb2a-70b4-5114-9f2d-52be7286eaf5"
    })
    response = connection.getresponse()
    print(response.status, response.read().decode())
finally:
    connection.close()
PY_QUOTE
```

Expected: a nonempty `quoteId`, `commissionRate: "0.02"`, and `totalCommission: "200"`. Repeat unchanged to obtain the same quote. Changing a field under that key returns 409; use a fresh printable key for unrelated manual cases. Restarting clears stored quotes. To verify errors, use missing/wrong authentication (401), amount 3999 (400), and all six dev triggers with fresh keys (400/429).

Startup must fail for missing/invalid `port` (integer 1–65535), a missing `apiKeyFile`, unreadable/empty key file, missing/malformed profile, or invalid random-mode rate. `loanAmount` mode accepts any valid JSON value for its ignored rate. JSON logs use UTC timestamps and go to stderr with a distinct request ID per HTTP attempt, start/completion status and timing, quote generation/replay decisions, and one ERROR per detected failure. No API keys or request bodies are logged. Configuration errors use fixed messages; HTTP server startup failures include the underlying error in the `cause` field.

## Implementation and assumptions

The implemented flow is browser → application → quotevendor. Both Go services use chi routing and standard-library HTTP, configuration, JSON, and logging. The application validates browser input and returns every downstream failure as the same generic 500 response, with a three-second timeout and no automatic retries. There is no database, staff authentication, or quote history.

Routes, field limits, rates, fraction units, monetary calculation, idempotency, opaque IDs, and failure probabilities are project assumptions, not vendor requirements from the challenge. Rates are 1%/2%/3% for low/medium/high; multiply the decimal loan amount by the decimal commission rate to return exact AUD dollars. Both Go services use `shopspring/decimal`, the frontend uses `decimal.js`, and both APIs transmit monetary fields as decimal strings. Term is validated but does not affect commission.

One mutex protects the in-memory input bindings and successful quotes. Failures keep the binding and can be retried; successful replays bypass simulation. There is no TTL, persistence, cross-instance coordination, artificial delay, or automatic retry. Memory grows with distinct valid keys until restart; this is suitable for the local challenge mock.

## AI Usage

Codex assisted with implementing the React UI, Go services, configuration, tests, and local startup tooling from the supplied contracts and user-directed design choices. It ran automated checks, inspected logs, and verified a real quote in the browser. The user reviewed and refined file layout and dependency construction. No AI service is called by the application at runtime.
