# Contest Results Watcher

Contest Results Watcher is a terminal client for the Coding Contest System API.

It watches one contest continuously, polls API data, computes live leaderboard updates, and highlights ranking/score changes between polling cycles.

## Overview

This client is built for contest organizers and participants who need a live view of contest progress without a browser. It provides:

- contest browsing and contest details
- live leaderboard updates
- submission drill-down with score details
- PAT management for non-interactive access

The client uses a Bubble Tea terminal UI and follows a screen-based navigation flow controlled by keyboard shortcuts.

## External libraries

| Library | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | Terminal UI framework |
| `github.com/charmbracelet/lipgloss` | Terminal styling and layout helpers |

All other code in this repository was implemented from scratch for the project.

## API resources used

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/contests` | Load available contests |
| GET | `/contests/{contestId}/submissions` | Load all submissions in a contest |
| GET | `/contests/{contestId}/submissions/{submissionId}` | Load a single submission with metadata |
| GET | `/submissions/{submissionId}/scores` | Load all scores for a submission |
| GET | `/teams/{teamId}` | Resolve readable team names |

All requests use `Authorization: Bearer <JWT or PAT>`.

## Requirements

- Go 1.22+
- Running API server
- Valid token (JWT or PAT)
- Contest UUID

## Setup and installation

```bash
go mod tidy
cp .env.example .env
```

If needed, adjust the values in `.env` or pass equivalent CLI flags when starting the client.

## Configuration

The client accepts CLI flags and environment variables.

Environment variables:

- `CCS_BASE_URL` — API server URL
- `CCS_API_PREFIX` — API path prefix, default: `/api`
- `CCS_INTERVAL` — polling interval, default: `8s`
- `CCS_TIMEOUT` — request timeout, default: `20s`
- `CCS_TOP_N` — leaderboard size, default: `10`

CLI flags override environment variables.

## Running the client

```bash
go run ./cmd/ccwatch \
  --base-url http://localhost:3000 \
  --api-prefix /api \
  --token YOUR_TOKEN \
  --contest-id YOUR_CONTEST_UUID \
  --interval 8s \
  --timeout 20s \
  --top 10
```

## Quality checks and tests

```bash
make lint
make test
make build
make fmt
```

- `make lint` checks formatting with `gofmt` and runs `go vet`
- `make test` runs the unit test suite
- `make build` compiles the binary
- `make fmt` applies automatic formatting

## Testing scope

The repository includes tests for the main client components, including:

- API client behavior
- authentication and session handling
- configuration validation
- leaderboard aggregation and diff detection

## Documentation note

Public methods and functions are documented in the source code with short descriptions, parameters, return values, and expected failure cases where relevant.
