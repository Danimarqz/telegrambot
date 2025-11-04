# Serverbot

Telegram bot written in Go that lets you monitor and operate a Linux server from a private chat. It exposes commands for system metrics, Docker management, service introspection, automated alerts, and routine maintenance tasks.

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

| Variable                   | Description                                                                                  |
|----------------------------|----------------------------------------------------------------------------------------------|
| `DISK_TARGETS`             | Comma-separated list of mount points to monitor (defaults to `/`)                            |
| `ENABLE_ALERTS`            | `true/false` toggle for automatic health alerts (default `false`)                            |
| `ALERT_INTERVAL`           | Polling interval for alerts, Go duration format (default `1m`)                               |
| `ALERT_COOLDOWN`           | Cooldown between repeated alerts of the same type (default `5m`)                             |
| `ALERT_CPU_THRESHOLD`      | CPU usage percentage that triggers an alert (default `90`)                                   |
| `ALERT_MEMORY_THRESHOLD`   | Memory usage percentage that triggers an alert (default `90`)                                |
| `ALERT_DISK_THRESHOLD`     | Disk usage percentage that triggers an alert for any monitored mount (default `90`)          |

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

- `/help` - show this command catalog
- `/stats` - system snapshot (CPU, memory, network, disks, GPU, uptime)

Admin only:

- `/top` - top CPU/memory consuming processes
- `/docker` - running containers and status
- `/docker_exec <name> <cmd>` - run an arbitrary command inside a container
- `/docker_logs <name>` - show the last 20 log lines of a container
- `/logs_suscripcion <name> [duracion]` - poll container logs for a limited time (default 1m, accepts `30s`, `2m`, etc.)
- `/docker_stats <name>` - CPU, RAM, network, and IO usage for a container
- `/docker_restart <name>` - restart a Docker container
- `/service_status <service>` - short `systemctl status` snippet
- `/ping <host>` - connectivity test (`8.8.8.8` by default)
- `/reboot` - reboot the server (requires `sudo` and a confirmation via `/reboot confirmar`)

Each command is registered in a central dispatcher with middleware that validates the administrator before execution.

## System metrics

The `internal/metrics` package uses `gopsutil` and samples network/disk IO during a one-second interval. Adjust the monitored mount points via `DISK_TARGETS`. If `nvidia-smi` is unavailable, GPU stats fall back to "not available".

## Automatic alerts

When `ENABLE_ALERTS=true`, the bot collects metrics every `ALERT_INTERVAL` and pushes a warning to the admin chat whenever CPU, RAM, or any monitored disk exceeds its threshold. Repeated alerts of the same type respect the `ALERT_COOLDOWN` window to avoid spam.

## Log subscriptions

`/logs_suscripcion` creates a temporary watcher that polls the last 20 lines of `docker logs` every 10 seconds and sends only the new content. The subscription ends automatically when the configured duration elapses or the bot stops.

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
