package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"telegram-garbage-reminder/internal/config"
	"telegram-garbage-reminder/internal/eupfin"
	"telegram-garbage-reminder/internal/history"
	"telegram-garbage-reminder/internal/notifier"
	"telegram-garbage-reminder/internal/reminder"
	"telegram-garbage-reminder/internal/state"
)

func main() {
	log.Println("Starting garbage Telegram reminder service...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration load failed: %v", err)
	}
	if err := ensurePaths(cfg); err != nil {
		log.Fatalf("Failed to prepare shared data paths: %v", err)
	}
	if err := migrateLegacyData(cfg); err != nil {
		log.Fatalf("Failed to migrate legacy data paths: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localState, err := state.NewLocalStore(cfg.StateFile)
	if err != nil {
		log.Fatalf("Failed to create local state store: %v", err)
	}

	historyStore, err := history.NewSQLiteStore(cfg.DatabaseFile)
	if err != nil {
		log.Fatalf("Failed to create sqlite history store: %v", err)
	}
	defer historyStore.Close()

	collectorLog, err := reminder.NewCollectorLogger(cfg.CollectorLogFile, 5*1024*1024)
	if err != nil {
		log.Fatalf("Failed to create collector log: %v", err)
	}

	eupfinClient := eupfin.NewClient(cfg.EupfinBaseURL)
	telegramNotifier := notifier.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID)
	haNotifier := notifier.NewHomeAssistant(cfg.HABaseURL, cfg.HAToken, cfg.HANotifyMode, cfg.HATTSTarget)
	haNotifier.SetStateStore(localState)
	haNotifier.SetGoogleCloud(notifier.NewGoogleCloudTTS(cfg.GoogleCloudTTSAPIKey, cfg.GoogleCloudMediaDir))
	alertNotifier := notifier.NewMultiSender(telegramNotifier, haNotifier)
	service := reminder.NewService(cfg, eupfinClient, alertNotifier, telegramNotifier, localState, historyStore, collectorLog)

	statusServer := startStatusServer(cfg.HTTPPort, service, haNotifier)
	defer shutdownStatusServer(statusServer)

	if err := service.Initialize(ctx); err != nil {
		log.Printf("Startup validation failed, service will retry on scheduled checks: %v", err)
	}

	if cfg.SendTestMessageOnStart {
		if err := service.SendStartupTestMessage(ctx); err != nil {
			log.Printf("Startup test message failed: %v", err)
		}
	}

	if err := service.CheckOnce(ctx); err != nil {
		log.Printf("Initial reminder check failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		service.Start(ctx)
	}()

	waitForShutdown(cancel)
	shutdownStatusServer(statusServer)
	<-done
}

func waitForShutdown(cancel context.CancelFunc) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	<-quit
	cancel()
	log.Println("Reminder service stopped")
}

func startStatusServer(port string, service *reminder.Service, haControl *notifier.HomeAssistant) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", reminder.NewDashboardHandler())
	mux.Handle("/status", reminder.NewStatusHandler(service))
	mux.Handle("/api/history/dates", reminder.NewHistoryDatesHandler(service))
	mux.Handle("/api/history/today", reminder.NewHistoryTodayHandler(service))
	mux.Handle("/api/history/day", reminder.NewHistoryDayHandler(service))
	mux.Handle("/api/history/day.json", reminder.NewHistoryDayJSONHandler(service))
	mux.Handle("/api/history/day.csv", reminder.NewHistoryDayCSVHandler(service))
	mux.Handle("/api/broadcast/options", reminder.NewBroadcastOptionsHandler(haControl))
	mux.Handle("/api/broadcast/test", reminder.NewBroadcastTestHandler(haControl))
	mux.Handle("/api/broadcast/auto", reminder.NewAutoBroadcastSettingsHandler(haControl))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("Status server listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Status server failed: %v", err)
		}
	}()

	return server
}

func shutdownStatusServer(server *http.Server) {
	if server == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		log.Printf("Status server shutdown failed: %v", err)
	}
}

func ensurePaths(cfg *config.Config) error {
	for _, dir := range []string{
		cfg.SharedDataDir,
		filepath.Dir(cfg.StateFile),
		filepath.Dir(cfg.DatabaseFile),
		filepath.Dir(cfg.CollectorLogFile),
		cfg.ExportsDir,
		cfg.GoogleCloudMediaDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyData(cfg *config.Config) error {
	for _, item := range []struct {
		label  string
		legacy string
		target string
	}{
		{label: "state", legacy: "/data/state.json", target: cfg.StateFile},
		{label: "history", legacy: "/data/history.db", target: cfg.DatabaseFile},
	} {
		migrated, err := migrateFileIfMissing(item.legacy, item.target)
		if err != nil {
			return fmt.Errorf("%s file migration failed: %w", item.label, err)
		}
		if migrated {
			log.Printf("Migrated legacy %s file from %s to %s", item.label, item.legacy, item.target)
		}
	}
	return nil
}

func migrateFileIfMissing(legacyPath, targetPath string) (bool, error) {
	if samePath(legacyPath, targetPath) {
		return false, nil
	}

	if _, err := os.Stat(targetPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat target: %w", err)
	}

	legacyInfo, err := os.Stat(legacyPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat legacy: %w", err)
	}
	if legacyInfo.IsDir() {
		return false, fmt.Errorf("legacy path is a directory: %s", legacyPath)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return false, fmt.Errorf("ensure target directory: %w", err)
	}

	source, err := os.Open(legacyPath)
	if err != nil {
		return false, fmt.Errorf("open legacy file: %w", err)
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return false, fmt.Errorf("create target file: %w", err)
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return false, fmt.Errorf("copy file contents: %w", err)
	}
	return true, nil
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
