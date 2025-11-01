package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"serverbot/internal/bot"
)

func main() {
	logger := log.New(os.Stdout, "serverbot: ", log.LstdFlags)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	runner := bot.New(logger)
	if err := runner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("bot stopped: %v", err)
	}
}
