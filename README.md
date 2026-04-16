# Renobot

A Go SlackOps service that manages Renovate PRs via emoji reactions and periodically posts summaries of open Renovate PRs to Slack.

## Features

- 🕐 Periodic Renovate PR summaries on a configurable cron schedule
- 📢 Publishes summaries to a Slack channel via [SlackLiner](https://github.com/its-the-vibe/SlackLiner)
- ⏳ Configurable TTL for Slack messages (default: 24 hours) — messages carry expiry metadata for automatic deletion
- 😻 Emoji reaction handling — react to a Renobot message with 😻 (`heart_eyes_cat`) to trigger `revamp merge` for the branch, or a number emoji (e.g., 3️⃣) to merge up to that many PRs
- 🐳 Lightweight Docker image built from `scratch`
- ⚙️ Configuration file + `.env` for sensitive values
- 🔗 Decoupled command execution via [Poppit](https://github.com/its-the-vibe/Poppit) — commands are dispatched through Redis for scalable, CI/CD-style execution

## Prerequisites

- [Go 1.24+](https://go.dev/) (for building locally)
- [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
- A running [SlackLiner](https://github.com/its-the-vibe/SlackLiner) instance connected to the same Redis
- A running [Poppit](https://github.com/its-the-vibe/Poppit) instance connected to the same Redis (with `revamp` available in its working directory)

## Quick Start

### 1. Create your config file

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` and set your GitHub org, Slack channel, Redis, and Poppit details:

```yaml
org: your-github-org
channel: "#renovate"
cron: "0 9 * * 1-5"   # 09:00 Mon–Fri
slack_ttl: "24h"       # delete Slack messages after 24 hours
revamp_path: revamp
redis:
  addr: "redis-host:6379"
  db: 0
  list_key: "slack_messages"
poppit:
  input_list: "poppit:notifications"
  output_channel: "poppit:command-output"
  repo: "its-the-vibe/Renobot"
  branch: "refs/heads/main"
  base_dir: "/path/to/working/dir"
slack:
  reaction_channel: "slack:reactions"
```

### 2. Create your `.env` file

```bash
cp .env.example .env
```

Set `REDIS_PASSWORD` if your Redis instance requires authentication:

```env
REDIS_PASSWORD=your-redis-password
```

### 3. Run with Docker Compose

```bash
docker-compose up -d
```

> **Note:** The `docker-compose.yml` assumes `revamp` is available to Poppit in the configured `base_dir`.

### 4. Run locally

```bash
go build -o renobot .
./renobot --config config.yaml
```

## Configuration

| Field | Description | Default |
|-------|-------------|---------|
| `org` | GitHub organisation (passed to `revamp`) | **required** |
| `channel` | Slack channel name or ID | **required** |
| `cron` | Cron schedule for summary runs | `0 9 * * 1-5` |
| `slack_ttl` | Time-to-live for Slack messages (Go duration string, e.g. `"24h"`, `"12h"`) | `24h` |
| `revamp_path` | Path to the `revamp` binary (used in Poppit command strings) | `revamp` |
| `redis.addr` | Redis server address | `localhost:6379` |
| `redis.db` | Redis database number | `0` |
| `redis.list_key` | SlackLiner message queue key | `slack_messages` |
| `poppit.input_list` | Redis list Poppit reads command payloads from | `poppit:notifications` |
| `poppit.output_channel` | Redis pub/sub channel Poppit publishes command output to | `poppit:command-output` |
| `poppit.repo` | GitHub repo identifier included in Poppit payloads | `its-the-vibe/Renobot` |
| `poppit.branch` | Git branch reference included in Poppit payloads | `refs/heads/main` |
| `poppit.base_dir` | Working directory Poppit uses when executing commands | `.` |
| `slack.reaction_channel` | Redis pub/sub channel SlackLiner publishes emoji reaction events to | `slack:reactions` |

Sensitive values are read from environment variables:

| Variable | Description |
|----------|-------------|
| `REDIS_PASSWORD` | Redis authentication password |

> **Poppit alignment:** `poppit.input_list` must match `POPPIT_SERVICE_REDIS_LIST_NAME` and `poppit.output_channel` must match `POPPIT_SERVICE_COMMAND_OUTPUT_CHANNEL` in your Poppit deployment.

## How It Works

1. On each cron tick Renobot publishes a JSON command payload to the Poppit Redis list (`poppit.input_list`). The payload asks Poppit to run `revamp summary --org <org> --branch` in the configured `base_dir`.
2. [Poppit](https://github.com/its-the-vibe/Poppit) pops the payload, executes the command, and publishes its output as JSON to the `poppit.output_channel` Redis pub/sub channel.
3. Renobot's background listener receives the branch-list output, parses it, and pushes one more Poppit payload per branch — each asking Poppit to run `revamp summary --org <org> --head <branch>`. Branch name and PR count are carried in the payload's `metadata` field.
4. Poppit executes each `--head` command and publishes the repo-list output to the same output channel.
5. Renobot's listener receives each repo-list output, formats a message, and pushes it as JSON to the SlackLiner Redis list (`redis.list_key`). Each message carries Slack metadata with `event_type: renobot` and the branch name.
6. [SlackLiner](https://github.com/its-the-vibe/SlackLiner) picks up the messages and posts them to the configured Slack channel.

### Emoji Reaction Flow

7. When a user reacts to a Renobot-posted summary message, SlackLiner enriches the Slack reaction event with the original message's metadata (including `type` and `branch`) and publishes it to the `slack.reaction_channel` Redis pub/sub channel.
8. Renobot's reaction listener receives the event and checks that the message metadata has `type: renobot` and a `branch` field.
9. Depending on the reaction emoji, Renobot dispatches one of the following commands via Poppit:
   - 😻 (`heart_eyes_cat`) → `revamp merge --org <org> --branch <branch>`
   - Number emoji (e.g., 3️⃣ / `three`) → `revamp merge --org <org> --branch <branch> --max <n>`
   - Any other reaction is silently ignored.
10. Poppit runs the merge command and publishes its output to the `poppit.output_channel`.
11. Renobot's Poppit listener receives the output and posts it as a thread reply to the original Slack message via SlackLiner.

## Project Structure

```
.
├── main.go               # Entry point, cron scheduler, wiring
├── config.go             # Config file loading
├── revamp.go             # Revamp output parsing helpers
├── poppit.go             # Poppit Redis dispatch and output listener
├── publish.go            # SlackLiner Redis queue publishing
├── reaction.go           # Slack emoji reaction event listener and handler
├── config.example.yaml   # Example configuration (commit this)
├── .env.example          # Example environment file (commit this)
├── Dockerfile            # Multi-stage build -> scratch runtime
└── docker-compose.yml    # Service definition (read-only, no Redis)
```

