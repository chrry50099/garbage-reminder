package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CachedTarget struct {
	CustID        int       `json:"cust_id"`
	TeamID        int       `json:"team_id"`
	RouteID       int       `json:"route_id"`
	RouteName     string    `json:"route_name"`
	PointID       int       `json:"point_id"`
	PointSeq      int       `json:"point_seq"`
	PointName     string    `json:"point_name"`
	ScheduledTime string    `json:"scheduled_time"`
	GISX          float64   `json:"gisx"`
	GISY          float64   `json:"gisy"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeliveryRecord struct {
	ScheduledDate         string    `json:"scheduled_date"`
	ReminderOffsetMinutes int       `json:"reminder_offset_minutes"`
	DeliveryStatus        string    `json:"delivery_status"`
	GPSMode               string    `json:"gps_mode"`
	PredictionSource      string    `json:"prediction_source,omitempty"`
	Confidence            string    `json:"confidence,omitempty"`
	SentAt                time.Time `json:"sent_at"`
}

type AppState struct {
	CachedTarget *CachedTarget             `json:"cached_target,omitempty"`
	Deliveries   map[string]DeliveryRecord `json:"deliveries"`
}

type LocalStore struct {
	path  string
	mu    sync.Mutex
	state AppState
}

func NewLocalStore(path string) (*LocalStore, error) {
	store := &LocalStore{
		path: path,
		state: AppState{
			Deliveries: make(map[string]DeliveryRecord),
		},
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *LocalStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return s.persistLocked()
	} else if err != nil {
		return fmt.Errorf("stat state file: %w", err)
	}

	content, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read state file: %w", err)
	}

	if len(content) == 0 {
		return s.persistLocked()
	}

	if err := json.Unmarshal(content, &s.state); err != nil {
		return fmt.Errorf("decode state file: %w", err)
	}

	if s.state.Deliveries == nil {
		s.state.Deliveries = make(map[string]DeliveryRecord)
	}

	return nil
}

func (s *LocalStore) SaveCachedTarget(target CachedTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target.UpdatedAt = time.Now()
	s.state.CachedTarget = &target

	return s.persistLocked()
}

func (s *LocalStore) GetCachedTarget() *CachedTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.CachedTarget == nil {
		return nil
	}

	copy := *s.state.CachedTarget
	return &copy
}

func (s *LocalStore) HasDelivery(deliveryKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.state.Deliveries[deliveryKey]
	return ok
}

func (s *LocalStore) RecordDelivery(deliveryKey string, record DeliveryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Deliveries[deliveryKey] = record
	s.pruneDeliveriesLocked(30)

	return s.persistLocked()
}

func (s *LocalStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	content, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file: %w", err)
	}

	if err := os.WriteFile(s.path, content, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

func (s *LocalStore) pruneDeliveriesLocked(retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02")
	for key, record := range s.state.Deliveries {
		if record.ScheduledDate < cutoff {
			delete(s.state.Deliveries, key)
		}
	}
}
