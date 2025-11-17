package commands

import (
	"context"
	"testing"

	"serverbot/internal/app"
	"serverbot/internal/testutil"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestRegistryDispatchFlow(t *testing.T) {
	reg := NewRegistry(Dependencies{
		Config: app.Config{OwnerID: 999},
	})

	var sequence []string
	reg.Use(func(next Handler) Handler {
		return func(ctx *Context) error {
			sequence = append(sequence, "global")
			return next(ctx)
		}
	})

	reg.Handle("ping", "desc", ScopePublic, func(ctx *Context) error {
		sequence = append(sequence, "handler")
		if ctx.Command != "ping" {
			t.Errorf("Command = %s, want ping", ctx.Command)
		}
		if ctx.Arguments != "value" {
			t.Errorf("Arguments = %q, want value", ctx.Arguments)
		}
		return nil
	}, func(next Handler) Handler {
		return func(ctx *Context) error {
			sequence = append(sequence, "command")
			return next(ctx)
		}
	})

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "/ping value",
			Chat: &tgbotapi.Chat{ID: 999},
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 5},
			},
		},
	}

	if err := reg.Dispatch(context.Background(), nil, update); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	want := []string{"global", "command", "handler"}
	if len(sequence) != len(want) {
		t.Fatalf("sequence length = %d, want %d", len(sequence), len(want))
	}
	for i := range want {
		if sequence[i] != want[i] {
			t.Fatalf("sequence[%d] = %q, want %q", i, sequence[i], want[i])
		}
	}
}

func TestRegistryNotFound(t *testing.T) {
	reg := NewRegistry(Dependencies{})
	var called bool
	reg.SetNotFound(func(ctx *Context) error {
		called = true
		if ctx.Command != "unknown" {
			t.Errorf("Command = %s, want unknown", ctx.Command)
		}
		return nil
	})

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "/unknown arg",
			Chat: &tgbotapi.Chat{ID: 1},
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 8},
			},
		},
	}
	if err := reg.Dispatch(context.Background(), nil, update); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if !called {
		t.Fatalf("expected not-found handler to be called")
	}
}

func TestRegistryListFiltersHidden(t *testing.T) {
	reg := NewRegistry(Dependencies{})
	reg.Handle("public", "visible", ScopePublic, func(ctx *Context) error { return nil })
	reg.Handle("admin", "admin command", ScopeAdmin, func(ctx *Context) error { return nil })
	reg.Handle("owner", "owner command", ScopeOwner, func(ctx *Context) error { return nil })
	reg.HandleHidden("secret", func(ctx *Context) error { return nil })

	public := reg.List(ScopePublic)
	if len(public) != 1 || public["public"] != "visible" {
		t.Fatalf("public list = %v, want only public", public)
	}

	admin := reg.List(ScopeAdmin)
	if len(admin) != 1 {
		t.Fatalf("admin list length = %d, want 1", len(admin))
	}
	if _, ok := admin["admin"]; !ok {
		t.Fatalf("admin command missing from admin list")
	}
	if _, ok := admin["secret"]; ok {
		t.Fatalf("hidden command appeared in admin list")
	}
	if _, ok := admin["public"]; ok {
		t.Fatalf("public command should not appear in admin list")
	}
	owner := reg.List(ScopeOwner)
	if len(owner) != 1 || owner["owner"] != "owner command" {
		t.Fatalf("owner list = %v, want only owner", owner)
	}
}

func TestAdminOnlyMiddleware(t *testing.T) {
	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		AppConfig: app.Config{OwnerID: 1, AdminIDs: []int64{2}},
		Bot:       bot,
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 99},
			},
		},
	}

	var called bool
	handler := func(ctx *Context) error {
		called = true
		return nil
	}

	called = false
	if err := AdminOnly()(handler)(ctx); err != nil {
		t.Fatalf("AdminOnly() returned error: %v", err)
	}
	if called {
		t.Fatalf("handler invoked for non-admin")
	}
	if len(client.Requests()) != 1 {
		t.Fatalf("expected reply to be sent for non-admin")
	}

	ctx.Update.Message.Chat.ID = 2
	called = false
	if err := AdminOnly()(handler)(ctx); err != nil {
		t.Fatalf("AdminOnly() returned error for admin ID: %v", err)
	}
	if !called {
		t.Fatalf("handler not invoked for admin ID")
	}
	if len(client.Requests()) != 1 {
		t.Fatalf("unexpected extra request recorded for admin ID")
	}

	ctx.Update.Message.Chat.ID = 1
	called = false
	if err := AdminOnly()(handler)(ctx); err != nil {
		t.Fatalf("AdminOnly() returned error for owner: %v", err)
	}
	if !called {
		t.Fatalf("handler not invoked for owner")
	}
	if len(client.Requests()) != 1 {
		t.Fatalf("unexpected extra request recorded for owner")
	}
}
