package history

import (
	"math"
	"sort"
	"time"

	"telegram-garbage-reminder/internal/geo"
)

type Observation struct {
	CollectedAt         time.Time
	Weekday             time.Weekday
	RouteID             int
	PointID             int
	TruckLat            float64
	TruckLng            float64
	GPSAvailable        bool
	APIEstimatedMinutes *int
	APIEstimatedText    string
	APIWaitingTime      *int
}

type Prediction struct {
	PredictedArrivalAt time.Time `json:"predicted_arrival_at"`
	RemainingMinutes   int       `json:"remaining_minutes"`
	Source             string    `json:"source"`
	Confidence         string    `json:"confidence"`
	MatchedSamples     int       `json:"matched_samples"`
	HistoricalRuns     int       `json:"historical_runs"`
}

type PredictorConfig struct {
	MatchRadiusMeters float64
	MinHistoryRuns    int
}

type rankedSample struct {
	remainingMinutes int
	weight           float64
	distanceMeters   float64
}

func PredictFromHistory(observation Observation, samples []HistoricalSample, runCount int, cfg PredictorConfig) *Prediction {
	if !observation.GPSAvailable || runCount < cfg.MinHistoryRuns {
		return nil
	}

	matches := make([]rankedSample, 0, len(samples))
	for _, sample := range samples {
		distanceMeters := geo.CalculateDistance(observation.TruckLat, observation.TruckLng, sample.TruckLat, sample.TruckLng)
		if distanceMeters > cfg.MatchRadiusMeters {
			continue
		}

		remaining := int(math.Ceil(sample.ArrivalAt.Sub(sample.CollectedAt).Minutes()))
		if remaining <= 0 {
			continue
		}

		daysAgo := observation.CollectedAt.Sub(sample.CollectedAt).Hours() / 24
		if daysAgo < 0 {
			daysAgo = 0
		}

		distanceWeight := 1 / (1 + distanceMeters/25)
		recencyWeight := 1 / (1 + daysAgo/7)
		matches = append(matches, rankedSample{
			remainingMinutes: remaining,
			weight:           distanceWeight * recencyWeight,
			distanceMeters:   distanceMeters,
		})
	}

	if len(matches) == 0 {
		return nil
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distanceMeters < matches[j].distanceMeters
	})
	if len(matches) > 5 {
		matches = matches[:5]
	}

	remaining := weightedMedian(matches)
	confidence := "medium"
	if len(matches) >= 4 {
		confidence = "high"
	}

	return &Prediction{
		PredictedArrivalAt: observation.CollectedAt.Add(time.Duration(remaining) * time.Minute),
		RemainingMinutes:   remaining,
		Source:             "historical_model",
		Confidence:         confidence,
		MatchedSamples:     len(matches),
		HistoricalRuns:     runCount,
	}
}

func PredictFromFallback(observation Observation) *Prediction {
	if observation.APIEstimatedMinutes != nil {
		return &Prediction{
			PredictedArrivalAt: observation.CollectedAt.Add(time.Duration(*observation.APIEstimatedMinutes) * time.Minute),
			RemainingMinutes:   *observation.APIEstimatedMinutes,
			Source:             "api_estimated_time",
			Confidence:         "medium",
		}
	}
	if observation.APIWaitingTime != nil && *observation.APIWaitingTime >= 0 {
		return &Prediction{
			PredictedArrivalAt: observation.CollectedAt.Add(time.Duration(*observation.APIWaitingTime) * time.Minute),
			RemainingMinutes:   *observation.APIWaitingTime,
			Source:             "api_waiting_time",
			Confidence:         "low",
		}
	}
	return nil
}

func weightedMedian(samples []rankedSample) int {
	sorted := append([]rankedSample(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].remainingMinutes < sorted[j].remainingMinutes
	})

	var total float64
	for _, sample := range sorted {
		total += sample.weight
	}

	var cumulative float64
	for _, sample := range sorted {
		cumulative += sample.weight
		if cumulative >= total/2 {
			return sample.remainingMinutes
		}
	}

	return sorted[len(sorted)-1].remainingMinutes
}
