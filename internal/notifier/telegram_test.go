package notifier

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSendMessagePostsExpectedFields(t *testing.T) {
	var received url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		received, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	notifier := NewTelegram("test-token", "12345")
	notifier.httpClient = server.Client()
	notifier.apiBaseURL = server.URL

	if err := notifier.SendMessage(context.Background(), "hello"); err != nil {
		t.Fatalf("SendMessage() returned error: %v", err)
	}

	if received.Get("chat_id") != "12345" {
		t.Fatalf("unexpected chat_id: %s", received.Get("chat_id"))
	}
	if received.Get("text") != "hello" {
		t.Fatalf("unexpected text: %s", received.Get("text"))
	}
}

func TestSendMessageReturnsTelegramError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer server.Close()

	notifier := NewTelegram("test-token", "12345")
	notifier.httpClient = server.Client()
	notifier.apiBaseURL = server.URL

	err := notifier.SendMessage(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected SendMessage() to return an error")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("expected telegram description in error, got %v", err)
	}
}

func TestHomeAssistantWebhookPostsExpectedPayload(t *testing.T) {
	var body string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/webhook/garbage_alert" {
			t.Fatalf("unexpected webhook path: %s", r.URL.Path)
		}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		body = string(payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewHomeAssistant(server.URL, "token", "webhook", "garbage_alert")
	notifier.httpClient = server.Client()

	if err := notifier.SendMessage(context.Background(), "hello"); err != nil {
		t.Fatalf("SendMessage() returned error: %v", err)
	}

	if !strings.Contains(body, "\"message\":\"hello\"") {
		t.Fatalf("unexpected payload body: %s", body)
	}
}

func TestHomeAssistantServiceCallUsesDomainServicePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/script/homepod_broadcast" {
			t.Fatalf("unexpected service path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewHomeAssistant(server.URL, "token", "service_call", "script.homepod_broadcast")
	notifier.httpClient = server.Client()

	if err := notifier.SendMessage(context.Background(), "hello"); err != nil {
		t.Fatalf("SendMessage() returned error: %v", err)
	}
}
