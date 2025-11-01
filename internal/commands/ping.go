package commands

import (
	"fmt"
	"runtime"
	"strings"

	"serverbot/internal/system"
)

// Ping runs a connectivity test against the given host.
func Ping(ctx *Context) error {
	target := ctx.Args()
	if target == "" {
		target = "8.8.8.8"
	}

	command, args, ok := pingCommandForOS(target)
	if !ok {
		return ctx.Reply("El comando /ping no está disponible en este sistema operativo.")
	}

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	stdout, stderr, err := ctx.Runner.Run(runCtx, command, args...)
	if err != nil {
		return ctx.ReplyError("No se pudo ejecutar ping.", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)))
	}

	if strings.TrimSpace(stdout) == "" {
		return ctx.Reply("No se obtuvo respuesta de ping.")
	}

	return ctx.ReplyPre(stdout)
}

func pingCommandForOS(target string) (string, []string, bool) {
	switch runtime.GOOS {
	case "linux", "darwin":
		return "ping", []string{"-c", "4", target}, true
	case "windows":
		return "ping", []string{"-n", "4", target}, true
	default:
		return "", nil, false
	}
}
