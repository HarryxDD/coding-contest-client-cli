# Contest Results Watcher

A terminal client for the Coding Contest System API.

It watches one contest continuously, polls API data, computes live leaderboard updates, and highlights ranking/score changes between polling cycles.

## Features

- Polls submissions from one contest
- Fetches scores for each submission
- Resolves team names
- Builds leaderboard (team score totals)
- Detects state diffs between polls:
  - new teams
  - rank changes
  - score changes
- Live terminal table rendering

## API resources used

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/contests/{contestId}/submissions` | Load all submissions in contest |
| GET | `/submissions/{submissionId}/scores` | Load all scores for each submission |
| GET | `/teams/{teamId}` | Resolve readable team names |

All requests use `Authorization: Bearer <JWT or PAT>`.

## Requirements

- Go 1.22+
- Running API server
- Valid token (JWT or PAT)
- Contest UUID

## Setup

```bash
go mod tidy
cp .env.example .env
```

## Quality checks

```bash
make lint
make test
```

- `make lint` checks formatting with `gofmt` and runs `go vet`
- `make test` runs the unit test suite

## Running

go run ./cmd/ccwatch \
  --base-url http://localhost:3000 \
  --api-prefix /api \
  --token YOUR_TOKEN \
  --contest-id YOUR_CONTEST_UUID \
  --interval 8s \
  --timeout 20s \
  --top 10