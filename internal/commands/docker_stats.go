package commands

import (
	"fmt"
	"strings"

	"serverbot/internal/system"
)

// DockerStats shows basic resource usage for a Docker container.
func DockerStats(ctx *Context) error {
	args := ctx.ArgsList()
	if len(args) == 0 {
		return ctx.Reply("Uso: /docker_stats <nombre_contenedor>")
	}

	container := args[0]

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", "stats", "--no-stream", "--format",
		"{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}", container)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		return ctx.ReplyError("No se pudieron obtener las estadisticas del contenedor.", fmt.Errorf("%w: %s", err, msg))
	}

	line := strings.TrimSpace(stdout)
	if line == "" {
		return ctx.Reply("No se encontraron datos para el contenedor solicitado.")
	}

	fields := strings.Split(line, "\t")
	if len(fields) < 6 {
		return ctx.ReplyPre(line)
	}

	body := fmt.Sprintf(
		"Contenedor: %s\nCPU: %s\nMemoria: %s (%s)\nRed: %s\nDisco: %s",
		fields[0], fields[1], fields[2], fields[3], fields[4], fields[5],
	)

	return ctx.ReplyPre(body)
}
