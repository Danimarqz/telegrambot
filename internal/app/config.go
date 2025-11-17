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
	OwnerID        int64
	AdminIDs       []int64
	CommandTimeout time.Duration
	DiskTargets    []string
	Alerts         AlertConfig
}

// AlertConfig contains settings for automatic alert notifications.
type AlertConfig struct {
	Enabled         bool
	Interval        time.Duration
	Cooldown        time.Duration
	CPUThreshold    float64
	MemoryThreshold float64
	DiskThreshold   float64
}

const (
	defaultCommandTimeout = 10 * time.Second
	defaultDiskTargets    = "/"
)

// LoadConfig reads the necessary environment variables and returns a validated Config.
func LoadConfig() (Config, error) {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	adminIDs := strings.TrimSpace(os.Getenv("ADMIN_IDS"))
	ownerStr := strings.TrimSpace(os.Getenv("OWNER_ID"))
	fallbackOwner := strings.TrimSpace(os.Getenv("ADMIN_ID"))
	diskTargets := strings.TrimSpace(os.Getenv("DISK_TARGETS"))
	enableAlerts := strings.TrimSpace(os.Getenv("ENABLE_ALERTS"))
	alertInterval := strings.TrimSpace(os.Getenv("ALERT_INTERVAL"))
	alertCooldown := strings.TrimSpace(os.Getenv("ALERT_COOLDOWN"))
	alertCPU := strings.TrimSpace(os.Getenv("ALERT_CPU_THRESHOLD"))
	alertMem := strings.TrimSpace(os.Getenv("ALERT_MEMORY_THRESHOLD"))
	alertDisk := strings.TrimSpace(os.Getenv("ALERT_DISK_THRESHOLD"))

	if token == "" {
		return Config{}, errors.New("missing TELEGRAM_BOT_TOKEN")
	}
	if ownerStr == "" {
		ownerStr = fallbackOwner
	}
	if ownerStr == "" {
		return Config{}, errors.New("missing OWNER_ID")
	}

	ownerID, err := strconv.ParseInt(ownerStr, 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("invalid OWNER_ID: %w", err)
	}

	adminIDList, err := parseAdminIDs(adminIDs)
	if err != nil {
		return Config{}, fmt.Errorf("invalid ADMIN_IDS: %w", err)
	}

	cfg := Config{
		Token:          token,
		OwnerID:        ownerID,
		AdminIDs:       adminIDList,
		CommandTimeout: defaultCommandTimeout,
		DiskTargets:    parseDiskTargets(diskTargets),
		Alerts: AlertConfig{
			Enabled:         parseBool(enableAlerts),
			Interval:        parseDuration(alertInterval, time.Minute),
			Cooldown:        parseDuration(alertCooldown, 5*time.Minute),
			CPUThreshold:    parseFloat(alertCPU, 90),
			MemoryThreshold: parseFloat(alertMem, 90),
			DiskThreshold:   parseFloat(alertDisk, 90),
		},
	}

	if cfg.Alerts.Interval <= 0 {
		cfg.Alerts.Interval = time.Minute
	}
	if cfg.Alerts.Cooldown <= 0 {
		cfg.Alerts.Cooldown = 5 * time.Minute
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

func parseAdminIDs(raw string) ([]int64, error) {
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			id, err := strconv.ParseInt(trimmed, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid admin ID %q: %w", trimmed, err)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func parseBool(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return strings.EqualFold(raw, "on") || strings.EqualFold(raw, "yes")
	}
	return v
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func parseFloat(raw string, fallback float64) float64 {
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}
