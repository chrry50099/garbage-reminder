package history

import (
	"testing"
	"time"
)

func TestPredictFromHistoryUsesNearestSamples(t *testing.T) {
	now := time.Date(2026, 4, 13, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	observation := Observation{
		CollectedAt:  now,
		Weekday:      time.Monday,
		RouteID:      461,
		PointID:      27,
		TruckLat:     24.7400,
		TruckLng:     121.0100,
		GPSAvailable: true,
	}

	samples := []HistoricalSample{
		{
			RunID:       "2026-04-06",
			ServiceDate: "2026-04-06",
			TruckLat:    24.7401,
			TruckLng:    121.0101,
			CollectedAt: now.AddDate(0, 0, -7),
			ArrivalAt:   now.AddDate(0, 0, -7).Add(8 * time.Minute),
		},
		{
			RunID:       "2026-03-30",
			ServiceDate: "2026-03-30",
			TruckLat:    24.7402,
			TruckLng:    121.0102,
			CollectedAt: now.AddDate(0, 0, -14),
			ArrivalAt:   now.AddDate(0, 0, -14).Add(10 * time.Minute),
		},
		{
			RunID:       "2026-03-23",
			ServiceDate: "2026-03-23",
			TruckLat:    24.7403,
			TruckLng:    121.0103,
			CollectedAt: now.AddDate(0, 0, -21),
			ArrivalAt:   now.AddDate(0, 0, -21).Add(9 * time.Minute),
		},
	}

	prediction := PredictFromHistory(observation, samples, 3, PredictorConfig{
		MatchRadiusMeters: 250,
		MinHistoryRuns:    3,
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
