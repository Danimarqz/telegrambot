# Serverbot

Telegram bot written in Go that lets you monitor and operate a Linux server from a private chat. It exposes commands for system metrics, Docker management, and routine maintenance tasks.

## Requirements

- Go 1.24 or newer
- Telegram bot token and admin chat ID
- Access to the binaries invoked by the commands (`docker`, `ping`, `ps`, `nvidia-smi`, etc.)
- Sufficient privileges to run `sudo reboot` when using `/reboot`

## Configuration

Required environment variables:

| Variable             | Description                                   |
|----------------------|-----------------------------------------------|
| `TELEGRAM_BOT_TOKEN` | Token generated through BotFather             |
| `ADMIN_ID`           | Numeric ID of the administrator chat          |

Optional environment variables:

| Variable        | Description                                                                  |
|-----------------|------------------------------------------------------------------------------|
| `DISK_TARGETS`  | Comma-separated list of mount points to monitor (defaults to `/`)            |

You can export them directly or load them from an `.env` file before starting the bot.

## Running

```bash
go run ./cmd/serverbot
```

To build a local binary:

```bash
go build -o serverbot ./cmd/serverbot
./serverbot
```

The process listens for `SIGINT` and `SIGTERM` to shut down gracefully.

## Telegram commands

Public:

- `/help` — show this command catalog
- `/stats` — system snapshot (CPU, memory, network, disks, GPU, uptime)

Admin only:

- `/top` — top CPU/memory consuming processes
- `/docker` — running containers and status
- `/docker-restart <name>` — restart a Docker container
- `/ping <host>` — connectivity test (`8.8.8.8` by default)
- `/reboot` — reboot the server (requires `sudo` privileges)

Each command is registered in a central dispatcher with middleware that validates the administrator before execution.

## System metrics

The `internal/metrics` package uses `gopsutil` and samples network/disk IO during a one-second interval. Adjust the monitored mount points via `DISK_TARGETS`. If `nvidia-smi` is unavailable, GPU stats fall back to “not available”.

## Development

- Configuration loads through `internal/app.LoadConfig()`.
- Commands are registered in `internal/bot/registerCommands`.
- `internal/system.CommandRunner` wraps `exec.CommandContext` with timeouts and stdout/stderr capture.

### Tests

```bash
go test ./...
```

Unit tests currently cover core helpers in the metrics package; extend them with handler-focused tests when adding features.

### Style and linting

- Run `gofmt` before committing.
- Keep Telegram responses ASCII-friendly; the HTML formatter escapes content when needed.
- Consider enabling `golangci-lint` for additional safety nets.

## Deployment

Run the binary under a service manager (systemd, supervisord, etc.) that exports the required environment variables. Ensure the service account has permissions for the maintenance commands exposed by the bot.
