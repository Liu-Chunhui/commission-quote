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

From the repository root:

```sh
make dev up
```

Before startup, provide the shared key in Git-ignored `test/mock/data/API_KEY`, as referenced by both dev profiles. The services read this existing file; the startup command does not generate or replace it. Open **http://localhost:5173**. The command installs frontend dependencies when needed and starts the mock API (8090), application backend (8080), and frontend (5173). Press Ctrl+C to stop all three components.

See the [challenge README](test/mock/commissionquote/README.md) for example inputs, tests, configuration, assumptions, and AI usage.
