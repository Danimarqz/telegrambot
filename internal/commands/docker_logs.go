package commands

import (
	"fmt"
	"strings"

	"serverbot/internal/system"
)

// DockerLogs shows the last 20 log lines of a container.
func DockerLogs(ctx *Context) error {
	args := ctx.ArgsList()
	if len(args) == 0 {
		return ctx.Reply("Uso: /docker_logs <nombre_contenedor>")
	}

	container := args[0]

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", "logs", "--tail", "20", container)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		return ctx.ReplyError("No se pudieron obtener los logs.", fmt.Errorf("%w: %s", err, msg))
	}

	out := strings.TrimSpace(stdout)
	errOut := strings.TrimSpace(stderr)

	switch {
	case out != "" && errOut != "":
		return ctx.ReplyPre(out + "\n" + errOut)
	case out != "":
		return ctx.ReplyPre(out)
	case errOut != "":
		return ctx.ReplyPre(errOut)
	default:
		return ctx.Reply("Sin logs recientes.")
	}
}
