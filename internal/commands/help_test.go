package commands

import (
	"strings"
	"testing"

	"serverbot/internal/testutil"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestAppendCommandsKeepsSortedOrder(t *testing.T) {
	builder := &strings.Builder{}
	appendCommands(builder, map[string]string{
		"zeta":  "last",
		"alpha": "first",
		"beta":  "second",
	})

	output := builder.String()
	expected := []string{"/alpha", "/beta", "/zeta"}
	currentIndex := 0
	for _, name := range expected {
		idx := strings.Index(output, name)
		if idx == -1 {
			t.Fatalf("command %s not found in output %q", name, output)
		}
		if idx < currentIndex {
			t.Fatalf("commands not in ascending order: %q", output)
		}
		currentIndex = idx
	}
}

func TestNewHelpHandlerRendersSections(t *testing.T) {
	reg := NewRegistry(Dependencies{})
	reg.Handle("public", "public command", ScopePublic, func(ctx *Context) error { return nil })
	reg.Handle("admin", "admin command", ScopeAdminOnly, func(ctx *Context) error { return nil })

	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		Bot: bot,
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 1},
			},
		},
	}

	handler := NewHelpHandler(reg)
	if err := handler(ctx); err != nil {
		t.Fatalf("NewHelpHandler returned error: %v", err)
	}

	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected single request, got %d", len(reqs))
	}

	body := reqs[0].Values.Get("text")
	for _, needle := range []string{
		"<b>Publicos</b>",
		"<b>Solo administrador</b>",
		"- <b>/public</b> - public command",
		"- <b>/admin</b> - admin command",
		"Desarrollado por DaniMarqz",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("help message missing %q: %s", needle, body)
		}
	}
}
