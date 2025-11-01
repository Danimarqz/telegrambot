package app

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config groups together the configuration required by the bot.
type Config struct {
	Token          string
	AdminID        int64
	CommandTimeout time.Duration
	DiskTargets    []string
}

const (
	defaultCommandTimeout = 10 * time.Second
	defaultDiskTargets    = "/"
)

// LoadConfig reads the necessary environment variables and returns a validated Config.
func LoadConfig() (Config, error) {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	adminStr := strings.TrimSpace(os.Getenv("ADMIN_ID"))
	diskTargets := strings.TrimSpace(os.Getenv("DISK_TARGETS"))

	if token == "" {
		return Config{}, errors.New("missing TELEGRAM_BOT_TOKEN")
	}
	if adminStr == "" {
		return Config{}, errors.New("missing ADMIN_ID")
	}

	adminID, err := strconv.ParseInt(adminStr, 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("invalid ADMIN_ID: %w", err)
	}

	cfg := Config{
		Token:          token,
		AdminID:        adminID,
		CommandTimeout: defaultCommandTimeout,
		DiskTargets:    parseDiskTargets(diskTargets),
	}

	return cfg, nil
}

func parseDiskTargets(raw string) []string {
	if raw == "" {
		return []string{defaultDiskTargets}
	}

	parts := strings.Split(raw, ",")
	targets := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			targets = append(targets, trimmed)
		}
	}

	if len(targets) == 0 {
		return []string{defaultDiskTargets}
	}
	return targets
}
