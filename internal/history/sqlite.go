package history

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

type Run struct {
	RunID           string
	ServiceDate     string
	Weekday         time.Weekday
	RouteID         int
	PointID         int
	StartedAt       time.Time
	LastCollectedAt *time.Time
	ArrivalAt       *time.Time
	Status          string
	ArrivalSource   string
	TargetLat       float64
	TargetLng       float64
}

type Sample struct {
	RunID               string
	ServiceDate         string
	Weekday             time.Weekday
	RouteID             int
	PointID             int
	TruckLat            float64
	TruckLng            float64
	GPSAvailable        bool
	APIEstimatedMinutes *int
	APIEstimatedText    string
	APIWaitingTime      *int
	CollectedAt         time.Time
}

type HistoricalSample struct {
	RunID       string
	ServiceDate string
	TruckLat    float64
	TruckLng    float64
	CollectedAt time.Time
	ArrivalAt   time.Time
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) EnsureRun(serviceDate string, weekday time.Weekday, routeID, pointID int, targetLat, targetLng float64, startedAt time.Time) (*Run, error) {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO runs (
			run_id, service_date, weekday, route_id, point_id, started_at, status, target_lat, target_lng
		) VALUES (?, ?, ?, ?, ?, ?, 'collecting', ?, ?)
	`, serviceDate, serviceDate, int(weekday), routeID, pointID, startedAt.Format(time.RFC3339), targetLat, targetLng)
	if err != nil {
		return nil, fmt.Errorf("ensure run: %w", err)
	}

	return s.GetRun(serviceDate)
}

func (s *SQLiteStore) GetRun(runID string) (*Run, error) {
	row := s.db.QueryRow(`
		SELECT run_id, service_date, weekday, route_id, point_id, started_at, last_collected_at,
		       arrival_at, status, IFNULL(arrival_source, ''), target_lat, target_lng
		FROM runs
		WHERE run_id = ?
	`, runID)

	var run Run
	var weekday int
	var startedAt string
	var lastCollectedAt sql.NullString
	var arrivalAt sql.NullString
	if err := row.Scan(
		&run.RunID,
		&run.ServiceDate,
		&weekday,
		&run.RouteID,
		&run.PointID,
		&startedAt,
		&lastCollectedAt,
		&arrivalAt,
		&run.Status,
		&run.ArrivalSource,
		&run.TargetLat,
		&run.TargetLng,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get run: %w", err)
	}

	parsedStartedAt, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse started_at: %w", err)
	}
	run.StartedAt = parsedStartedAt
	run.Weekday = time.Weekday(weekday)

	if lastCollectedAt.Valid {
		parsed, err := time.Parse(time.RFC3339, lastCollectedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse last_collected_at: %w", err)
		}
		run.LastCollectedAt = &parsed
	}

	if arrivalAt.Valid {
		parsed, err := time.Parse(time.RFC3339, arrivalAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse arrival_at: %w", err)
		}
		run.ArrivalAt = &parsed
	}

	return &run, nil
}

func (s *SQLiteStore) InsertSample(sample Sample) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin insert sample tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO samples (
			run_id, service_date, weekday, route_id, point_id, truck_lat, truck_lng, gps_available,
			api_estimated_minutes, api_estimated_text, api_waiting_time, collected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		sample.RunID,
		sample.ServiceDate,
		int(sample.Weekday),
		sample.RouteID,
		sample.PointID,
		sample.TruckLat,
		sample.TruckLng,
		boolToInt(sample.GPSAvailable),
		nullableInt(sample.APIEstimatedMinutes),
		sample.APIEstimatedText,
		nullableInt(sample.APIWaitingTime),
		sample.CollectedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert sample: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE runs
		SET last_collected_at = ?
		WHERE run_id = ?
	`, sample.CollectedAt.Format(time.RFC3339), sample.RunID)
	if err != nil {
		return fmt.Errorf("update run last_collected_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert sample tx: %w", err)
	}

	return nil
}

func (s *SQLiteStore) MarkRunCompleted(runID string, arrivalAt time.Time, arrivalSource string) error {
	_, err := s.db.Exec(`
		UPDATE runs
		SET status = 'completed', arrival_at = ?, arrival_source = ?
		WHERE run_id = ?
	`, arrivalAt.Format(time.RFC3339), arrivalSource, runID)
	if err != nil {
		return fmt.Errorf("mark run completed: %w", err)
	}
	return nil
}

func (s *SQLiteStore) PruneBefore(cutoff time.Time) error {
	cutoffDate := cutoff.Format("2006-01-02")
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin prune tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM samples WHERE service_date < ?`, cutoffDate); err != nil {
		return fmt.Errorf("delete old samples: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM runs WHERE service_date < ?`, cutoffDate); err != nil {
		return fmt.Errorf("delete old runs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit prune tx: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListHistoricalSamples(weekday time.Weekday, routeID, pointID int, since time.Time) ([]HistoricalSample, int, error) {
	rows, err := s.db.Query(`
		SELECT s.run_id, r.service_date, s.truck_lat, s.truck_lng, s.collected_at, r.arrival_at
		FROM samples s
		JOIN runs r ON r.run_id = s.run_id
		WHERE s.weekday = ?
		  AND s.route_id = ?
		  AND s.point_id = ?
		  AND s.gps_available = 1
		  AND r.status = 'completed'
		  AND r.arrival_at IS NOT NULL
		  AND r.service_date >= ?
		ORDER BY r.service_date DESC, s.collected_at ASC
	`, int(weekday), routeID, pointID, since.Format("2006-01-02"))
	if err != nil {
		return nil, 0, fmt.Errorf("list historical samples: %w", err)
	}
	defer rows.Close()

	runSeen := make(map[string]struct{})
	samples := make([]HistoricalSample, 0)
	for rows.Next() {
		var sample HistoricalSample
		var collectedAt string
		var arrivalAt string
		if err := rows.Scan(
			&sample.RunID,
			&sample.ServiceDate,
			&sample.TruckLat,
			&sample.TruckLng,
			&collectedAt,
			&arrivalAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan historical sample: %w", err)
		}

		sample.CollectedAt, err = time.Parse(time.RFC3339, collectedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse sample collected_at: %w", err)
		}
		sample.ArrivalAt, err = time.Parse(time.RFC3339, arrivalAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse sample arrival_at: %w", err)
		}

		runSeen[sample.RunID] = struct{}{}
		samples = append(samples, sample)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate historical samples: %w", err)
	}

	return samples, len(runSeen), nil
}

func (s *SQLiteStore) ensureSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS runs (
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

CREATE TABLE IF NOT EXISTS samples (
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

CREATE INDEX IF NOT EXISTS idx_samples_lookup ON samples (weekday, route_id, point_id, service_date);
`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("ensure sqlite schema: %w", err)
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
