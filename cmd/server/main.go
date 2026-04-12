package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	eupfinClient := eupfin.NewClient(cfg.EupfinBaseURL)
	telegramNotifier := notifier.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID)
	haNotifier := notifier.NewHomeAssistant(cfg.HABaseURL, cfg.HAToken, cfg.HANotifyMode, cfg.HATTSTarget)
	alertNotifier := notifier.NewMultiSender(telegramNotifier, haNotifier)
	service := reminder.NewService(cfg, eupfinClient, alertNotifier, telegramNotifier, localState, historyStore)

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
	mux.Handle("/api/broadcast/options", reminder.NewBroadcastOptionsHandler(haControl))
	mux.Handle("/api/broadcast/test", reminder.NewBroadcastTestHandler(haControl))

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
