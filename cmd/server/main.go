package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegram-garbage-reminder/internal/config"
	"telegram-garbage-reminder/internal/eupfin"
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

	eupfinClient := eupfin.NewClient(cfg.EupfinBaseURL)
	telegramNotifier := notifier.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID)
	service := reminder.NewService(cfg, eupfinClient, telegramNotifier, localState)

	if err := service.Initialize(ctx); err != nil {
		log.Fatalf("Startup validation failed: %v", err)
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
