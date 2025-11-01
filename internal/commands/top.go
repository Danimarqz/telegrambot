package commands

import (
	"fmt"
	"runtime"
	"strings"

	"serverbot/internal/system"
)

// Top displays the processes with the highest CPU and memory usage.
func Top(ctx *Context) error {
	command, args, ok := topCommandForOS()
	if !ok {
		return ctx.Reply("El comando /top no está disponible en este sistema operativo.")
	}

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	stdout, stderr, err := ctx.Runner.Run(runCtx, command, args...)
	if err != nil {
		return ctx.ReplyError("No se pudo obtener la información de procesos.", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)))
	}

	if stdout == "" {
		return ctx.Reply("No se obtuvo información de procesos.")
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) > 10 {
		lines = lines[:10]
	}

	body := "PID   COMMAND         %CPU  %MEM\n" + strings.Join(lines, "\n")
	return ctx.ReplyPre(body)
}

func topCommandForOS() (string, []string, bool) {
	switch runtime.GOOS {
	case "linux", "darwin":
		return "ps", []string{"-eo", "pid,comm,%cpu,%mem", "--sort=-%cpu", "--no-headers"}, true
	default:
		return "", nil, false
	}
}
