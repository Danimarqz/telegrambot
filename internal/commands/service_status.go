package commands

import (
	"fmt"
	"strings"

	"serverbot/internal/system"
)

// ServiceStatus shows a short summary of a systemd service.
func ServiceStatus(ctx *Context) error {
	args := ctx.ArgsList()
	if len(args) == 0 {
		return ctx.Reply("Uso: /service_status <servicio>")
	}

	service := args[0]

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	stdout, stderr, err := ctx.Runner.Run(runCtx, "systemctl", "status", service, "--no-pager", "--lines=20")
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		return ctx.ReplyError("No se pudo consultar el estado del servicio.", fmt.Errorf("%w: %s", err, msg))
	}

	body := strings.TrimSpace(stdout)
	if body == "" {
		return ctx.Reply("Sin informacion disponible para el servicio.")
	}

	return ctx.ReplyPre(body)
}
