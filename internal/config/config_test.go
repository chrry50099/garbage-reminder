package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRequiresTelegramHAAndTargetSettings(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("TARGET_CUST_ID", "")
	t.Setenv("TARGET_ROUTE_ID", "")
	t.Setenv("TARGET_POINT_SEQ", "")
	t.Setenv("TARGET_POINT_NAME", "")
	t.Setenv("ALERT_OFFSETS", "")
	t.Setenv("REMINDER_MINUTES", "")
	t.Setenv("HA_BASE_URL", "")
	t.Setenv("HA_TOKEN", "")
	t.Setenv("HA_NOTIFY_MODE", "")
	t.Setenv("HA_TTS_TARGET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to fail when required settings are missing")
	}
}

func TestLoadUsesSupervisorDefaultsWhenAvailable(t *testing.T) {
	t.Setenv("SUPERVISOR_TOKEN", "supervisor-token")
	t.Setenv("HA_BASE_URL", "")
	t.Setenv("HA_TOKEN", "")
	setRequiredEnvWithoutHA(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.HABaseURL != "http://supervisor/core" {
		t.Fatalf("unexpected HA base URL: %s", cfg.HABaseURL)
	}
	if cfg.HAToken != "supervisor-token" {
		t.Fatalf("unexpected HA token: %s", cfg.HAToken)
	}
}

func TestLoadParsesCollectionDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.TargetCustID != 5005808 {
		t.Fatalf("unexpected target cust id: %d", cfg.TargetCustID)
	}
	if len(cfg.TargetDays) != 5 {
		t.Fatalf("unexpected target days length: %d", len(cfg.TargetDays))
	}
	if cfg.CheckInterval != 20*time.Second {
		t.Fatalf("unexpected default check interval: %s", cfg.CheckInterval)
	}
	if cfg.SharedDataDir != "/share/garbage_eta" {
		t.Fatalf("unexpected shared data dir: %s", cfg.SharedDataDir)
	}
	if cfg.StateFile != filepath.Join("/share/garbage_eta", "state.json") || cfg.DatabaseFile != filepath.Join("/share/garbage_eta", "history.db") {
		t.Fatalf("unexpected shared file defaults: state=%s db=%s", cfg.StateFile, cfg.DatabaseFile)
	}
	if cfg.CollectorLogFile != filepath.Join("/share/garbage_eta", "logs", "collector.log") || cfg.ExportsDir != filepath.Join("/share/garbage_eta", "exports") {
		t.Fatalf("unexpected export defaults: log=%s exports=%s", cfg.CollectorLogFile, cfg.ExportsDir)
	}
	if cfg.CollectionStart != "19:00" || cfg.CollectionEnd != "21:30" {
		t.Fatalf("unexpected collection window: %s-%s", cfg.CollectionStart, cfg.CollectionEnd)
	}
	if cfg.HistoryWeeks != 8 {
		t.Fatalf("unexpected history weeks: %d", cfg.HistoryWeeks)
	}
	if cfg.ProgressWindowMeters != 150 || cfg.LateralOffsetLimitMeters != 80 {
		t.Fatalf("unexpected projection defaults: progress=%v lateral=%v", cfg.ProgressWindowMeters, cfg.LateralOffsetLimitMeters)
	}
	if cfg.BacktrackToleranceMeters != 30 || cfg.AmbiguousSegmentEpsilonMeters != 15 {
		t.Fatalf("unexpected ambiguity defaults: backtrack=%v epsilon=%v", cfg.BacktrackToleranceMeters, cfg.AmbiguousSegmentEpsilonMeters)
	}
	if cfg.SendTestMessageOnStart {
		t.Fatal("expected startup test message to be disabled by default")
	}
}

func TestLoadFallsBackToLegacyReminderMinutes(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("ALERT_OFFSETS", "")
	t.Setenv("REMINDER_MINUTES", "10,3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if len(cfg.AlertOffsets) != 2 || cfg.AlertOffsets[0] != 10 || cfg.AlertOffsets[1] != 3 {
		t.Fatalf("unexpected alert offsets: %+v", cfg.AlertOffsets)
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
		"TARGET_DAYS":        "MON,TUE,THU,FRI,SAT",
		"ALERT_OFFSETS":      "10,3",
		"HA_BASE_URL":        "http://homeassistant.local:8123",
		"HA_TOKEN":           "token",
		"HA_NOTIFY_MODE":     "webhook",
		"HA_TTS_TARGET":      "garbage_truck",
	}

	for key, value := range env {
		t.Setenv(key, value)
	}

	t.Setenv("SUPERVISOR_TOKEN", "")

	t.Setenv("CHECK_INTERVAL", "")
	t.Setenv("SEND_TEST_MESSAGE_ON_START", "")
	t.Setenv("EUPFIN_BASE_URL", "")
	t.Setenv("SHARED_DATA_DIR", "")
	t.Setenv("STATE_FILE", "")
	t.Setenv("DATABASE_FILE", "")
	t.Setenv("COLLECTOR_LOG_FILE", "")
	t.Setenv("EXPORTS_DIR", "")
	t.Setenv("COLLECTION_START", "")
	t.Setenv("COLLECTION_END", "")
	t.Setenv("HISTORY_WEEKS", "")
	t.Setenv("ARRIVAL_RADIUS_METERS", "")
	t.Setenv("MATCH_RADIUS_METERS", "")
	t.Setenv("MIN_HISTORY_RUNS", "")
	t.Setenv("PROGRESS_WINDOW_METERS", "")
	t.Setenv("LATERAL_OFFSET_LIMIT_METERS", "")
	t.Setenv("BACKTRACK_TOLERANCE_METERS", "")
	t.Setenv("AMBIGUOUS_SEGMENT_EPSILON_METERS", "")
	t.Setenv("TARGET_TIME", "")
	t.Setenv("REMINDER_MINUTES", "")
}

func setRequiredEnvWithoutHA(t *testing.T) {
	t.Helper()

	env := map[string]string{
		"TELEGRAM_BOT_TOKEN": "token",
		"TELEGRAM_CHAT_ID":   "chat-id",
		"TARGET_CUST_ID":     "5005808",
		"TARGET_ROUTE_ID":    "461",
		"TARGET_POINT_SEQ":   "27",
		"TARGET_POINT_NAME":  "有謙家園",
		"TARGET_DAYS":        "MON,TUE,THU,FRI,SAT",
		"ALERT_OFFSETS":      "10,3",
		"HA_NOTIFY_MODE":     "webhook",
		"HA_TTS_TARGET":      "garbage_truck",
	}

	for key, value := range env {
		t.Setenv(key, value)
	}

	t.Setenv("CHECK_INTERVAL", "")
	t.Setenv("SEND_TEST_MESSAGE_ON_START", "")
	t.Setenv("EUPFIN_BASE_URL", "")
	t.Setenv("SHARED_DATA_DIR", "")
	t.Setenv("STATE_FILE", "")
	t.Setenv("DATABASE_FILE", "")
	t.Setenv("COLLECTOR_LOG_FILE", "")
	t.Setenv("EXPORTS_DIR", "")
	t.Setenv("COLLECTION_START", "")
	t.Setenv("COLLECTION_END", "")
	t.Setenv("HISTORY_WEEKS", "")
	t.Setenv("ARRIVAL_RADIUS_METERS", "")
	t.Setenv("MATCH_RADIUS_METERS", "")
	t.Setenv("MIN_HISTORY_RUNS", "")
	t.Setenv("PROGRESS_WINDOW_METERS", "")
	t.Setenv("LATERAL_OFFSET_LIMIT_METERS", "")
	t.Setenv("BACKTRACK_TOLERANCE_METERS", "")
	t.Setenv("AMBIGUOUS_SEGMENT_EPSILON_METERS", "")
	t.Setenv("TARGET_TIME", "")
	t.Setenv("REMINDER_MINUTES", "")
}
