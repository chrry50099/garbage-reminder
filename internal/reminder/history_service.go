package reminder

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"telegram-garbage-reminder/internal/history"
	"telegram-garbage-reminder/internal/state"
	"telegram-garbage-reminder/internal/utils"
)

func (s *Service) ListHistoryDates(limit int) ([]string, error) {
	return s.history.ListServiceDates(limit)
}

func (s *Service) LoadTodayHistory() (*history.DayData, error) {
	serviceDate := s.now().In(utils.GetTaiwanTimezone()).Format("2006-01-02")
	return s.LoadHistoryDay(serviceDate)
}

func (s *Service) LoadHistoryDay(serviceDate string) (*history.DayData, error) {
	run, err := s.history.GetRun(serviceDate)
	if err != nil {
		return nil, fmt.Errorf("get run for %s: %w", serviceDate, err)
	}

	samples, err := s.history.ListSamplesByServiceDate(serviceDate)
	if err != nil {
		return nil, fmt.Errorf("list samples for %s: %w", serviceDate, err)
	}

	deliveries := s.store.ListDeliveriesForDate(serviceDate)
	target := s.store.GetCachedTarget()
	day := s.buildDayData(serviceDate, run, target, deliveries, samples)
	if err := s.writeDayExports(day); err != nil {
		return nil, err
	}
	return day, nil
}

func (s *Service) buildDayData(
	serviceDate string,
	run *history.Run,
	target *state.CachedTarget,
	deliveries []state.DeliveryRecord,
	samples []history.Sample,
) *history.DayData {
	day := &history.DayData{
		ServiceDate:      serviceDate,
		CollectionWindow: fmt.Sprintf("%s-%s", s.cfg.CollectionStart, s.cfg.CollectionEnd),
		SharedDataPath:   s.cfg.SharedDataDir,
		CheckInterval:    s.cfg.CheckInterval.String(),
	}

	if target != nil {
		day.RouteName = target.RouteName
		day.PointName = target.PointName
		day.PointSeq = target.PointSeq
		day.TargetLat = target.GISY
		day.TargetLng = target.GISX
	}

	if run != nil {
		day.RunStatus = run.Status
		day.TargetLat = run.TargetLat
		day.TargetLng = run.TargetLng
		if run.ArrivalAt != nil {
			arrivalAt := *run.ArrivalAt
			day.ArrivalAt = &arrivalAt
		}
	}

	for _, delivery := range deliveries {
		if delivery.DeliveryStatus == deliveryStatusSent {
			day.NotifiedOffsets = append(day.NotifiedOffsets, delivery.ReminderOffsetMinutes)
		}
	}

	for _, sample := range samples {
		day.Samples = append(day.Samples, history.DaySample{
			CollectedAt:         sample.CollectedAt,
			GPSAvailable:        sample.GPSAvailable,
			TruckLat:            sample.TruckLat,
			TruckLng:            sample.TruckLng,
			APIEstimatedMinutes: sample.APIEstimatedMinutes,
			APIEstimatedText:    sample.APIEstimatedText,
			APIWaitingTime:      sample.APIWaitingTime,
			ProgressMeters:      sample.ProgressMeters,
			SegmentIndex:        sample.SegmentIndex,
			LateralOffsetMeters: sample.LateralOffsetMeters,
		})
	}

	day.Finalize()
	return day
}

func (s *Service) writeDayExports(day *history.DayData) error {
	if day == nil {
		return nil
	}

	jsonPath, csvPath := history.BuildExportPaths(s.cfg.ExportsDir, day.ServiceDate)
	day.JSONExportPath = jsonPath
	day.CSVExportPath = csvPath

	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return fmt.Errorf("create exports directory: %w", err)
	}

	jsonBytes, err := day.MarshalJSONBytes()
	if err != nil {
		return fmt.Errorf("marshal day json export: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		return fmt.Errorf("write day json export: %w", err)
	}

	csvBytes, err := day.MarshalCSV()
	if err != nil {
		return fmt.Errorf("marshal day csv export: %w", err)
	}
	if err := os.WriteFile(csvPath, csvBytes, 0o644); err != nil {
		return fmt.Errorf("write day csv export: %w", err)
	}

	return nil
}

func (s *Service) updateStatusWithDayCounts(snapshot StatusSnapshot, serviceDate string) StatusSnapshot {
	day, err := s.LoadHistoryDay(serviceDate)
	if err != nil {
		s.logCollectorError("load_day_history", serviceDate, s.now().In(utils.GetTaiwanTimezone()), err)
		return snapshot
	}

	snapshot.TodaySampleCount = day.SampleCount
	snapshot.TodayGPSSamples = day.GPSSampleCount
	if day.LastCollectedAt != nil {
		lastCollectedAt := *day.LastCollectedAt
		snapshot.LastCollectedAt = &lastCollectedAt
	}
	return snapshot
}

func (s *Service) logCollectorSample(
	serviceDate string,
	collectedAt time.Time,
	sample history.Sample,
	prediction *history.Prediction,
	runStatus string,
) {
	if s.collectorLog == nil {
		return
	}

	entry := collectorSampleLog{
		ServiceDate:         serviceDate,
		CollectedAt:         isoTime(collectedAt),
		GPSAvailable:        sample.GPSAvailable,
		TruckLat:            sample.TruckLat,
		TruckLng:            sample.TruckLng,
		ProgressMeters:      sample.ProgressMeters,
		SegmentIndex:        sample.SegmentIndex,
		LateralOffsetMeters: sample.LateralOffsetMeters,
		APIEstimatedMinutes: sample.APIEstimatedMinutes,
		APIWaitingTime:      sample.APIWaitingTime,
		RunStatus:           runStatus,
	}
	if prediction != nil {
		entry.PredictionSource = prediction.Source
	}
	if err := s.collectorLog.LogSample(entry); err != nil {
		log.Printf("collector log write failed: %v", err)
	}
}

func (s *Service) logCollectorError(step, serviceDate string, collectedAt time.Time, err error) {
	if s.collectorLog == nil || err == nil {
		return
	}
	if logErr := s.collectorLog.LogError(collectorErrorLog{
		ServiceDate: serviceDate,
		CollectedAt: isoTime(collectedAt),
		Step:        step,
		Error:       err.Error(),
	}); logErr != nil {
		log.Printf("collector error log write failed: %v", logErr)
	}
}

func deliveriesToOffsets(deliveries []state.DeliveryRecord) []int {
	offsets := make([]int, 0, len(deliveries))
	for _, delivery := range deliveries {
		if delivery.DeliveryStatus == deliveryStatusSent {
			offsets = append(offsets, delivery.ReminderOffsetMinutes)
		}
	}
	sort.Ints(offsets)
	return offsets
}
