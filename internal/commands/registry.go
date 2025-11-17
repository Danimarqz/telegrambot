package commands

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"serverbot/internal/app"
	"serverbot/internal/system"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Handler processes a command request.
type Handler func(ctx *Context) error

// Middleware wraps a handler to inject pre/post logic.
type Middleware func(Handler) Handler

// registeredCommand stores metadata for a registered command.
type registeredCommand struct {
	Handler      Handler
	Description  string
	Middlewares  []Middleware
	Scope        CommandScope
	HideFromHelp bool
}

// CommandScope describes the visibility of a command.
type CommandScope int

const (
	ScopePublic CommandScope = iota
	ScopeAdmin
	ScopeOwner
)

// Dependencies groups shared dependencies provided to handlers.
type Dependencies struct {
	Config app.Config
	Runner system.Runner
	Logger *log.Logger
}

type Registry struct {
	deps       Dependencies
	commands   map[string]registeredCommand
	notFound   Handler
	middleware []Middleware
}

// NewRegistry creates a registry with the supplied dependencies.
func NewRegistry(deps Dependencies) *Registry {
	return &Registry{
		deps:     deps,
		commands: make(map[string]registeredCommand),
	}
}

// Use appends global middleware applied to every handler.
func (r *Registry) Use(mw ...Middleware) {
	r.middleware = append(r.middleware, mw...)
}

// Handle registers a command with its metadata.
func (r *Registry) Handle(name string, description string, scope CommandScope, handler Handler, middlewares ...Middleware) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return
	}

	r.commands[name] = registeredCommand{
		Handler:     handler,
		Description: description,
		Middlewares: middlewares,
		Scope:       scope,
	}
}

// HandleHidden registers a command that should not appear in /help.
func (r *Registry) HandleHidden(name string, handler Handler, middlewares ...Middleware) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return
	}

	r.commands[name] = registeredCommand{
		Handler:      handler,
		Middlewares:  middlewares,
		HideFromHelp: true,
	}
}

// SetNotFound sets the fallback handler for unknown commands.
func (r *Registry) SetNotFound(handler Handler) {
	r.notFound = handler
}

// Dispatch resolves and executes the command referenced by the update.
func (r *Registry) Dispatch(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.Message == nil {
		return errors.New("update has no message")
	}
	if !update.Message.IsCommand() {
		return errors.New("message is not a command")
	}

	name := strings.ToLower(update.Message.Command())
	entry, ok := r.commands[name]
	if !ok {
		if r.notFound != nil {
			return r.notFound(r.buildContext(ctx, bot, update, name, update.Message.CommandArguments()))
		}
		return fmt.Errorf("command %q not found", name)
	}

	handler := entry.Handler
	for i := len(entry.Middlewares) - 1; i >= 0; i-- {
		handler = entry.Middlewares[i](handler)
	}
	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}

	return handler(r.buildContext(ctx, bot, update, name, update.Message.CommandArguments()))
}

// List returns visible commands for the given scope.
func (r *Registry) List(scope CommandScope) map[string]string {
	result := make(map[string]string)
	for name, cmd := range r.commands {
		if cmd.HideFromHelp {
			continue
		}
		if cmd.Scope != scope {
			continue
		}
		if cmd.Description != "" {
			result[name] = cmd.Description
		}
	}
	return result
}

func (r *Registry) buildContext(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update, command, args string) *Context {
	return &Context{
		AppConfig:      r.deps.Config,
		Runner:         r.deps.Runner,
		Logger:         r.deps.Logger,
		RequestContext: ctx,
		Bot:            bot,
		Update:         update,
		Command:        command,
		Arguments:      strings.TrimSpace(args),
	}
}

// AdminOnly middleware rejects non-admin requests.
func AdminOnly() Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			if !ctx.IsAdmin() {
				return ctx.Reply("No autorizado.")
			}
			return next(ctx)
		}
	}
}

// OwnerOnly middleware rejects non-owner requests.
func OwnerOnly() Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			if !ctx.IsOwner() {
				return ctx.Reply("No autorizado.")
			}
			return next(ctx)
		}
	}
}
