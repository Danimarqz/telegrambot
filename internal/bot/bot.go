package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"serverbot/internal/app"
	"serverbot/internal/commands"
	"serverbot/internal/metrics"
	"serverbot/internal/system"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Runner orchestrates the bot lifecycle.
type Runner struct {
	logger *log.Logger
}

// New constructs a Runner with the provided logger.
func New(logger *log.Logger) *Runner {
	if logger == nil {
		logger = log.New(os.Stdout, "serverbot: ", log.LstdFlags)
	}
	return &Runner{logger: logger}
}

// Run starts the bot and processes commands until the context is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return fmt.Errorf("create bot api: %w", err)
	}

	r.logger.Printf("Bot iniciado como @%s", botAPI.Self.UserName)

	commandRunner := system.NewCommandRunner()
	collector := metrics.NewCollector(metrics.Options{
		DiskTargets: cfg.DiskTargets,
	})

	deps := commands.Dependencies{
		Config: cfg,
		Runner: commandRunner,
		Logger: r.logger,
	}

	registry := commands.NewRegistry(deps)
	registerCommands(registry, collector)

	registry.SetNotFound(func(ctx *commands.Context) error {
		return ctx.Reply("Comando no reconocido.")
	})

	registry.Use(logCommand(r.logger))
	r.startAlerts(ctx, botAPI, collector, cfg)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := botAPI.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message == nil {
				continue
			}
			if !update.Message.IsCommand() {
				continue
			}
			if err := registry.Dispatch(ctx, botAPI, update); err != nil {
				r.logger.Printf("dispatch error: %v", err)
			}
		}
	}
}

func registerCommands(registry *commands.Registry, collector *metrics.Collector) {
	registry.Handle("help", "Muestra esta ayuda", commands.ScopePublic, commands.NewHelpHandler(registry))
	registry.Handle("stats", "Uso de CPU, RAM, red, discos y GPU", commands.ScopePublic, commands.NewStatsHandler(collector))

	registry.Handle("top", "Procesos con mayor uso de CPU/RAM", commands.ScopeAdminOnly, commands.Top, commands.AdminOnly())
	registry.Handle("docker", "Contenedores activos y estado", commands.ScopeAdminOnly, commands.Docker, commands.AdminOnly())
	registry.Handle("docker_exec", "Ejecuta un comando en un contenedor Docker", commands.ScopeAdminOnly, commands.DockerExec, commands.AdminOnly())
	registry.Handle("docker_logs", "Ultimas 20 lineas del log de un contenedor", commands.ScopeAdminOnly, commands.DockerLogs, commands.AdminOnly())
	registry.Handle("logs_suscripcion", "Envia actualizaciones periodicas de logs", commands.ScopeAdminOnly, commands.DockerLogsSubscribe, commands.AdminOnly())
	registry.Handle("docker_stats", "Uso de recursos de un contenedor", commands.ScopeAdminOnly, commands.DockerStats, commands.AdminOnly())
	registry.Handle("docker_restart", "Reinicia un contenedor Docker", commands.ScopeAdminOnly, commands.DockerRestart, commands.AdminOnly())
	registry.Handle("service_status", "Estado de un servicio systemd", commands.ScopeAdminOnly, commands.ServiceStatus, commands.AdminOnly())
	registry.Handle("ping", "Prueba de conectividad", commands.ScopeAdminOnly, commands.Ping, commands.AdminOnly())
	registry.Handle("reboot", "Reinicia el servidor", commands.ScopeAdminOnly, commands.Reboot, commands.AdminOnly())
}

func logCommand(logger *log.Logger) commands.Middleware {
	return func(next commands.Handler) commands.Handler {
		return func(ctx *commands.Context) error {
			if logger != nil && ctx.Update.Message != nil {
				logger.Printf("Comando recibido: /%s por %d", ctx.Command, ctx.Update.Message.Chat.ID)
			}
			return next(ctx)
		}
	}
}

func (r *Runner) startAlerts(ctx context.Context, bot *tgbotapi.BotAPI, collector *metrics.Collector, cfg app.Config) {
	if !cfg.Alerts.Enabled || bot == nil || collector == nil {
		return
	}

	if cfg.AdminID == 0 {
		return
	}

	state := make(map[string]time.Time)

	go func() {
		// goroutine (multithread)
		ticker := time.NewTicker(cfg.Alerts.Interval)
		defer ticker.Stop()
		// if the app is closed, exit the goroutine (ctx.Done())

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// each interval, runAlertCycle
				r.runAlertCycle(ctx, bot, collector, cfg, state)
			}
		}
	}()
}

func (r *Runner) runAlertCycle(ctx context.Context, bot *tgbotapi.BotAPI, collector *metrics.Collector, cfg app.Config, state map[string]time.Time) {
	alertCtx, cancel := context.WithTimeout(ctx, cfg.CommandTimeout)
	defer cancel()

	stats, err := collector.Collect(alertCtx)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("alert collect error: %v", err)
		}
		return
	}

	type alert struct {
		key  string
		body string
	}

	var alerts []alert
	if stats.CPU.Usage >= cfg.Alerts.CPUThreshold && stats.CPU.Usage > 0 {
		alerts = append(alerts, alert{
			key:  "cpu",
			body: fmt.Sprintf("[⚠️ ALERTA] CPU alta: %.1f%% (umbral %.0f%%)", stats.CPU.Usage, cfg.Alerts.CPUThreshold),
		})
	}
	if stats.Memory.UsedPercent >= cfg.Alerts.MemoryThreshold && stats.Memory.UsedPercent > 0 {
		alerts = append(alerts, alert{
			key:  "memory",
			body: fmt.Sprintf("[⚠️ ALERTA] RAM alta: %.1f%% (umbral %.0f%%)", stats.Memory.UsedPercent, cfg.Alerts.MemoryThreshold),
		})
	}
	for _, disk := range stats.Disks {
		if disk.UsedPercent >= cfg.Alerts.DiskThreshold && disk.UsedPercent > 0 {
			alerts = append(alerts, alert{
				key:  "disk:" + disk.Mount,
				body: fmt.Sprintf("[⚠️ ALERTA] Disco %s al %.1f%% (umbral %.0f%%)", disk.Mount, disk.UsedPercent, cfg.Alerts.DiskThreshold),
			})
		}
	}

	now := time.Now()
	for _, a := range alerts {
		if last, ok := state[a.key]; ok && now.Sub(last) < cfg.Alerts.Cooldown {
			continue
		}

		if err := sendAlert(bot, cfg.AdminID, a.body); err != nil && r.logger != nil {
			r.logger.Printf("alert send error: %v", err)
		} else {
			state[a.key] = now
		}
	}
}

func sendAlert(bot *tgbotapi.BotAPI, chatID int64, message string) error {
	if bot == nil || chatID == 0 || message == "" {
		return nil
	}

	msg := tgbotapi.NewMessage(chatID, message)
	_, err := bot.Send(msg)
	return err
}
