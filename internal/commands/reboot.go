package commands

import (
	"fmt"
	"strings"

	"serverbot/internal/system"
)

// Reboot triggers a server reboot.
func Reboot(ctx *Context) error {
	if err := ctx.Reply("Reiniciando servidor..."); err != nil {
		return err
	}

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	_, stderr, err := ctx.Runner.Run(runCtx, "sudo", "reboot")
	if err != nil {
		return ctx.ReplyError("No se pudo iniciar el reinicio.", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)))
	}

	return nil
}
