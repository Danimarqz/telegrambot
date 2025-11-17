package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"serverbot/internal/system"
)

const (
	mcServerContainer    = "mc-server"
	mcServerModContainer = "mc-server-mod"
)

// SwapMC detiene el contenedor de Minecraft activo y levanta la otra variante.
func SwapMC(ctx *Context) error {
	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	running, err := detectRunningMC(ctx, runCtx)
	if err != nil {
		return ctx.ReplyError("No se pudo leer el estado de los contenedores.", err)
	}
	if running == "" {
		return ctx.Reply("No se detecta mc-server ni mc-server-mod en ejecución.")
	}

	target := oppositeContainer(running)
	if target == "" {
		return ctx.Reply("El contenedor activo no es compatible con esta operación.")
	}

	if _, stderr, err := ctx.Runner.Run(runCtx, "docker", "stop", running); err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return ctx.ReplyError("No se pudo detener el servidor actual.", fmt.Errorf("%w: %s", err, msg))
	}

	if err := waitForContainerStop(ctx, runCtx, running); err != nil {
		return ctx.ReplyError("No se detuvo el contenedor anterior.", err)
	}

	if err := startContainer(ctx, runCtx, target); err != nil {
		return ctx.ReplyError("No se pudo iniciar el nuevo servidor.", err)
	}

	return ctx.Reply(fmt.Sprintf("Se detuvo %s y se inició %s.", running, target))
}

func detectRunningMC(ctx *Context, runCtx context.Context) (string, error) {
	stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", "ps", "--format", "{{.Names}}")
	if err != nil {
		msg := strings.TrimSpace(stderr)
		return "", fmt.Errorf("%w: %s", err, msg)
	}

	for _, line := range strings.Split(stdout, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if name == mcServerContainer || name == mcServerModContainer {
			return name, nil
		}
	}

	return "", nil
}

func waitForContainerStop(ctx *Context, runCtx context.Context, name string) error {
	const attempts = 10
	for range attempts {
		stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", "ps", "--filter", "name="+name, "--filter", "status=running", "--format", "{{.Names}}")
		if err != nil {
			msg := strings.TrimSpace(stderr)
			return fmt.Errorf("%w: %s", err, msg)
		}
		if strings.TrimSpace(stdout) == "" {
			return nil
		}

		select {
		case <-runCtx.Done():
			return runCtx.Err()
		default:
		}
		time.Sleep(1000 * time.Millisecond)
	}
	return fmt.Errorf("el contenedor %s continúa activo", name)
}

func startContainer(ctx *Context, runCtx context.Context, name string) error {
	stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", "start", name)
	if err == nil {
		return nil
	}

	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = strings.TrimSpace(stdout)
	}
	return fmt.Errorf("%w: %s", err, msg)
}

func oppositeContainer(current string) string {
	switch current {
	case mcServerContainer:
		return mcServerModContainer
	case mcServerModContainer:
		return mcServerContainer
	default:
		return ""
	}
}
