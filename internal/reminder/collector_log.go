package reminder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const defaultCollectorLogMaxBytes int64 = 5 * 1024 * 1024

type CollectorLogger struct {
	path     string
	maxBytes int64
	mu       sync.Mutex
}

type collectorSampleLog struct {
	Kind                string   `json:"kind"`
	ServiceDate         string   `json:"service_date"`
	CollectedAt         string   `json:"collected_at"`
	GPSAvailable        bool     `json:"gps_available"`
	TruckLat            float64  `json:"truck_lat,omitempty"`
	TruckLng            float64  `json:"truck_lng,omitempty"`
	ProgressMeters      *float64 `json:"progress_meters,omitempty"`
	SegmentIndex        *int     `json:"segment_index,omitempty"`
	LateralOffsetMeters *float64 `json:"lateral_offset_meters,omitempty"`
	APIEstimatedMinutes *int     `json:"api_estimated_minutes,omitempty"`
	APIWaitingTime      *int     `json:"api_waiting_time,omitempty"`
	PredictionSource    string   `json:"prediction_source,omitempty"`
	RunStatus           string   `json:"run_status,omitempty"`
}

type collectorErrorLog struct {
	Kind        string `json:"kind"`
	ServiceDate string `json:"service_date"`
	CollectedAt string `json:"collected_at"`
	Step        string `json:"step"`
	Error       string `json:"error"`
}

func NewCollectorLogger(path string, maxBytes int64) (*CollectorLogger, error) {
	if maxBytes <= 0 {
		maxBytes = defaultCollectorLogMaxBytes
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create collector log directory: %w", err)
	}
	logger := &CollectorLogger{path: path, maxBytes: maxBytes}
	if err := logger.ensureWritable(); err != nil {
		return nil, err
	}
	return logger, nil
}

func (l *CollectorLogger) ensureWritable() error {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open collector log: %w", err)
	}
	return file.Close()
}

func (l *CollectorLogger) LogSample(entry collectorSampleLog) error {
	entry.Kind = "sample"
	return l.write(entry)
}

func (l *CollectorLogger) LogError(entry collectorErrorLog) error {
	entry.Kind = "error"
	return l.write(entry)
}

func (l *CollectorLogger) write(entry any) error {
	if l == nil {
		return nil
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal collector log entry: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotateIfNeeded(int64(len(payload) + 1)); err != nil {
		return err
	}

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open collector log for append: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("append collector log: %w", err)
	}
	return nil
}

func (l *CollectorLogger) rotateIfNeeded(incomingBytes int64) error {
	info, err := os.Stat(l.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat collector log: %w", err)
	}
	if info.Size()+incomingBytes <= l.maxBytes {
		return nil
	}

	backupPath := l.path + ".1"
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove previous collector log backup: %w", err)
	}
	if err := os.Rename(l.path, backupPath); err != nil {
		return fmt.Errorf("rotate collector log: %w", err)
	}
	return nil
}

func isoTime(value time.Time) string {
	return value.Format(time.RFC3339)
}
