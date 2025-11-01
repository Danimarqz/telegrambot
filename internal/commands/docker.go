package commands

import (
	"fmt"
	"strings"

	"serverbot/internal/system"
)

// Docker lists the active containers and their status.
func Docker(ctx *Context) error {
	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", "ps", "--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}")
	if err != nil {
		return ctx.ReplyError("No se pudo consultar Docker.", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)))
	}

	if strings.TrimSpace(stdout) == "" {
		return ctx.Reply("No hay contenedores activos.")
	}

	return ctx.ReplyPre(stdout)
}
