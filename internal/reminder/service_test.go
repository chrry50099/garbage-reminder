package reminder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"telegram-garbage-reminder/internal/config"
	"telegram-garbage-reminder/internal/eupfin"
	"telegram-garbage-reminder/internal/history"
	"telegram-garbage-reminder/internal/state"
)

func TestCheckOnceSkipsOutsideCollectionWindow(t *testing.T) {
	cfg := testConfig()
	service := NewService(cfg, &fakeEupfinClient{}, &fakeNotifier{}, &fakeNotifier{}, newFakeStore(), newFakeHistoryStore(), nil)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 18, 59, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	status := service.CurrentStatus()
	if status.Active {
		t.Fatal("expected service to stay inactive outside collection window")
	}
	if !strings.Contains(status.Message, "outside") {
		t.Fatalf("unexpected status message: %s", status.Message)
	}
}

func TestCheckOnceUsesHistoricalPredictionAndSendsAlert(t *testing.T) {
	cfg := testConfig()
	alerts := &fakeNotifier{}
	store := newFakeStore()
	historyStore := newFakeHistoryStore()
	historyStore.historicalRuns = 3
	now := time.Date(2026, 4, 13, 20, 22, 0, 0, time.FixedZone("CST", 8*3600))
	currentProjection := mustProjectBaseRoute(t, 24.745500, 121.013500, nil)

	historyStore.historicalSamples = []history.HistoricalSample{
		historicalSampleForProgress("2026-04-06", now.AddDate(0, 0, -7), currentProjection.ProgressMeters-4, 6, 8),
		historicalSampleForProgress("2026-03-30", now.AddDate(0, 0, -14), currentProjection.ProgressMeters+3, 10, 9),
		historicalSampleForProgress("2026-03-23", now.AddDate(0, 0, -21), currentProjection.ProgressMeters+9, 12, 8),
	}

	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
		cars: []eupfin.CarStatus{
			{CarUnicode: "68715", LogGISX: 121.013500, LogGISY: 24.745500},
		},
	}, alerts, &fakeNotifier{}, store, historyStore, nil)
	service.now = func() time.Time { return now }

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	if len(alerts.messages) != 1 {
		t.Fatalf("expected one 10-minute alert, got %d", len(alerts.messages))
	}
	if !strings.Contains(alerts.messages[0], "historical_model") {
		t.Fatalf("expected historical_model in alert message, got %s", alerts.messages[0])
	}
	if !store.HasDelivery("2026-04-13|10") {
		t.Fatal("expected 10-minute delivery record to be stored")
	}
	if len(historyStore.samples) != 1 || historyStore.samples[0].ProgressMeters == nil {
		t.Fatalf("expected inserted sample to include route progress, got %+v", historyStore.samples)
	}
}

func TestCheckOnceFallsBackToAPIWhenHistoryUnavailable(t *testing.T) {
	cfg := testConfig()
	alerts := &fakeNotifier{}
	store := newFakeStore()
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
		statuses: []eupfin.RouteStatus{
			{
				RouteID: 461,
				Points: []eupfin.RouteStatusPoint{
					{
						PointID:       27,
						EstimatedTime: "20:25",
						WaitingTime:   3,
					},
				},
			},
		},
	}, alerts, &fakeNotifier{}, store, newFakeHistoryStore(), nil)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 22, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	if len(alerts.messages) != 1 {
		t.Fatalf("expected only the 3-minute alert from API fallback, got %d", len(alerts.messages))
	}
	if !strings.Contains(alerts.messages[0], "api_estimated_time") || store.HasDelivery("2026-04-13|10") {
		t.Fatalf("expected api_estimated_time in alert messages, got %+v", alerts.messages)
	}
}

func TestCheckOnceFallsBackToAPIWhenRouteShapeUnavailable(t *testing.T) {
	cfg := testConfig()
	alerts := &fakeNotifier{}
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routeErr: errors.New("route basic data unavailable"),
		statuses: []eupfin.RouteStatus{
			{
				RouteID: 461,
				Points: []eupfin.RouteStatusPoint{
					{
						PointID:       27,
						EstimatedTime: "20:25",
						WaitingTime:   3,
					},
				},
			},
		},
	}, alerts, &fakeNotifier{}, newFakeStore(), newFakeHistoryStore(), nil)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 22, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	if len(alerts.messages) != 1 {
		t.Fatalf("expected only the 3-minute API fallback alert, got %d", len(alerts.messages))
	}
	if !strings.Contains(alerts.messages[0], "api_estimated_time") {
		t.Fatalf("expected api_estimated_time alert, got %+v", alerts.messages)
	}
}

func TestCheckOnceDoesNotMarkRunCompletedWhenTargetIsNear(t *testing.T) {
	cfg := testConfig()
	historyStore := newFakeHistoryStore()
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
		cars: []eupfin.CarStatus{
			{CarUnicode: "68715", LogGISX: 121.020320, LogGISY: 24.748448},
		},
		statuses: []eupfin.RouteStatus{
			{
				RouteID:          461,
				RouteWaitingTime: 1,
				Points: []eupfin.RouteStatusPoint{
					{
						PointID:       27,
						EstimatedTime: "20:29",
						WaitingTime:   1,
					},
				},
			},
		},
	}, &fakeNotifier{}, &fakeNotifier{}, newFakeStore(), historyStore, nil)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 29, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	run, err := historyStore.GetRun("2026-04-13")
	if err != nil {
		t.Fatalf("GetRun() returned error: %v", err)
	}
	if run == nil || run.Status != "post_arrival" || run.ArrivalAt == nil || run.ArrivalSource != "gps_radius" {
		t.Fatalf("expected run to enter post_arrival near target without completing, got %+v", run)
	}
}

func TestCheckOnceContinuesRecordingLoopAndCompletesAfterDeparture(t *testing.T) {
	cfg := testConfig()
	cfg.PostArrivalDepartureMeters = 500
	cfg.PostArrivalMaxDuration = 30 * time.Minute
	historyStore := newFakeHistoryStore()
	alerts := &fakeNotifier{}
	client := &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
		cars: []eupfin.CarStatus{
			{CarUnicode: "68715", LogGISX: 121.020320, LogGISY: 24.748448},
		},
		statuses: []eupfin.RouteStatus{
			{
				RouteID:          461,
				RouteWaitingTime: -2,
				Points: []eupfin.RouteStatusPoint{
					{
						PointID:       27,
						EstimatedTime: "",
						WaitingTime:   -2,
					},
				},
			},
		},
	}
	service := NewService(cfg, client, alerts, &fakeNotifier{}, newFakeStore(), historyStore, nil)
	baseTime := time.Date(2026, 4, 13, 20, 15, 0, 0, time.FixedZone("CST", 8*3600))
	service.now = func() time.Time { return baseTime }

	// Tick 1: arrives at 有謙家園 and route reports -2 -> enters post_arrival, keeps recording
	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("tick 1 CheckOnce() error: %v", err)
	}
	run, _ := historyStore.GetRun("2026-04-13")
	if run == nil || run.Status != "post_arrival" || run.ArrivalAt == nil || !run.ArrivalAt.Equal(baseTime) {
		t.Fatalf("expected post_arrival with arrival_at=%v, got %+v", baseTime, run)
	}

	// Tick 2: loops inside 有謙家園 (~200m away) -> still post_arrival, sample recorded
	client.cars = []eupfin.CarStatus{{CarUnicode: "68715", LogGISX: 121.022000, LogGISY: 24.749200}}
	service.now = func() time.Time { return baseTime.Add(20 * time.Second) }
	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("tick 2 CheckOnce() error: %v", err)
	}
	run, _ = historyStore.GetRun("2026-04-13")
	if run.Status != "post_arrival" || len(historyStore.samples) != 2 {
		t.Fatalf("expected continued post_arrival recording inside community, status=%s samples=%d", run.Status, len(historyStore.samples))
	}

	// Ticks 3-5: departs >= 500m away for 3 consecutive samples -> completed, preserving original arrival_at
	for i := 1; i <= 3; i++ {
		client.cars = []eupfin.CarStatus{{CarUnicode: "68715", LogGISX: 121.013000, LogGISY: 24.752000}}
		service.now = func() time.Time { return baseTime.Add(time.Duration(20+i*20) * time.Second) }
		if err := service.CheckOnce(context.Background()); err != nil {
			t.Fatalf("departure tick %d CheckOnce() error: %v", i, err)
		}
	}
	run, _ = historyStore.GetRun("2026-04-13")
	if run.Status != "completed" {
		t.Fatalf("expected run to complete after 3 departed samples, got %s", run.Status)
	}
	if run.ArrivalAt == nil || !run.ArrivalAt.Equal(baseTime) {
		t.Fatalf("expected original arrival_at %v to be preserved, got %v", baseTime, run.ArrivalAt)
	}
	if len(historyStore.samples) != 5 {
		t.Fatalf("expected all 5 samples to be recorded, got %d", len(historyStore.samples))
	}
	if len(alerts.messages) != 0 {
		t.Fatalf("expected no alerts after arrival, got %d", len(alerts.messages))
	}
}

func TestCheckOnceCompletesPostArrivalAtMaxDuration(t *testing.T) {
	cfg := testConfig()
	cfg.PostArrivalDepartureMeters = 500
	cfg.PostArrivalMaxDuration = 30 * time.Minute
	historyStore := newFakeHistoryStore()
	client := &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
		cars:     []eupfin.CarStatus{{CarUnicode: "68715", LogGISX: 121.020320, LogGISY: 24.748448}},
	}
	service := NewService(cfg, client, &fakeNotifier{}, &fakeNotifier{}, newFakeStore(), historyStore, nil)
	baseTime := time.Date(2026, 4, 13, 20, 15, 0, 0, time.FixedZone("CST", 8*3600))
	service.now = func() time.Time { return baseTime }

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("tick 1 error: %v", err)
	}
	service.now = func() time.Time { return baseTime.Add(30 * time.Minute) }
	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("tick 2 error: %v", err)
	}
	run, _ := historyStore.GetRun("2026-04-13")
	if run == nil || run.Status != "completed" || run.ArrivalAt == nil || !run.ArrivalAt.Equal(baseTime) {
		t.Fatalf("expected completed run after max post-arrival duration with preserved arrival_at, got %+v", run)
	}
}

func TestCheckOnceContinuesWhenPruneFailsAndPrunesOncePerDay(t *testing.T) {
	cfg := testConfig()
	historyStore := newFakeHistoryStore()
	historyStore.pruneErr = errors.New("database is locked (SQLITE_BUSY)")
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
		cars:     []eupfin.CarStatus{{CarUnicode: "68715", LogGISX: 121.013500, LogGISY: 24.745500}},
	}, &fakeNotifier{}, &fakeNotifier{}, newFakeStore(), historyStore, nil)
	now := time.Date(2026, 4, 13, 20, 5, 0, 0, time.FixedZone("CST", 8*3600))
	service.now = func() time.Time { return now }

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("expected CheckOnce to succeed even when PruneBefore fails, got: %v", err)
	}
	if len(historyStore.samples) != 1 {
		t.Fatalf("expected sample to be collected despite prune error, got %d", len(historyStore.samples))
	}

	historyStore.pruneErr = nil
	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("second CheckOnce error: %v", err)
	}
	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("third CheckOnce error: %v", err)
	}
	if historyStore.pruneCalls != 2 {
		t.Fatalf("expected PruneBefore to stop retrying once succeeded for the day (2 calls total), got %d", historyStore.pruneCalls)
	}
}

func TestSendStartupTestMessageUsesStartupNotifier(t *testing.T) {
	cfg := testConfig()
	startup := &fakeNotifier{}
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
	}, &fakeNotifier{}, startup, newFakeStore(), newFakeHistoryStore(), nil)

	if err := service.SendStartupTestMessage(context.Background()); err != nil {
		t.Fatalf("SendStartupTestMessage() returned error: %v", err)
	}
	if len(startup.messages) != 1 {
		t.Fatalf("expected 1 startup message, got %d", len(startup.messages))
	}
	if !strings.Contains(startup.messages[0], "收集時窗：19:00-21:30") {
		t.Fatalf("unexpected startup message: %s", startup.messages[0])
	}
}

func TestCheckOnceUsesLogCoordinatesWhenLegacyGISFieldsAreEmpty(t *testing.T) {
	cfg := testConfig()
	historyStore := newFakeHistoryStore()
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		routes:   []eupfin.Route{baseRoute()},
		cars: []eupfin.CarStatus{
			{
				CarUnicode: "68715",
				CarNumber:  "KEG-5915",
				LogGISX:    121.013500,
				LogGISY:    24.745500,
			},
		},
	}, &fakeNotifier{}, &fakeNotifier{}, newFakeStore(), historyStore, nil)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 5, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	if len(historyStore.samples) != 1 {
		t.Fatalf("expected one inserted sample, got %d", len(historyStore.samples))
	}
	if !historyStore.samples[0].GPSAvailable {
		t.Fatalf("expected sample to use GPS from log coordinates, got %+v", historyStore.samples[0])
	}
	if historyStore.samples[0].TruckLat != 24.745500 || historyStore.samples[0].TruckLng != 121.013500 {
		t.Fatalf("unexpected truck coordinates: %+v", historyStore.samples[0])
	}
}

type fakeEupfinClient struct {
	district *eupfin.DistrictConfig
	target   *eupfin.TargetStop
	routes   []eupfin.Route
	routeErr error
	cars     []eupfin.CarStatus
	statuses []eupfin.RouteStatus
}

func (f *fakeEupfinClient) GetDistrictByCustID(context.Context, int) (*eupfin.DistrictConfig, error) {
	return f.district, nil
}

func (f *fakeEupfinClient) GetAllRouteBasicData(context.Context, int) ([]eupfin.Route, error) {
	if f.routeErr != nil {
		return nil, f.routeErr
	}
	if len(f.routes) > 0 {
		return f.routes, nil
	}
	if f.target == nil {
		return nil, nil
	}
	return []eupfin.Route{baseRoute()}, nil
}

func (f *fakeEupfinClient) ResolveTargetStop(context.Context, int, int, int, string) (*eupfin.TargetStop, error) {
	return f.target, nil
}

func (f *fakeEupfinClient) GetCarStatusGarbage(context.Context, int, int) ([]eupfin.CarStatus, error) {
	return f.cars, nil
}

func (f *fakeEupfinClient) GetAllRouteStatusData(context.Context, int) ([]eupfin.RouteStatus, error) {
	return f.statuses, nil
}

type fakeNotifier struct {
	messages []string
}

func (f *fakeNotifier) SendMessage(_ context.Context, text string) error {
	f.messages = append(f.messages, text)
	return nil
}

type fakeStore struct {
	cachedTarget *state.CachedTarget
	deliveries   map[string]state.DeliveryRecord
}

func newFakeStore() *fakeStore {
	return &fakeStore{deliveries: make(map[string]state.DeliveryRecord)}
}

func (f *fakeStore) SaveCachedTarget(target state.CachedTarget) error {
	copy := target
	f.cachedTarget = &copy
	return nil
}

func (f *fakeStore) GetCachedTarget() *state.CachedTarget {
	if f.cachedTarget == nil {
		return nil
	}
	copy := *f.cachedTarget
	return &copy
}

func (f *fakeStore) HasDelivery(deliveryKey string) bool {
	_, ok := f.deliveries[deliveryKey]
	return ok
}

func (f *fakeStore) RecordDelivery(deliveryKey string, record state.DeliveryRecord) error {
	f.deliveries[deliveryKey] = record
	return nil
}

func (f *fakeStore) ListDeliveriesForDate(serviceDate string) []state.DeliveryRecord {
	results := make([]state.DeliveryRecord, 0)
	for _, record := range f.deliveries {
		if record.ScheduledDate == serviceDate {
			results = append(results, record)
		}
	}
	return results
}

type fakeHistoryStore struct {
	runs              map[string]*history.Run
	samples           []history.Sample
	historicalSamples []history.HistoricalSample
	historicalRuns    int
	recentSamples     []history.RecentSample
	pruneErr          error
	pruneCalls        int
}

func newFakeHistoryStore() *fakeHistoryStore {
	return &fakeHistoryStore{runs: make(map[string]*history.Run)}
}

func (f *fakeHistoryStore) EnsureRun(serviceDate string, weekday time.Weekday, routeID, pointID int, targetLat, targetLng float64, startedAt time.Time) (*history.Run, error) {
	if existing, ok := f.runs[serviceDate]; ok {
		return existing, nil
	}
	run := &history.Run{
		RunID:       serviceDate,
		ServiceDate: serviceDate,
		Weekday:     weekday,
		RouteID:     routeID,
		PointID:     pointID,
		StartedAt:   startedAt,
		Status:      "collecting",
		TargetLat:   targetLat,
		TargetLng:   targetLng,
	}
	f.runs[serviceDate] = run
	return run, nil
}

func (f *fakeHistoryStore) GetRun(runID string) (*history.Run, error) {
	return f.runs[runID], nil
}

func (f *fakeHistoryStore) InsertSample(sample history.Sample) error {
	f.samples = append(f.samples, sample)
	if run := f.runs[sample.RunID]; run != nil {
		collectedAt := sample.CollectedAt
		run.LastCollectedAt = &collectedAt
	}
	return nil
}

func (f *fakeHistoryStore) MarkRunArrived(runID string, arrivalAt time.Time, arrivalSource string) error {
	run := f.runs[runID]
	if run == nil || run.Status == "completed" {
		return nil
	}
	run.Status = "post_arrival"
	if run.ArrivalAt == nil {
		t := arrivalAt
		run.ArrivalAt = &t
	}
	if run.ArrivalSource == "" {
		run.ArrivalSource = arrivalSource
	}
	return nil
}

func (f *fakeHistoryStore) MarkRunCompleted(runID string, arrivalAt time.Time, arrivalSource string) error {
	run := f.runs[runID]
	if run == nil {
		return nil
	}
	run.Status = "completed"
	if run.ArrivalAt == nil {
		t := arrivalAt
		run.ArrivalAt = &t
	}
	if run.ArrivalSource == "" {
		run.ArrivalSource = arrivalSource
	}
	return nil
}

func (f *fakeHistoryStore) PruneBefore(time.Time) error {
	f.pruneCalls++
	return f.pruneErr
}

func (f *fakeHistoryStore) ListHistoricalSamples(time.Weekday, int, int, time.Time) ([]history.HistoricalSample, int, error) {
	return f.historicalSamples, f.historicalRuns, nil
}

func (f *fakeHistoryStore) ListRecentRunSamples(runID string, limit int) ([]history.RecentSample, error) {
	if len(f.recentSamples) > 0 {
		return append([]history.RecentSample(nil), f.recentSamples...), nil
	}
	matched := make([]history.RecentSample, 0)
	for _, sample := range f.samples {
		if sample.RunID != runID {
			continue
		}
		matched = append(matched, history.RecentSample{
			GPSAvailable:        sample.GPSAvailable,
			TruckLat:            sample.TruckLat,
			TruckLng:            sample.TruckLng,
			ProgressMeters:      sample.ProgressMeters,
			SegmentIndex:        sample.SegmentIndex,
			LateralOffsetMeters: sample.LateralOffsetMeters,
			CollectedAt:         sample.CollectedAt,
		})
	}
	if limit > 0 && len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	return matched, nil
}

func (f *fakeHistoryStore) ListServiceDates(limit int) ([]string, error) {
	results := make([]string, 0, len(f.runs))
	for serviceDate := range f.runs {
		results = append(results, serviceDate)
	}
	return results, nil
}

func (f *fakeHistoryStore) ListSamplesByServiceDate(serviceDate string) ([]history.Sample, error) {
	results := make([]history.Sample, 0)
	for _, sample := range f.samples {
		if sample.ServiceDate == serviceDate {
			results = append(results, sample)
		}
	}
	return results, nil
}

func testConfig() *config.Config {
	tempBase := filepath.Join(os.TempDir(), "garbage-reminder-test")
	return &config.Config{
		TelegramBotToken:              "token",
		TelegramChatID:                "chat",
		EupfinBaseURL:                 "https://example.com",
		SharedDataDir:                 "/share/garbage_eta",
		StateFile:                     filepath.Join(tempBase, "state.json"),
		DatabaseFile:                  filepath.Join(tempBase, "history.db"),
		CollectorLogFile:              filepath.Join(tempBase, "collector.log"),
		ExportsDir:                    filepath.Join(tempBase, "exports"),
		CheckInterval:                 time.Minute,
		HTTPPort:                      "8080",
		TargetCustID:                  5005808,
		TargetRouteID:                 461,
		TargetPointSeq:                27,
		TargetPointName:               "有謙家園",
		TargetDays:                    []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday},
		CollectionStart:               "19:00",
		CollectionEnd:                 "21:30",
		AlertOffsets:                  []int{10, 3},
		HistoryWeeks:                  8,
		ArrivalRadiusMeters:           80,
		MatchRadiusMeters:             250,
		MinHistoryRuns:                3,
		ProgressWindowMeters:          150,
		LateralOffsetLimitMeters:      80,
		BacktrackToleranceMeters:      30,
		AmbiguousSegmentEpsilonMeters: 15,
		HABaseURL:                     "http://homeassistant.local:8123",
		HAToken:                       "token",
		HANotifyMode:                  "webhook",
		HATTSTarget:                   "garbage_truck",
		SendTestMessageOnStart:        false,
	}
}

func baseDistrict() *eupfin.DistrictConfig {
	return &eupfin.DistrictConfig{CustID: 5005808, TeamID: 5005609}
}

func baseTarget() *eupfin.TargetStop {
	return &eupfin.TargetStop{
		CustID:        5005808,
		TeamID:        5005609,
		RouteID:       461,
		RouteName:     "雙溪線(每周一、二、四、五資源回收)",
		CarUnicode:    "68715",
		CarNumber:     "KEG-5915",
		PointID:       27,
		PointSeq:      27,
		PointName:     "有謙家園",
		ScheduledTime: "20:30",
		GISX:          121.02032,
		GISY:          24.748448,
	}
}

func baseRoute() eupfin.Route {
	return eupfin.Route{
		RouteID:    461,
		RouteName:  "雙溪線(每周一、二、四、五資源回收)",
		CarUnicode: "68715",
		CarNumber:  "KEG-5915",
		Points: []eupfin.Point{
			{PointID: 1, Seq: 1, PointName: "起點", GISX: 121.010000, GISY: 24.740000},
			{PointID: 10, Seq: 10, PointName: "中途", GISX: 121.011111, GISY: 24.744444},
			{PointID: 27, Seq: 27, PointName: "有謙家園", GISX: 121.020320, GISY: 24.748448},
		},
	}
}

func mustProjectBaseRoute(t *testing.T, lat, lng float64, recent []history.RecentSample) *history.ProjectionResult {
	t.Helper()

	shape, err := history.BuildRouteShape(baseRoute(), 27)
	if err != nil {
		t.Fatalf("BuildRouteShape() returned error: %v", err)
	}
	projection, ok := shape.Project(lat, lng, recent, history.ProjectionConfig{
		ProgressWindowMeters:          150,
		LateralOffsetLimitMeters:      80,
		BacktrackToleranceMeters:      30,
		AmbiguousSegmentEpsilonMeters: 15,
	})
	if !ok {
		t.Fatal("expected test projection to succeed")
	}
	return projection
}

func historicalSampleForProgress(runID string, collectedAt time.Time, progressValue, lateralValue float64, remainingMinutes int) history.HistoricalSample {
	progress := progressValue
	lateral := lateralValue
	return history.HistoricalSample{
		RunID:               runID,
		ServiceDate:         collectedAt.Format("2006-01-02"),
		CollectedAt:         collectedAt,
		ArrivalAt:           collectedAt.Add(time.Duration(remainingMinutes) * time.Minute),
		ProgressMeters:      &progress,
		LateralOffsetMeters: &lateral,
	}
}
