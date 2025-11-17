package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"serverbot/internal/app"
	"serverbot/internal/testutil"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type swapCall struct {
	wantName string
	wantArgs []string
	stdout   string
	stderr   string
	err      error
}

type swapRunner struct {
	t     *testing.T
	calls []swapCall
	idx   int
}

func (r *swapRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	if r.idx >= len(r.calls) {
		r.t.Fatalf("unexpected command %q %v", name, args)
	}

	call := r.calls[r.idx]
	r.idx++

	if call.wantName != "" && call.wantName != name {
		r.t.Fatalf("Run name = %q, want %q", name, call.wantName)
	}
	if call.wantArgs != nil && strings.Join(call.wantArgs, " ") != strings.Join(args, " ") {
		r.t.Fatalf("Run args = %v, want %v", args, call.wantArgs)
	}
	return call.stdout, call.stderr, call.err
}

func TestSwapMCNoContainer(t *testing.T) {
	runner := &swapRunner{
		t: t,
		calls: []swapCall{
			{wantName: "docker", wantArgs: []string{"ps", "--format", "{{.Names}}"}, stdout: ""},
		},
	}

	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		AppConfig: app.Config{
			CommandTimeout: time.Second,
		},
		Runner:         runner,
		Bot:            bot,
		RequestContext: context.Background(),
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 1},
			},
		},
	}

	if err := SwapMC(ctx); err != nil {
		t.Fatalf("SwapMC() returned error: %v", err)
	}

	if len(client.Requests()) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(client.Requests()))
	}
	if got := client.Requests()[0].Values.Get("text"); got != "No se detecta mc-server ni mc-server-mod en ejecución." {
		t.Fatalf("reply = %q", got)
	}
	if runner.idx != len(runner.calls) {
		t.Fatalf("unexpected runner usage: used %d calls, want %d", runner.idx, len(runner.calls))
	}
}

func TestSwapMCSwitchesContainers(t *testing.T) {
	runner := &swapRunner{
		t: t,
		calls: []swapCall{
			{wantName: "docker", wantArgs: []string{"ps", "--format", "{{.Names}}"}, stdout: "mc-server\n"},
			{wantName: "docker", wantArgs: []string{"stop", "mc-server"}, stdout: "mc-server"},
			{wantName: "docker", wantArgs: []string{"ps", "--filter", "name=mc-server", "--filter", "status=running", "--format", "{{.Names}}"}, stdout: ""},
			{wantName: "docker", wantArgs: []string{"start", "mc-server-mod"}, stdout: "mc-server-mod"},
		},
	}

	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		AppConfig: app.Config{
			CommandTimeout: time.Second,
		},
		Runner:         runner,
		Bot:            bot,
		RequestContext: context.Background(),
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 1},
			},
		},
	}

	if err := SwapMC(ctx); err != nil {
		t.Fatalf("SwapMC() returned error: %v", err)
	}

	if len(client.Requests()) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(client.Requests()))
	}
	if got := client.Requests()[0].Values.Get("text"); got != fmt.Sprintf("Se detuvo %s y se inició %s.", "mc-server", "mc-server-mod") {
		t.Fatalf("reply = %q", got)
	}
	if runner.idx != len(runner.calls) {
		t.Fatalf("unexpected runner usage: used %d calls, want %d", runner.idx, len(runner.calls))
	}
}

func TestSwapMCStartFails(t *testing.T) {
	runner := &swapRunner{
		t: t,
		calls: []swapCall{
			{wantName: "docker", wantArgs: []string{"ps", "--format", "{{.Names}}"}, stdout: "mc-server\n"},
			{wantName: "docker", wantArgs: []string{"stop", "mc-server"}, stdout: "mc-server"},
			{wantName: "docker", wantArgs: []string{"ps", "--filter", "name=mc-server", "--filter", "status=running", "--format", "{{.Names}}"}, stdout: ""},
			{
				wantName: "docker",
				wantArgs: []string{"start", "mc-server-mod"},
				stderr:   "permission denied",
				err:      errors.New("exit status 1"),
			},
		},
	}

	bot, client := testutil.NewFakeBot()
	ctx := &Context{
		AppConfig: app.Config{
			CommandTimeout: time.Second,
		},
		Runner:         runner,
		Bot:            bot,
		RequestContext: context.Background(),
		Update: tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 1},
			},
		},
	}

	if err := SwapMC(ctx); err != nil {
		t.Fatalf("SwapMC() returned error: %v", err)
	}

	if len(client.Requests()) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(client.Requests()))
	}
	if got := client.Requests()[0].Values.Get("text"); got != "No se pudo iniciar el nuevo servidor." {
		t.Fatalf("reply = %q", got)
	}
	if runner.idx != len(runner.calls) {
		t.Fatalf("unexpected runner usage: used %d calls, want %d", runner.idx, len(runner.calls))
	}
}
