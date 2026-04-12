package config

import (
	"testing"
	"time"
)

func TestLoadRequiresTelegramAndTargetSettings(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("TARGET_CUST_ID", "")
	t.Setenv("TARGET_ROUTE_ID", "")
	t.Setenv("TARGET_POINT_SEQ", "")
	t.Setenv("TARGET_POINT_NAME", "")
	t.Setenv("TARGET_TIME", "")
	t.Setenv("TARGET_DAYS", "")
	t.Setenv("REMINDER_MINUTES", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to fail when required settings are missing")
	}
}

func TestLoadParsesReminderDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.TargetCustID != 5005808 {
		t.Fatalf("unexpected target cust id: %d", cfg.TargetCustID)
	}
	if len(cfg.TargetDays) != 4 {
		t.Fatalf("unexpected target days length: %d", len(cfg.TargetDays))
	}
	if cfg.CheckInterval != time.Minute {
		t.Fatalf("unexpected default check interval: %s", cfg.CheckInterval)
	}
	if cfg.GPSRefreshInterval != 5*time.Minute {
		t.Fatalf("unexpected default gps refresh interval: %s", cfg.GPSRefreshInterval)
	}
	if !cfg.SendTestMessageOnStart {
		t.Fatal("expected startup test message to be enabled by default")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()

	env := map[string]string{
		"TELEGRAM_BOT_TOKEN": "token",
		"TELEGRAM_CHAT_ID":   "chat-id",
		"TARGET_CUST_ID":     "5005808",
		"TARGET_ROUTE_ID":    "461",
		"TARGET_POINT_SEQ":   "27",
		"TARGET_POINT_NAME":  "有謙家園",
		"TARGET_TIME":        "20:30",
		"TARGET_DAYS":        "MON,TUE,THU,FRI",
		"REMINDER_MINUTES":   "10,1",
	}

	for key, value := range env {
		t.Setenv(key, value)
	}

	t.Setenv("CHECK_INTERVAL", "")
	t.Setenv("GPS_REFRESH_INTERVAL", "")
	t.Setenv("SEND_TEST_MESSAGE_ON_START", "")
	t.Setenv("EUPFIN_BASE_URL", "")
	t.Setenv("STATE_FILE", "")
}
