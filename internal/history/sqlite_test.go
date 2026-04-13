package history

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteStoreMigratesLegacySamplesTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "history.db")
	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() returned error: %v", err)
	}

	legacySchema := `
CREATE TABLE runs (
	run_id TEXT PRIMARY KEY,
	service_date TEXT NOT NULL,
	weekday INTEGER NOT NULL,
	route_id INTEGER NOT NULL,
	point_id INTEGER NOT NULL,
	started_at TEXT NOT NULL,
	last_collected_at TEXT,
	arrival_at TEXT,
	status TEXT NOT NULL,
	arrival_source TEXT,
	target_lat REAL NOT NULL,
	target_lng REAL NOT NULL
);

CREATE TABLE samples (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id TEXT NOT NULL,
	service_date TEXT NOT NULL,
	weekday INTEGER NOT NULL,
	route_id INTEGER NOT NULL,
	point_id INTEGER NOT NULL,
	truck_lat REAL NOT NULL,
	truck_lng REAL NOT NULL,
	gps_available INTEGER NOT NULL,
	api_estimated_minutes INTEGER,
	api_estimated_text TEXT,
	api_waiting_time INTEGER,
	collected_at TEXT NOT NULL,
	FOREIGN KEY(run_id) REFERENCES runs(run_id)
);
`
	if _, err := legacyDB.Exec(legacySchema); err != nil {
		t.Fatalf("Exec() returned error: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() returned error: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	progress := 120.0
	segment := 3
	lateral := 15.0
	if _, err := store.EnsureRun("2026-04-13", time.Monday, 461, 27, 24.7, 121.0, now); err != nil {
		t.Fatalf("EnsureRun() returned error: %v", err)
	}
	if err := store.InsertSample(Sample{
		RunID:               "2026-04-13",
		ServiceDate:         "2026-04-13",
		Weekday:             time.Monday,
		RouteID:             461,
		PointID:             27,
		TruckLat:            24.7,
		TruckLng:            121.0,
		GPSAvailable:        true,
		ProgressMeters:      &progress,
		SegmentIndex:        &segment,
		LateralOffsetMeters: &lateral,
		CollectedAt:         now,
	}); err != nil {
		t.Fatalf("InsertSample() returned error: %v", err)
	}

	recent, err := store.ListRecentRunSamples("2026-04-13", 2)
	if err != nil {
		t.Fatalf("ListRecentRunSamples() returned error: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("unexpected recent sample count: %d", len(recent))
	}
	if recent[0].ProgressMeters == nil || *recent[0].ProgressMeters != progress {
		t.Fatalf("unexpected progress from migrated schema: %+v", recent[0].ProgressMeters)
	}
}

func TestSQLiteStoreListsHistoricalSamplesWithProjectionFields(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore() returned error: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	progress := 260.0
	segment := 7
	lateral := 12.0
	if _, err := store.EnsureRun("2026-04-06", time.Monday, 461, 27, 24.7, 121.0, now.AddDate(0, 0, -7)); err != nil {
		t.Fatalf("EnsureRun() returned error: %v", err)
	}
	if err := store.InsertSample(Sample{
		RunID:               "2026-04-06",
		ServiceDate:         "2026-04-06",
		Weekday:             time.Monday,
		RouteID:             461,
		PointID:             27,
		TruckLat:            24.7,
		TruckLng:            121.0,
		GPSAvailable:        true,
		ProgressMeters:      &progress,
		SegmentIndex:        &segment,
		LateralOffsetMeters: &lateral,
		CollectedAt:         now.AddDate(0, 0, -7),
	}); err != nil {
		t.Fatalf("InsertSample() returned error: %v", err)
	}
	if err := store.MarkRunCompleted("2026-04-06", now.AddDate(0, 0, -7).Add(12*time.Minute), "gps_radius"); err != nil {
		t.Fatalf("MarkRunCompleted() returned error: %v", err)
	}

	samples, runCount, err := store.ListHistoricalSamples(time.Monday, 461, 27, now.AddDate(0, 0, -56))
	if err != nil {
		t.Fatalf("ListHistoricalSamples() returned error: %v", err)
	}
	if runCount != 1 || len(samples) != 1 {
		t.Fatalf("unexpected results: runs=%d samples=%d", runCount, len(samples))
	}
	if samples[0].ProgressMeters == nil || *samples[0].ProgressMeters != progress {
		t.Fatalf("unexpected historical progress: %+v", samples[0].ProgressMeters)
	}
	if samples[0].SegmentIndex == nil || *samples[0].SegmentIndex != segment {
		t.Fatalf("unexpected historical segment: %+v", samples[0].SegmentIndex)
	}
	if samples[0].LateralOffsetMeters == nil || *samples[0].LateralOffsetMeters != lateral {
		t.Fatalf("unexpected historical lateral offset: %+v", samples[0].LateralOffsetMeters)
	}
}

func TestSQLiteStoreListsServiceDatesAndSamplesByDay(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore() returned error: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	if _, err := store.EnsureRun("2026-04-13", time.Monday, 461, 27, 24.7, 121.0, now); err != nil {
		t.Fatalf("EnsureRun() returned error: %v", err)
	}
	if _, err := store.EnsureRun("2026-04-12", time.Sunday, 461, 27, 24.7, 121.0, now.AddDate(0, 0, -1)); err != nil {
		t.Fatalf("EnsureRun() returned error: %v", err)
	}
	if err := store.InsertSample(Sample{
		RunID:        "2026-04-13",
		ServiceDate:  "2026-04-13",
		Weekday:      time.Monday,
		RouteID:      461,
		PointID:      27,
		TruckLat:     24.7,
		TruckLng:     121.0,
		GPSAvailable: true,
		CollectedAt:  now,
	}); err != nil {
		t.Fatalf("InsertSample() returned error: %v", err)
	}

	dates, err := store.ListServiceDates(10)
	if err != nil {
		t.Fatalf("ListServiceDates() returned error: %v", err)
	}
	if len(dates) != 2 || dates[0] != "2026-04-13" {
		t.Fatalf("unexpected service dates: %+v", dates)
	}

	samples, err := store.ListSamplesByServiceDate("2026-04-13")
	if err != nil {
		t.Fatalf("ListSamplesByServiceDate() returned error: %v", err)
	}
	if len(samples) != 1 || !samples[0].GPSAvailable {
		t.Fatalf("unexpected samples: %+v", samples)
	}
}
