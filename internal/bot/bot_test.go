package bot

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"serverbot/internal/app"
	"serverbot/internal/commands"
	"serverbot/internal/metrics"
	"serverbot/internal/testutil"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestNewNilLogger(t *testing.T) {
	r := New(nil)
	if r.logger == nil {
		t.Fatalf("logger is nil")
	}
	if prefix := r.logger.Prefix(); prefix != "serverbot: " {
		t.Fatalf("logger prefix = %q, want \"serverbot: \"", prefix)
	}
}

func TestLogCommandMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	ctx := &commands.Context{
		Command: "ping",
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 555},
			},
		},
	}

	var called bool
	err := logCommand(logger)(func(ctx *commands.Context) error {
		called = true
		return nil
	})(ctx)
	if err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}
	if !called {
		t.Fatalf("next handler was not called")
	}
	if !strings.Contains(buf.String(), "/ping") || !strings.Contains(buf.String(), "555") {
		t.Fatalf("log output %q does not contain command information", buf.String())
	}
}

func TestSendAlert(t *testing.T) {
	if err := sendAlert(nil, 1, "msg"); err != nil {
		t.Fatalf("sendAlert with nil bot returned error: %v", err)
	}
	bot, client := testutil.NewFakeBot()
	if err := sendAlert(bot, 0, "msg"); err != nil {
		t.Fatalf("sendAlert with zero chat returned error: %v", err)
	}
	if err := sendAlert(bot, 1, ""); err != nil {
		t.Fatalf("sendAlert with empty message returned error: %v", err)
	}

	if len(client.Requests()) != 0 {
		t.Fatalf("unexpected request captured before valid call")
	}

	if err := sendAlert(bot, 777, "alert!"); err != nil {
		t.Fatalf("sendAlert valid call returned error: %v", err)
	}

	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Endpoint != "sendMessage" {
		t.Fatalf("endpoint = %s, want sendMessage", reqs[0].Endpoint)
	}
	if reqs[0].Values.Get("chat_id") != "777" {
		t.Fatalf("chat_id = %s, want 777", reqs[0].Values.Get("chat_id"))
	}
	if reqs[0].Values.Get("text") != "alert!" {
		t.Fatalf("text = %q, want alert!", reqs[0].Values.Get("text"))
	}
}

func TestRegisterCommandsCatalog(t *testing.T) {
	reg := commands.NewRegistry(commands.Dependencies{
		Config: app.Config{AdminID: 123},
	})
	collector := metrics.NewCollector(metrics.Options{})

	registerCommands(reg, collector)

	public := reg.List(commands.ScopePublic)
	if len(public) != 2 {
		t.Fatalf("public commands = %v, want exactly help and stats", public)
	}
	if _, ok := public["help"]; !ok {
		t.Fatalf("help command missing from public list")
	}
	if _, ok := public["stats"]; !ok {
		t.Fatalf("stats command missing from public list")
	}

	admin := reg.List(commands.ScopeAdminOnly)
	expectedAdmin := []string{"top", "docker", "docker_exec", "docker_logs", "logs_suscripcion", "docker_stats", "docker_restart", "service_status", "ping", "reboot"}
	if len(admin) != len(expectedAdmin) {
		t.Fatalf("admin commands length = %d, want %d", len(admin), len(expectedAdmin))
	}
	for _, name := range expectedAdmin {
		if _, ok := admin[name]; !ok {
			t.Fatalf("admin command %q missing from registry", name)
		}
	}
}
