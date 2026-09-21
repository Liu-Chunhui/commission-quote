# Commission Quote App: Mock Service

Task A is implemented independently. Application backend, browser UI, and end-to-end integration are separate tasks. See the [shared contract](../../../docs/01-development-approach.md), [Task A acceptance](../../../docs/02-commission-quote-task.md), and [standalone vendor API](api/commissionquote.openapi.json).

## Run

Prerequisite: Go 1.24 or newer. From the repository root, enter the module and start the service. Run the remaining commands from this module directory:

```sh
cd test/mock/commissionquote
go run ./cmd
```

Before startup, provision the private key in `../data/API_KEY` (`test/mock/data/API_KEY` from the repository root; ignored by Git). Both JSON profiles contain `"apiKeyFile": "../../data/API_KEY"`. The future application backend must reference the same private key file. Both profiles set `"port": 8090`. Change this JSON setting to select another port. The service listens on localhost; there are no environment-variable overrides. No database or application backend is needed.

The default profile is `config/dev.json`, relative to the working directory. To select the CI profile:

```sh
go run ./cmd -config config/ci.json
```

Configuration and the key file are read once; restart after edits or key rotation. Relative `apiKeyFile` paths resolve against the selected JSON file directory, not the shell working directory. Absolute paths are supported. Surrounding whitespace in the key file is trimmed. In production, the deployment platform mounts the value from its secret manager before starting the app; configure `apiKeyFile` to that mounted path. This service reads the file and does not implement secret-manager access or mounting. Development mode returns simulated 400/429 errors for amounts 100400/100429. CI mode returns 503 with probability 0.1, with amount triggers disabled. For deterministic random-mode checks, copy a profile, preserve its port, point `apiKeyFile` to the private file, and set `failureMode: "random"` with `failureRate` set to 0 or 1. Authentication and validation always run first.

## Verify

```sh
gofmt -l cmd internal/app internal/httpapi internal/config
go test -race ./...
go vet ./...
go build -o /tmp/commissionquote-vendor ./cmd
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
        "loanAmount": 10000, "loanTermInMonths": 36, "riskBand": "medium"
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

Expected: a nonempty `quoteId`, `commissionRate: 0.02`, and `totalCommission: 200`. Repeat unchanged to obtain the same quote. Changing a field under that key returns 409; use a fresh printable key for unrelated manual cases. Restarting clears stored quotes. To verify errors, use missing/wrong authentication (401), amount 3999 (400), and the two dev triggers with fresh keys (400/429).

Startup must fail for missing/invalid `port` (integer 1–65535), a missing `apiKeyFile`, unreadable/empty key file, missing/malformed profile, or invalid random-mode rate. `loanAmount` mode accepts any valid JSON value for its ignored rate. JSON logs use UTC timestamps and go to stderr with a distinct request ID per HTTP attempt, start/completion status and timing, quote generation/replay decisions, and one ERROR per detected failure. No API keys or request bodies are logged. Configuration errors use fixed messages; HTTP server startup failures include the underlying error in the `cause` field.

## Implementation and assumptions

The intended flow is browser → application → commission quote; only the mock is delivered here. The mock uses chi routing and standard-library configuration, JSON, logging, randomness, and synchronization.

Routes, field limits, rates, fraction units, monetary calculation, idempotency, opaque IDs, and failure probabilities are project assumptions, not vendor requirements from the challenge. Rates are 1%/2%/3% for low/medium/high; calculate integer cents as loan amount times 1/2/3, then return AUD dollars. Term is validated but does not affect commission.

One mutex protects the in-memory input bindings and successful quotes. Failures keep the binding and can be retried; successful replays bypass simulation. There is no TTL, persistence, cross-instance coordination, artificial delay, or automatic retry. Memory grows with distinct valid keys until restart; this is suitable for the local challenge mock.
