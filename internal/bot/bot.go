package bot

import (
	"context"
	"fmt"
	"log"
	"os"

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
	registry.Handle("docker_restart", "Reinicia un contenedor Docker", commands.ScopeAdminOnly, commands.DockerRestart, commands.AdminOnly())
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
