package commands

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"serverbot/internal/app"
	"serverbot/internal/testutil"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type fakeRunner struct {
	t        *testing.T
	wantName string
	wantArgs []string
	stdout   string
	stderr   string
	err      error
	called   bool
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	f.called = true
	if f.wantName != "" && name != f.wantName {
		f.t.Fatalf("Run name = %q, want %q", name, f.wantName)
	}
	if f.wantArgs != nil && strings.Join(args, " ") != strings.Join(f.wantArgs, " ") {
		f.t.Fatalf("Run args = %v, want %v", args, f.wantArgs)
	}
	return f.stdout, f.stderr, f.err
}

func TestPingSuccessRepliesWithOutput(t *testing.T) {
	target := "example.com"
	command, args, ok := pingCommandForOS(target)
	if !ok {
		t.Skip("ping not supported on this OS")
	}

	runner := &fakeRunner{
		t:        t,
		wantName: command,
		wantArgs: args,
		stdout:   "reply line\n",
	}

	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		AppConfig: app.Config{
			CommandTimeout: time.Second,
		},
		Runner: runner,
		Bot:    bot,
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 1},
			},
		},
		RequestContext: context.Background(),
		Arguments:      target,
	}

	if err := Ping(ctx); err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}
	if !runner.called {
		t.Fatalf("Runner.Run was not called")
	}

	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if got := reqs[0].Values.Get("text"); got != "<pre>reply line</pre>" {
		t.Fatalf("reply text = %q, want escaped pre block", got)
	}
}

func TestPingNoOutput(t *testing.T) {
	command, args, ok := pingCommandForOS("8.8.8.8")
	if !ok {
		t.Skip("ping not supported on this OS")
	}

	runner := &fakeRunner{
		t:        t,
		wantName: command,
		wantArgs: args,
		stdout:   "",
	}

	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		AppConfig: app.Config{
			CommandTimeout: time.Second,
		},
		Runner: runner,
		Bot:    bot,
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 1},
			},
		},
		RequestContext: context.Background(),
	}

	if err := Ping(ctx); err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}

	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(reqs))
	}
	if got := reqs[0].Values.Get("text"); got != "No se obtuvo respuesta de ping." {
		t.Fatalf("fallback message = %q, want no response", got)
	}
}

func TestPingRunnerError(t *testing.T) {
	command, args, ok := pingCommandForOS("8.8.4.4")
	if !ok {
		t.Skip("ping not supported on this OS")
	}

	runner := &fakeRunner{
		t:        t,
		wantName: command,
		wantArgs: args,
		stdout:   "",
		stderr:   "permission denied",
		err:      errors.New("exit status 1"),
	}

	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		AppConfig: app.Config{
			CommandTimeout: time.Second,
		},
		Runner: runner,
		Logger: testLogger(),
		Bot:    bot,
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 1},
			},
		},
		RequestContext: context.Background(),
		Arguments:      "8.8.4.4",
	}

	if err := Ping(ctx); err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}

	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(reqs))
	}
	if got := reqs[0].Values.Get("text"); got != "No se pudo ejecutar ping." {
		t.Fatalf("error message = %q, want ping failure", got)
	}
}

func testLogger() *log.Logger {
	return log.New(&bytes.Buffer{}, "", 0)
}
