# Renobot

A Go SlackOps service that manages Renovate PRs via emoji reactions and periodically posts summaries of open Renovate PRs to Slack.

## Features

- 🕐 Periodic Renovate PR summaries on a configurable cron schedule
- 📢 Publishes summaries to a Slack channel via [SlackLiner](https://github.com/its-the-vibe/SlackLiner)
- 🏷️ Messages carry structured metadata (branch name, type) for future emoji-reaction handling
- 🐳 Lightweight Docker image built from `scratch`
- ⚙️ Configuration file + `.env` for sensitive values

## Prerequisites

- [Go 1.24+](https://go.dev/) (for building locally)
- [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
- A running [SlackLiner](https://github.com/its-the-vibe/SlackLiner) instance connected to the same Redis
- The `revamp` CLI binary installed (accessible on `PATH` or configured via `revamp_path`)

## Quick Start

### 1. Create your config file

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` and set your GitHub org, Slack channel, and Redis details:

```yaml
org: your-github-org
channel: "#renovate"
cron: "0 9 * * 1-5"   # 09:00 Mon–Fri
revamp_path: revamp
redis:
  addr: "redis-host:6379"
  db: 0
  list_key: "slack_messages"
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

> **Note:** The `docker-compose.yml` assumes `revamp` is baked into the Docker image.
> If you supply it externally, uncomment the volume mount in `docker-compose.yml`.

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
| `revamp_path` | Path to the `revamp` binary | `revamp` |
| `redis.addr` | Redis server address | `localhost:6379` |
| `redis.db` | Redis database number | `0` |
| `redis.list_key` | SlackLiner message queue key | `slack_messages` |

Sensitive values are read from environment variables:

| Variable | Description |
|----------|-------------|
| `REDIS_PASSWORD` | Redis authentication password |

## How It Works

1. On each cron tick Renobot runs `revamp summary --org <org> --branch` to get a list of open Renovate branches and their PR counts.
2. For every branch it runs `revamp summary --org <org> --head <branch>` to fetch the associated repositories.
3. It formats a message for each branch and pushes it as JSON to the SlackLiner Redis list (`slack_messages`). Each message carries Slack metadata with `event_type: renobot` and the branch name, ready for future emoji-reaction handling.
4. [SlackLiner](https://github.com/its-the-vibe/SlackLiner) picks up the messages from Redis and posts them to the configured Slack channel.

## Project Structure

```
.
├── main.go               # Entry point, cron scheduler, wiring
├── config.go             # Config file loading
├── revamp.go             # Revamp CLI invocation and output parsing
├── publish.go            # SlackLiner Redis queue publishing
├── config.example.yaml   # Example configuration (commit this)
├── .env.example          # Example environment file (commit this)
├── Dockerfile            # Multi-stage build -> scratch runtime
└── docker-compose.yml    # Service definition (read-only, no Redis)
```
