package commands

import (
	"fmt"
	"strings"

	"serverbot/internal/system"
)

// DockerRestart restarts a specific container.
func DockerRestart(ctx *Context) error {
	args := ctx.ArgsList()
	if len(args) == 0 {
		return ctx.Reply("Uso: /docker_restart <nombre_contenedor>")
	}

	container := args[0]
	if !ctx.IsOwner() && container != "mc-server" {
		return ctx.Reply("Solo puedes reiniciar el contenedor \"mc-server\".")
	}

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", "restart", container)
	if err != nil {
		return ctx.ReplyError("No se pudo reiniciar el contenedor.", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)))
	}

	if strings.TrimSpace(stdout) == "" {
		return ctx.Reply("Contenedor reiniciado.")
	}

	return ctx.ReplyPre(stdout)
}
