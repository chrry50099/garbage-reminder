package reminder

import (
	"math"
	"sort"
	"strconv"
	"time"
)

type ETAEvaluation struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Summary     ETAEvaluationSummary  `json:"summary"`
	Rows        []ETAEvaluationRow    `json:"rows"`
	BySource    []ETAEvaluationBucket `json:"by_source"`
	ByOffset    []ETAEvaluationBucket `json:"by_offset"`
}

type ETAEvaluationSummary struct {
	AlertCount            int     `json:"alert_count"`
	HistoricalModelCount  int     `json:"historical_model_count"`
	APIFallbackCount      int     `json:"api_fallback_count"`
	AverageAbsErrorMinute float64 `json:"average_abs_error_minutes"`
}

type ETAEvaluationBucket struct {
	Name                  string  `json:"name"`
	Count                 int     `json:"count"`
	AverageLeadMinutes    float64 `json:"average_lead_minutes"`
	AverageErrorMinutes   float64 `json:"average_error_minutes"`
	AverageAbsErrorMinute float64 `json:"average_abs_error_minutes"`
}

type ETAEvaluationRow struct {
	ServiceDate      string     `json:"service_date"`
	OffsetMinutes    int        `json:"offset_minutes"`
	PredictionSource string     `json:"prediction_source"`
	Confidence       string     `json:"confidence,omitempty"`
	SentAt           time.Time  `json:"sent_at"`
	ArrivalAt        time.Time  `json:"arrival_at"`
	LeadMinutes      float64    `json:"lead_minutes"`
	ErrorMinutes     float64    `json:"error_minutes"`
	AbsoluteError    float64    `json:"absolute_error_minutes"`
	RunStatus        string     `json:"run_status,omitempty"`
	SampleCount      int        `json:"sample_count"`
	GPSSampleCount   int        `json:"gps_sample_count"`
	FirstCollectedAt *time.Time `json:"first_collected_at,omitempty"`
	LastCollectedAt  *time.Time `json:"last_collected_at,omitempty"`
}

func (s *Service) LoadETAEvaluation(limit int) (*ETAEvaluation, error) {
	if limit <= 0 {
		limit = 60
	}

	dates, err := s.ListHistoryDates(limit)
	if err != nil {
		return nil, err
	}

	rows := make([]ETAEvaluationRow, 0)
	for _, serviceDate := range dates {
		day, err := s.LoadHistoryDay(serviceDate)
		if err != nil || day == nil || day.ArrivalAt == nil {
			continue
		}

		deliveries := s.store.ListDeliveriesForDate(serviceDate)
		sort.Slice(deliveries, func(i, j int) bool {
			return deliveries[i].ReminderOffsetMinutes < deliveries[j].ReminderOffsetMinutes
		})

		for _, delivery := range deliveries {
			if delivery.DeliveryStatus != deliveryStatusSent || delivery.SentAt.IsZero() {
				continue
			}
			leadMinutes := day.ArrivalAt.Sub(delivery.SentAt).Minutes()
			errorMinutes := leadMinutes - float64(delivery.ReminderOffsetMinutes)
			rows = append(rows, ETAEvaluationRow{
				ServiceDate:      serviceDate,
				OffsetMinutes:    delivery.ReminderOffsetMinutes,
				PredictionSource: delivery.PredictionSource,
				Confidence:       delivery.Confidence,
				SentAt:           delivery.SentAt,
				ArrivalAt:        *day.ArrivalAt,
				LeadMinutes:      round1(leadMinutes),
				ErrorMinutes:     round1(errorMinutes),
				AbsoluteError:    round1(math.Abs(errorMinutes)),
				RunStatus:        day.RunStatus,
				SampleCount:      day.SampleCount,
				GPSSampleCount:   day.GPSSampleCount,
				FirstCollectedAt: day.FirstCollectedAt,
				LastCollectedAt:  day.LastCollectedAt,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ServiceDate == rows[j].ServiceDate {
			return rows[i].OffsetMinutes < rows[j].OffsetMinutes
		}
		return rows[i].ServiceDate > rows[j].ServiceDate
	})

	evaluation := &ETAEvaluation{
		GeneratedAt: s.now(),
		Rows:        rows,
		BySource:    buildETABuckets(rows, func(row ETAEvaluationRow) string { return firstNonEmpty(row.PredictionSource, "unknown") }),
		ByOffset:    buildETABuckets(rows, func(row ETAEvaluationRow) string { return intName(row.OffsetMinutes) + " min" }),
	}
	evaluation.Summary = buildETASummary(rows)
	return evaluation, nil
}

func buildETASummary(rows []ETAEvaluationRow) ETAEvaluationSummary {
	summary := ETAEvaluationSummary{AlertCount: len(rows)}
	if len(rows) == 0 {
		return summary
	}

	var totalAbs float64
	for _, row := range rows {
		totalAbs += row.AbsoluteError
		switch row.PredictionSource {
		case "historical_model":
			summary.HistoricalModelCount++
		case "api_estimated_time", "api_waiting_time":
			summary.APIFallbackCount++
		}
	}
	summary.AverageAbsErrorMinute = round1(totalAbs / float64(len(rows)))
	return summary
}

func buildETABuckets(rows []ETAEvaluationRow, key func(ETAEvaluationRow) string) []ETAEvaluationBucket {
	type aggregate struct {
		count  int
		lead   float64
		err    float64
		absErr float64
	}

	aggregates := make(map[string]*aggregate)
	for _, row := range rows {
		name := key(row)
		if name == "" {
			name = "unknown"
		}
		agg := aggregates[name]
		if agg == nil {
			agg = &aggregate{}
			aggregates[name] = agg
		}
		agg.count++
		agg.lead += row.LeadMinutes
		agg.err += row.ErrorMinutes
		agg.absErr += row.AbsoluteError
	}

	buckets := make([]ETAEvaluationBucket, 0, len(aggregates))
	for name, agg := range aggregates {
		buckets = append(buckets, ETAEvaluationBucket{
			Name:                  name,
			Count:                 agg.count,
			AverageLeadMinutes:    round1(agg.lead / float64(agg.count)),
			AverageErrorMinutes:   round1(agg.err / float64(agg.count)),
			AverageAbsErrorMinute: round1(agg.absErr / float64(agg.count)),
		})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})
	return buckets
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func intName(value int) string {
	return strconv.Itoa(value)
}
