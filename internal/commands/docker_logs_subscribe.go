package commands

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"serverbot/internal/system"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	defaultLogSubscriptionDuration = time.Minute
	logSubscriptionPollInterval    = 10 * time.Second
)

// DockerLogsSubscribe streams recent logs from a container for a limited time.
func DockerLogsSubscribe(ctx *Context) error {
	args := ctx.ArgsList()
	if len(args) == 0 {
		return ctx.Reply("Uso: /logs_suscripcion <nombre_contenedor> [duracion]")
	}

	container := args[0]
	duration := defaultLogSubscriptionDuration
	if len(args) > 1 {
		if parsed, err := time.ParseDuration(args[1]); err == nil && parsed > 0 {
			duration = parsed
		}
	}

	chatID := ctx.Update.Message.Chat.ID
	bot := ctx.Bot
	runner := ctx.Runner
	logger := ctx.Logger
	timeout := ctx.AppConfig.CommandTimeout

	subscriptionCtx, cancel := context.WithTimeout(ctx.RequestContext, duration)

	go func() {
		defer cancel()
		streamErr := streamDockerLogs(subscriptionCtx, bot, runner, chatID, container, timeout)
		if streamErr != nil {
			if logger != nil {
				logger.Printf("docker logs subscription error: %v", streamErr)
			}
			errNotify := notifyError(bot, chatID, fmt.Sprintf("Suscripcion cancelada: %v", streamErr))
			if errNotify != nil && logger != nil {
				logger.Printf("failed to send error notification: %v", errNotify)
			}
		}
	}()

	return ctx.Reply(fmt.Sprintf("Suscripcion iniciada a los logs de %s durante %s.", container, duration))
}

func streamDockerLogs(ctx context.Context, bot *tgbotapi.BotAPI, runner system.Runner, chatID int64, container string, timeout time.Duration) error {
	initial, err := fetchLogLines(ctx, runner, container, timeout)
	if err != nil {
		return err
	}

	if len(initial) == 0 {
		if err := notifyInfo(bot, chatID, fmt.Sprintf("Sin logs recientes para %s.", container)); err != nil {
			return err
		}
	} else {
		if err := sendLines(bot, chatID, fmt.Sprintf("Logs iniciales de %s:", container), initial); err != nil {
			return err
		}
	}

	lastLine := ""
	if len(initial) > 0 {
		lastLine = initial[len(initial)-1]
	}

	ticker := time.NewTicker(logSubscriptionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return notifyInfo(bot, chatID, fmt.Sprintf("Fin de la suscripcion a logs de %s.", container))
		case <-ticker.C:
			lines, err := fetchLogLines(ctx, runner, container, timeout)
			if err != nil {
				return err
			}
			if len(lines) == 0 {
				continue
			}

			newLines := extractNewLines(lastLine, lines)
			if len(newLines) == 0 {
				lastLine = lines[len(lines)-1]
				continue
			}

			if err := sendLines(bot, chatID, fmt.Sprintf("Actualizacion de %s:", container), newLines); err != nil {
				return err
			}
			lastLine = lines[len(lines)-1]
		}
	}
}

func fetchLogLines(parent context.Context, runner system.Runner, container string, timeout time.Duration) ([]string, error) {
	runCtx, cancel := system.WithTimeout(parent, timeout)
	defer cancel()

	stdout, stderr, err := runner.Run(runCtx, "docker", "logs", "--tail", "20", container)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		return nil, fmt.Errorf("%w: %s", err, msg)
	}

	body := strings.TrimSpace(stdout)
	if body == "" {
		return nil, nil
	}

	lines := strings.Split(body, "\n")
	return lines, nil
}

func extractNewLines(previous string, lines []string) []string {
	if previous == "" {
		return lines
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] == previous {
			if i+1 < len(lines) {
				return lines[i+1:]
			}
			return nil
		}
	}
	return lines
}

func sendLines(bot *tgbotapi.BotAPI, chatID int64, header string, lines []string) error {
	content := strings.Join(lines, "\n")
	return notifyPre(bot, chatID, fmt.Sprintf("%s\n%s", header, content))
}

func notifyError(bot *tgbotapi.BotAPI, chatID int64, message string) error {
	return notifyText(bot, chatID, fmt.Sprintf("[ALERTA] %s", message))
}

func notifyInfo(bot *tgbotapi.BotAPI, chatID int64, message string) error {
	return notifyText(bot, chatID, message)
}

func notifyText(bot *tgbotapi.BotAPI, chatID int64, message string) error {
	if bot == nil || message == "" {
		return nil
	}
	msg := tgbotapi.NewMessage(chatID, message)
	_, err := bot.Send(msg)
	return err
}

func notifyPre(bot *tgbotapi.BotAPI, chatID int64, content string) error {
	if bot == nil || content == "" {
		return nil
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("<pre>%s</pre>", html.EscapeString(strings.TrimSpace(content))))
	msg.ParseMode = "HTML"
	_, err := bot.Send(msg)
	return err
}
