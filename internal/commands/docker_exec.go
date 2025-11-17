package commands

import (
	"fmt"
	"strings"
	"unicode"

	"serverbot/internal/system"
)

// DockerExec runs a command inside a Docker container.
func DockerExec(ctx *Context) error {
	raw := strings.TrimSpace(ctx.Args())
	if raw == "" {
		return ctx.Reply("Uso: /docker_exec <nombre_contenedor> <comando>")
	}

	tokens, err := splitArgs(raw)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("Argumentos invalidos: %v", err))
	}
	if len(tokens) < 2 {
		return ctx.Reply("Uso: /docker_exec <nombre_contenedor> <comando>")
	}

	container := tokens[0]
	allowed := map[string]struct{}{
		"mc-server":     {},
		"mc-server-mod": {},
	}
	if !ctx.IsOwner() {
		if _, ok := allowed[container]; !ok {
			return ctx.Reply("Solo puedes ejecutar comando en el contenedor \"mc-server\" o \"mc-server-mod\".")
		}
	}
	commandArgs := tokens[1:]

	runCtx, cancel := system.WithTimeout(ctx.RequestContext, ctx.AppConfig.CommandTimeout)
	defer cancel()

	execArgs := append([]string{"exec", container}, commandArgs...)

	stdout, stderr, err := ctx.Runner.Run(runCtx, "docker", execArgs...)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		return ctx.ReplyError("No se pudo ejecutar el comando en el contenedor.", fmt.Errorf("%w: %s", err, msg))
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
		return ctx.Reply("Comando ejecutado.")
	}
}

func splitArgs(input string) ([]string, error) {
	var args []string
	var current strings.Builder

	var quote rune
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		args = append(args, current.String())
		current.Reset()
	}

	for _, r := range input {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		return nil, fmt.Errorf("secuencia de escape incompleta")
	}
	if quote != 0 {
		return nil, fmt.Errorf("comillas sin cerrar")
	}

	flush()

	return args, nil
}
