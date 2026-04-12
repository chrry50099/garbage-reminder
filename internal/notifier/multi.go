package notifier

import (
	"context"
	"errors"
	"strings"
)

type Sender interface {
	SendMessage(ctx context.Context, text string) error
}

type MultiSender struct {
	senders []Sender
}

func NewMultiSender(senders ...Sender) *MultiSender {
	filtered := make([]Sender, 0, len(senders))
	for _, sender := range senders {
		if sender != nil {
			filtered = append(filtered, sender)
		}
	}
	return &MultiSender{senders: filtered}
}

func (m *MultiSender) SendMessage(ctx context.Context, text string) error {
	var errs []error
	for _, sender := range m.senders {
		if err := sender.SendMessage(ctx, text); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}

	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	return errors.New(strings.Join(messages, "; "))
}
