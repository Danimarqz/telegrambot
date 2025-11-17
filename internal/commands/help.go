package commands

import (
	"sort"
	"strings"
)

// NewHelpHandler builds a handler that renders the command catalog.
func NewHelpHandler(registry *Registry) Handler {
	return func(ctx *Context) error {
		public := registry.List(ScopePublic)
		admin := registry.List(ScopeAdmin)
		owner := registry.List(ScopeOwner)

		var builder strings.Builder
		builder.WriteString("<b>Comandos disponibles</b>\n\n")

		if len(public) > 0 {
			builder.WriteString("<b>Publicos</b>\n")
			appendCommands(&builder, public)
			builder.WriteByte('\n')
		}

		if len(admin) > 0 {
			builder.WriteString("<b>Solo administrador</b>\n")
			appendCommands(&builder, admin)
			builder.WriteByte('\n')
		}

		if len(owner) > 0 {
			builder.WriteString("<b>Solo propietario</b>\n")
			appendCommands(&builder, owner)
			builder.WriteByte('\n')
		}

		builder.WriteString("<i>Desarrollado por DaniMarqz - Go + Telegram API</i>")
		return ctx.ReplyHTML(builder.String(), false)
	}
}

func appendCommands(builder *strings.Builder, commands map[string]string) {
	if len(commands) == 0 {
		return
	}

	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		builder.WriteString("- <b>/")
		builder.WriteString(name)
		builder.WriteString("</b> - ")
		builder.WriteString(commands[name])
		builder.WriteByte('\n')
	}
}
