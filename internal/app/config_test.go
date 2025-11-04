package app

import (
	"testing"
	"time"
)

func TestLoadConfigSuccess(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("ADMIN_ID", "123")
	t.Setenv("DISK_TARGETS", "/var , /data")
	t.Setenv("ENABLE_ALERTS", "yes")
	t.Setenv("ALERT_INTERVAL", "2m")
	t.Setenv("ALERT_COOLDOWN", "3m")
	t.Setenv("ALERT_CPU_THRESHOLD", "85.5")
	t.Setenv("ALERT_MEMORY_THRESHOLD", "80")
	t.Setenv("ALERT_DISK_THRESHOLD", "70")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}

	if cfg.Token != "token" {
		t.Errorf("Token = %q, want %q", cfg.Token, "token")
	}
	if cfg.AdminID != 123 {
		t.Errorf("AdminID = %d, want 123", cfg.AdminID)
	}
	wantTargets := []string{"/var", "/data"}
	if len(cfg.DiskTargets) != len(wantTargets) {
		t.Fatalf("DiskTargets length = %d, want %d", len(cfg.DiskTargets), len(wantTargets))
	}
	for i, target := range wantTargets {
		if cfg.DiskTargets[i] != target {
			t.Errorf("DiskTargets[%d] = %q, want %q", i, cfg.DiskTargets[i], target)
		}
	}
	if !cfg.Alerts.Enabled {
		t.Errorf("Alerts.Enabled = false, want true")
	}
	if cfg.Alerts.Interval != 2*time.Minute {
		t.Errorf("Alerts.Interval = %v, want 2m", cfg.Alerts.Interval)
	}
	if cfg.Alerts.Cooldown != 3*time.Minute {
		t.Errorf("Alerts.Cooldown = %v, want 3m", cfg.Alerts.Cooldown)
	}
	if cfg.Alerts.CPUThreshold != 85.5 {
		t.Errorf("Alerts.CPUThreshold = %v, want 85.5", cfg.Alerts.CPUThreshold)
	}
	if cfg.Alerts.MemoryThreshold != 80 {
		t.Errorf("Alerts.MemoryThreshold = %v, want 80", cfg.Alerts.MemoryThreshold)
	}
	if cfg.Alerts.DiskThreshold != 70 {
		t.Errorf("Alerts.DiskThreshold = %v, want 70", cfg.Alerts.DiskThreshold)
	}
}

func TestLoadConfigMissingValues(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("ADMIN_ID", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatalf("expected error when token and admin id missing")
	}

	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("ADMIN_ID", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatalf("expected error when admin id missing")
	}

	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("ADMIN_ID", "abc")
	if _, err := LoadConfig(); err == nil {
		t.Fatalf("expected error with invalid admin id")
	}
}

func TestParseHelpers(t *testing.T) {
	t.Run("parseDiskTargets", func(t *testing.T) {
		targets := parseDiskTargets("")
		if len(targets) != 1 || targets[0] != "/" {
			t.Fatalf("parseDiskTargets(\"\") = %v, want [/]", targets)
		}

		targets = parseDiskTargets(" /var , /data , ")
		want := []string{"/var", "/data"}
		if len(targets) != len(want) {
			t.Fatalf("parseDiskTargets() length = %d, want %d", len(targets), len(want))
		}
		for i, v := range want {
			if targets[i] != v {
				t.Errorf("parseDiskTargets()[%d] = %q, want %q", i, targets[i], v)
			}
		}
	})

	t.Run("parseBool", func(t *testing.T) {
		if !parseBool("on") {
			t.Errorf("parseBool(\"on\") = false, want true")
		}
		if parseBool("") {
			t.Errorf("parseBool(\"\") = true, want false")
		}
	})

	t.Run("parseDuration", func(t *testing.T) {
		got := parseDuration("invalid", time.Minute)
		if got != time.Minute {
			t.Errorf("parseDuration fallback = %v, want %v", got, time.Minute)
		}
		got = parseDuration("", 2*time.Minute)
		if got != 2*time.Minute {
			t.Errorf("parseDuration empty = %v, want %v", got, 2*time.Minute)
		}
	})

	t.Run("parseFloat", func(t *testing.T) {
		got := parseFloat("bad", 42.5)
		if got != 42.5 {
			t.Errorf("parseFloat fallback = %v, want 42.5", got)
		}
		got = parseFloat("", 33)
		if got != 33 {
			t.Errorf("parseFloat empty = %v, want 33", got)
		}
	})
}
