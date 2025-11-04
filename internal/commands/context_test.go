package commands

import (
	"bytes"
	"context"
	"log"
	"testing"

	"serverbot/internal/app"
	"serverbot/internal/testutil"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newTestMessage() *tgbotapi.Message {
	return &tgbotapi.Message{
		MessageID: 10,
		Chat: &tgbotapi.Chat{
			ID: 999,
		},
		From: &tgbotapi.User{
			ID: 321,
		},
		Text: "/cmd arg1 arg2",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 4},
		},
	}
}

func newContext(bot *tgbotapi.BotAPI) *Context {
	var buf bytes.Buffer
	return &Context{
		AppConfig: app.Config{
			AdminID: 999,
		},
		Logger:         log.New(&buf, "", 0),
		RequestContext: context.Background(),
		Bot:            bot,
		Update: tgbotapi.Update{
			Message: newTestMessage(),
		},
		Command:   "cmd",
		Arguments: " arg1 arg2 ",
	}
}

func TestContextArgsList(t *testing.T) {
	ctx := &Context{Arguments: "   foo   bar   "}
	args := ctx.ArgsList()
	if len(args) != 2 || args[0] != "foo" || args[1] != "bar" {
		t.Fatalf("ArgsList() = %v, want [foo bar]", args)
	}
	if ctx.Args() != "   foo   bar   " {
		t.Fatalf("Args() = %q, want original string", ctx.Args())
	}
}

func TestReplyPreEscapesContent(t *testing.T) {
	bot, client := testutil.NewFakeBot()
	ctx := newContext(bot)

	if err := ctx.ReplyPre("<payload>"); err != nil {
		t.Fatalf("ReplyPre() error = %v", err)
	}

	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Endpoint != "sendMessage" {
		t.Fatalf("endpoint = %s, want sendMessage", reqs[0].Endpoint)
	}
	if got := reqs[0].Values.Get("text"); got != "<pre>&lt;payload&gt;</pre>" {
		t.Fatalf("text = %q, want escaped pre block", got)
	}
	if mode := reqs[0].Values.Get("parse_mode"); mode != "HTML" {
		t.Fatalf("parse_mode = %q, want HTML", mode)
	}
}

func TestReplyAndEdit(t *testing.T) {
	bot, client := testutil.NewFakeBot(
		testutil.FakeResponse{Body: `{"ok":true,"result":{"message_id":42,"chat":{"id":999}}}`},
		testutil.FakeResponse{Body: `{"ok":true,"result":{"message_id":42,"chat":{"id":999}}}`},
	)
	ctx := newContext(bot)

	if err := ctx.ReplyAndEdit("Procesando...", "<b>done</b>"); err != nil {
		t.Fatalf("ReplyAndEdit() error = %v", err)
	}

	reqs := client.Requests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}
	if reqs[0].Endpoint != "sendMessage" {
		t.Fatalf("first endpoint = %s, want sendMessage", reqs[0].Endpoint)
	}
	if got := reqs[0].Values.Get("text"); got != "Procesando..." {
		t.Fatalf("placeholder text = %q, want Procesando...", got)
	}
	if reqs[1].Endpoint != "editMessageText" {
		t.Fatalf("second endpoint = %s, want editMessageText", reqs[1].Endpoint)
	}
	if got := reqs[1].Values.Get("text"); got != "<b>done</b>" {
		t.Fatalf("edited text = %q, want <b>done</b>", got)
	}
	if mode := reqs[1].Values.Get("parse_mode"); mode != "HTML" {
		t.Fatalf("parse_mode = %q, want HTML", mode)
	}
}

func TestReplyHTMLWithEscape(t *testing.T) {
	bot, client := testutil.NewFakeBot()
	ctx := newContext(bot)

	if err := ctx.ReplyHTML("<b>raw</b>", true); err != nil {
		t.Fatalf("ReplyHTML() error = %v", err)
	}

	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if got := reqs[0].Values.Get("text"); got != "&lt;b&gt;raw&lt;/b&gt;" {
		t.Fatalf("escaped text = %q, want &lt;b&gt;raw&lt;/b&gt;", got)
	}
}

func TestReplyWithoutMessage(t *testing.T) {
	ctx := &Context{}
	if err := ctx.Reply("hola"); err == nil {
		t.Fatalf("expected error replying without message context")
	}
}

func TestIsAdmin(t *testing.T) {
	ctx := &Context{
		AppConfig: app.Config{AdminID: 42},
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 42},
			},
		},
	}
	if !ctx.IsAdmin() {
		t.Fatalf("IsAdmin() = false, want true")
	}
	ctx.Update.Message.Chat.ID = 100
	if ctx.IsAdmin() {
		t.Fatalf("IsAdmin() = true, want false for non-admin")
	}
}
