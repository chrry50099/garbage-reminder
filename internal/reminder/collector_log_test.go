package reminder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectorLoggerWritesJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.log")
	logger, err := NewCollectorLogger(path, 1024)
	if err != nil {
		t.Fatalf("NewCollectorLogger() returned error: %v", err)
	}

	if err := logger.LogSample(collectorSampleLog{
		ServiceDate:  "2026-04-13",
		CollectedAt:  isoTime(time.Date(2026, 4, 13, 19, 0, 0, 0, time.UTC)),
		GPSAvailable: true,
		TruckLat:     24.7,
		TruckLng:     121.0,
		RunStatus:    "collecting",
	}); err != nil {
		t.Fatalf("LogSample() returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if !strings.Contains(string(content), `"service_date":"2026-04-13"`) {
		t.Fatalf("unexpected log content: %s", string(content))
	}
}
