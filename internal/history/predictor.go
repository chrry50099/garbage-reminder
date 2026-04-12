package history

import (
	"math"
	"sort"
	"time"
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
	ProgressMeters      *float64
	SegmentIndex        *int
	LateralOffsetMeters *float64
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
	ProgressWindowMeters     float64
	LateralOffsetLimitMeters float64
	MinHistoryRuns           int
	TargetProgressMeters     *float64
}

type rankedSample struct {
	remainingMinutes int
	weight           float64
	progressDelta    float64
	lateralOffset    float64
}

func PredictFromHistory(observation Observation, samples []HistoricalSample, runCount int, cfg PredictorConfig) *Prediction {
	if !observation.GPSAvailable || observation.ProgressMeters == nil || observation.LateralOffsetMeters == nil {
		return nil
	}
	if runCount < cfg.MinHistoryRuns || cfg.TargetProgressMeters == nil {
		return nil
	}
	if *observation.LateralOffsetMeters > cfg.LateralOffsetLimitMeters {
		return nil
	}
	if *observation.ProgressMeters >= *cfg.TargetProgressMeters {
		return nil
	}

	matches := make([]rankedSample, 0, len(samples))
	for _, sample := range samples {
		if sample.ProgressMeters == nil || sample.LateralOffsetMeters == nil {
			continue
		}
		if *sample.LateralOffsetMeters > cfg.LateralOffsetLimitMeters {
			continue
		}
		if *sample.ProgressMeters >= *cfg.TargetProgressMeters {
			continue
		}

		progressDelta := math.Abs(*observation.ProgressMeters - *sample.ProgressMeters)
		if progressDelta > cfg.ProgressWindowMeters {
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

		progressWeight := 1 / (1 + progressDelta/20)
		offsetWeight := 1 / (1 + *sample.LateralOffsetMeters/20)
		recencyWeight := 1 / (1 + daysAgo/7)
		matches = append(matches, rankedSample{
			remainingMinutes: remaining,
			weight:           progressWeight * offsetWeight * recencyWeight,
			progressDelta:    progressDelta,
			lateralOffset:    *sample.LateralOffsetMeters,
		})
	}

	if len(matches) == 0 {
		return nil
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].progressDelta == matches[j].progressDelta {
			return matches[i].lateralOffset < matches[j].lateralOffset
		}
		return matches[i].progressDelta < matches[j].progressDelta
	})
	if len(matches) > 5 {
		matches = matches[:5]
	}

	remaining := weightedMedian(matches)
	confidence := "medium"
	if len(matches) >= 4 {
		confidence = "high"
	} else if len(matches) == 1 {
		confidence = "low"
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
