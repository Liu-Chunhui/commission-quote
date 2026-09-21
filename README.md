# Commission Quote App

A React and TypeScript web form backed by a Go application and an independent mock Commission Quote API.

## Required tools

| Tool | Purpose |
| --- | --- |
| Go 1.26.4+ | Build and run both Go services |
| Node.js 22.12+ (includes npm) | Run and build the web frontend |
| Docker Desktop | Container tooling |

## Set up the development environment

On macOS with Homebrew available, run from the repository root to install the required tools:

```sh
./scripts/setup.sh
```

The script installs missing tools using Homebrew and skips those already installed. Ensure existing Go and Node.js versions meet the requirements above.

## Run the web application

Start Docker Desktop and provide the shared test key in `test/mock/data/API_KEY`. From the repository root:

```sh
make ci up
```

Open **http://localhost:8088**. Compose builds and starts quotevendor, the backend, and the web frontend, waiting for their health checks. Compose mounts each Go service's `ci.json` profile read-only and selects it with `--config`; configuration is not baked into images. The existing API key is mounted only into those services; it is not included in images.

To select configuration files for each service:

```sh
SERVER_CONFIG=/absolute/path/server.json \
QUOTEVENDOR_CONFIG=/absolute/path/quotevendor.json make ci up
```

These variables select Compose file mounts; the services still read JSON settings. Use the CI profiles as templates, keeping the container host, ports, downstream URL, and key paths compatible with Compose networking and mounts.

Stop and clean the CI stack:

```sh
make ci down
```

This removes the stack's containers, network, volumes, and locally built images. The private key and shared Docker build cache are preserved. No database, migrations, or persistent data volumes are needed; mock quotes live in memory until restart.

For local development with Go and Node.js, use `make dev up` and open **http://localhost:5173**. It selects the dev profiles; Ctrl+C stops the three processes.

See the [challenge README](test/mock/quotevendor/README.md) for example inputs, tests, configuration, assumptions, and AI usage.

## Simulation

The mock quote API supports two mutually exclusive failure modes. Configure them in the mock's selected JSON profile; keep the other settings unchanged:

- `make ci up` uses [test/mock/quotevendor/config/ci.json](test/mock/quotevendor/config/ci.json), defaulting to random failures.
- `make dev up` uses [test/mock/quotevendor/config/dev.json](test/mock/quotevendor/config/dev.json), defaulting to loan-amount triggers.

### Trigger errors by loan amount

Set this field in the selected profile:

```json
"failureMode": "loanAmount"
```

Use a valid term such as 36 months and risk band `medium`:

| Loan amount (AUD) | Mock API result |
| --- | --- |
| 100400 | HTTP 400 — simulated bad request |
| 100429 | HTTP 429 — simulated too many requests |
| Any other valid amount, such as 10000 | Successful quote |

This mode provides repeatable manual checks. It ignores `failureRate`, even if that field is present.

### Trigger errors randomly

Set these fields in the selected profile:

```json
"failureMode": "random",
"failureRate": 0.1
```

Each valid request that is not a successful idempotency replay has a 10% chance of returning HTTP 503. This is a probability, not a guarantee of one failure every ten requests. Loan-amount triggers are disabled in this mode. `failureRate` must be a number from 0 to 1: use 0 to disable simulated failures or 1 to make every eligible request fail.

Successful quotes are replayed without another random check. To try another random outcome after success, change the amount, term, or risk band to generate a new idempotency key. Failed requests can be retried with the same input.

After editing a mounted profile, run `docker compose -f test/compose.yaml up --force-recreate --wait` to reload it without rebuilding images (include the same configuration-file variables if used). For local development, stop and restart `make dev up`. Restarting the mock clears its stored quotes. Authentication and input validation always apply before simulation. All simulated vendor errors become the same HTTP 500 response in the application, and the browser displays: "Unable to generate a quote. Please try again later."

## Highlight changes

- **Idempotency:** Identical loan details generate the same UUIDv5 key, including after a page reload. The backend forwards the key unchanged, and the mock replays successful quotes. Reusing a key with different input is rejected; failed requests can be retried. State stays in mock-process memory until restart. See [web/src/quotes.ts](web/src/quotes.ts) and [mock quote handling](test/mock/quotevendor/internal/httpapi/quote.go).
- [web/src/App.tsx](web/src/App.tsx) — Loan form, input validation, loading and error states, and quote display.
- [cmd/app/main.go](cmd/app/main.go) — Application backend entry point and startup wiring.
- [internal/httpapi/quote.go](internal/httpapi/quote.go) — Browser request validation and quote endpoint handling.
- [internal/integration/commissionquote/quote.go](internal/integration/commissionquote/quote.go) — HTTP calls to the quote vendor with a server-side API key, timeout, and downstream error handling.
- [api/webapi.openapi.json](api/webapi.openapi.json) — Browser-facing HTTP API contract for quote requests, responses, validation, and idempotency.
- [test/mock/quotevendor/](test/mock/quotevendor/) — Independently runnable mock vendor with API-key authentication, commission calculation, failure simulation, and its own [API contract](test/mock/quotevendor/api/commissionquote.openapi.json). Its entry point is [cmd/main.go](test/mock/quotevendor/cmd/main.go).
- [Makefile](Makefile) — Local startup, build, test, and cleanup commands for the frontend and both Go services.
- [test/compose.yaml](test/compose.yaml) — CI containers, health checks, private networking, and API-key file mounts.
