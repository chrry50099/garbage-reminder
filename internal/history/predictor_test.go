package history

import (
	"testing"
	"time"
)

func TestPredictFromHistoryUsesProgressMatchedSamples(t *testing.T) {
	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	progress := 120.0
	lateral := 18.0
	targetProgress := 600.0
	observation := Observation{
		CollectedAt:         now,
		Weekday:             time.Monday,
		RouteID:             461,
		PointID:             27,
		GPSAvailable:        true,
		ProgressMeters:      &progress,
		LateralOffsetMeters: &lateral,
	}

	samples := []HistoricalSample{
		historicalProjectionSample("2026-04-06", now.AddDate(0, 0, -7), 118, 10, 8),
		historicalProjectionSample("2026-03-30", now.AddDate(0, 0, -14), 125, 16, 9),
		historicalProjectionSample("2026-03-23", now.AddDate(0, 0, -21), 132, 20, 10),
	}

	prediction := PredictFromHistory(observation, samples, 3, PredictorConfig{
		ProgressWindowMeters:     150,
		LateralOffsetLimitMeters: 80,
		MinHistoryRuns:           3,
		TargetProgressMeters:     &targetProgress,
	})
	if prediction == nil {
		t.Fatal("expected prediction")
	}
	if prediction.Source != "historical_model" {
		t.Fatalf("unexpected source: %s", prediction.Source)
	}
	if prediction.RemainingMinutes < 8 || prediction.RemainingMinutes > 10 {
		t.Fatalf("unexpected remaining minutes: %d", prediction.RemainingMinutes)
	}
}

func TestPredictFromHistoryIgnoresCrossingBranchSamples(t *testing.T) {
	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	progress := 260.0
	lateral := 8.0
	targetProgress := 900.0
	observation := Observation{
		CollectedAt:         now,
		GPSAvailable:        true,
		ProgressMeters:      &progress,
		LateralOffsetMeters: &lateral,
	}

	samples := []HistoricalSample{
		historicalProjectionSample("2026-04-06", now.AddDate(0, 0, -7), 255, 5, 12),
		historicalProjectionSample("2026-03-30", now.AddDate(0, 0, -14), 640, 4, 3),
		historicalProjectionSample("2026-03-23", now.AddDate(0, 0, -21), 270, 9, 11),
	}

	prediction := PredictFromHistory(observation, samples, 3, PredictorConfig{
		ProgressWindowMeters:     150,
		LateralOffsetLimitMeters: 80,
		MinHistoryRuns:           3,
		TargetProgressMeters:     &targetProgress,
	})
	if prediction == nil {
		t.Fatal("expected prediction")
	}
	if prediction.RemainingMinutes < 11 || prediction.RemainingMinutes > 12 {
		t.Fatalf("expected crossing samples to stay on first branch, got %d", prediction.RemainingMinutes)
	}
}

func TestPredictFromHistoryReturnsNilWhenProjectionMissingOrOffsetTooLarge(t *testing.T) {
	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	targetProgress := 600.0
	tooFar := 90.0
	samples := []HistoricalSample{
		historicalProjectionSample("2026-04-06", now.AddDate(0, 0, -7), 118, 10, 8),
		historicalProjectionSample("2026-03-30", now.AddDate(0, 0, -14), 125, 16, 9),
		historicalProjectionSample("2026-03-23", now.AddDate(0, 0, -21), 132, 20, 10),
	}

	if prediction := PredictFromHistory(Observation{
		CollectedAt:         now,
		GPSAvailable:        true,
		LateralOffsetMeters: &tooFar,
	}, samples, 3, PredictorConfig{
		ProgressWindowMeters:     150,
		LateralOffsetLimitMeters: 80,
		MinHistoryRuns:           3,
		TargetProgressMeters:     &targetProgress,
	}); prediction != nil {
		t.Fatal("expected nil prediction without current progress")
	}

	progress := 120.0
	if prediction := PredictFromHistory(Observation{
		CollectedAt:         now,
		GPSAvailable:        true,
		ProgressMeters:      &progress,
		LateralOffsetMeters: &tooFar,
	}, samples, 3, PredictorConfig{
		ProgressWindowMeters:     150,
		LateralOffsetLimitMeters: 80,
		MinHistoryRuns:           3,
		TargetProgressMeters:     &targetProgress,
	}); prediction != nil {
		t.Fatal("expected nil prediction for large lateral offset")
	}
}

func TestPredictFromFallbackUsesEstimatedMinutesFirst(t *testing.T) {
	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	estimated := 6
	waiting := 3
	prediction := PredictFromFallback(Observation{
		CollectedAt:         now,
		APIEstimatedMinutes: &estimated,
		APIWaitingTime:      &waiting,
	})
	if prediction == nil {
		t.Fatal("expected fallback prediction")
	}
	if prediction.Source != "api_estimated_time" {
		t.Fatalf("unexpected source: %s", prediction.Source)
	}
	if prediction.RemainingMinutes != 6 {
		t.Fatalf("unexpected remaining minutes: %d", prediction.RemainingMinutes)
	}
}

func historicalProjectionSample(runID string, collectedAt time.Time, progressValue, lateralValue float64, remainingMinutes int) HistoricalSample {
	progress := progressValue
	lateral := lateralValue
	return HistoricalSample{
		RunID:               runID,
		ServiceDate:         collectedAt.Format("2006-01-02"),
		CollectedAt:         collectedAt,
		ArrivalAt:           collectedAt.Add(time.Duration(remainingMinutes) * time.Minute),
		ProgressMeters:      &progress,
		LateralOffsetMeters: &lateral,
	}
}
