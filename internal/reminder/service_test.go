package reminder

import (
	"context"
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
	service := NewService(cfg, &fakeEupfinClient{}, &fakeNotifier{}, &fakeNotifier{}, newFakeStore(), newFakeHistoryStore())
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
	historyStore.historicalSamples = []history.HistoricalSample{
		{
			RunID:       "2026-04-06",
			ServiceDate: "2026-04-06",
			TruckLat:    24.74445,
			TruckLng:    121.01112,
			CollectedAt: now.AddDate(0, 0, -7),
			ArrivalAt:   now.AddDate(0, 0, -7).Add(8 * time.Minute),
		},
		{
			RunID:       "2026-03-30",
			ServiceDate: "2026-03-30",
			TruckLat:    24.74444,
			TruckLng:    121.01111,
			CollectedAt: now.AddDate(0, 0, -14),
			ArrivalAt:   now.AddDate(0, 0, -14).Add(9 * time.Minute),
		},
		{
			RunID:       "2026-03-23",
			ServiceDate: "2026-03-23",
			TruckLat:    24.74446,
			TruckLng:    121.01110,
			CollectedAt: now.AddDate(0, 0, -21),
			ArrivalAt:   now.AddDate(0, 0, -21).Add(8 * time.Minute),
		},
	}

	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		cars: []eupfin.CarStatus{
			{RouteID: 461, GISX: 121.011111, GISY: 24.744444},
		},
	}, alerts, &fakeNotifier{}, store, historyStore)
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
}

func TestCheckOnceFallsBackToAPIWhenHistoryUnavailable(t *testing.T) {
	cfg := testConfig()
	alerts := &fakeNotifier{}
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
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
	}, alerts, &fakeNotifier{}, newFakeStore(), newFakeHistoryStore())
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 22, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	if len(alerts.messages) != 2 {
		t.Fatalf("expected both alert thresholds from API fallback, got %d", len(alerts.messages))
	}
	if !strings.Contains(alerts.messages[0], "api_estimated_time") || !strings.Contains(alerts.messages[1], "api_estimated_time") {
		t.Fatalf("expected api_estimated_time in alert messages, got %+v", alerts.messages)
	}
}

func TestCheckOnceMarksRunCompletedOnArrival(t *testing.T) {
	cfg := testConfig()
	historyStore := newFakeHistoryStore()
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
		cars: []eupfin.CarStatus{
			{RouteID: 461, GISX: 121.020320, GISY: 24.748448},
		},
	}, &fakeNotifier{}, &fakeNotifier{}, newFakeStore(), historyStore)
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
	if run == nil || run.Status != "completed" {
		t.Fatalf("expected run to be marked completed, got %+v", run)
	}
}

func TestSendStartupTestMessageUsesStartupNotifier(t *testing.T) {
	cfg := testConfig()
	startup := &fakeNotifier{}
	service := NewService(cfg, &fakeEupfinClient{
		district: baseDistrict(),
		target:   baseTarget(),
	}, &fakeNotifier{}, startup, newFakeStore(), newFakeHistoryStore())

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

type fakeEupfinClient struct {
	district *eupfin.DistrictConfig
	target   *eupfin.TargetStop
	cars     []eupfin.CarStatus
	statuses []eupfin.RouteStatus
}

func (f *fakeEupfinClient) GetDistrictByCustID(context.Context, int) (*eupfin.DistrictConfig, error) {
	return f.district, nil
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

type fakeHistoryStore struct {
	runs              map[string]*history.Run
	samples           []history.Sample
	historicalSamples []history.HistoricalSample
	historicalRuns    int
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

func (f *fakeHistoryStore) MarkRunCompleted(runID string, arrivalAt time.Time, arrivalSource string) error {
	run := f.runs[runID]
	if run == nil {
		return nil
	}
	run.Status = "completed"
	run.ArrivalAt = &arrivalAt
	run.ArrivalSource = arrivalSource
	return nil
}

func (f *fakeHistoryStore) PruneBefore(time.Time) error { return nil }

func (f *fakeHistoryStore) ListHistoricalSamples(time.Weekday, int, int, time.Time) ([]history.HistoricalSample, int, error) {
	return f.historicalSamples, f.historicalRuns, nil
}

func testConfig() *config.Config {
	return &config.Config{
		TelegramBotToken:       "token",
		TelegramChatID:         "chat",
		EupfinBaseURL:          "https://example.com",
		StateFile:              "state.json",
		DatabaseFile:           "history.db",
		CheckInterval:          time.Minute,
		HTTPPort:               "8080",
		TargetCustID:           5005808,
		TargetRouteID:          461,
		TargetPointSeq:         27,
		TargetPointName:        "有謙家園",
		TargetDays:             []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday},
		CollectionStart:        "19:00",
		CollectionEnd:          "21:30",
		AlertOffsets:           []int{10, 3},
		HistoryWeeks:           8,
		ArrivalRadiusMeters:    80,
		MatchRadiusMeters:      250,
		MinHistoryRuns:         3,
		HABaseURL:              "http://homeassistant.local:8123",
		HAToken:                "token",
		HANotifyMode:           "webhook",
		HATTSTarget:            "garbage_truck",
		SendTestMessageOnStart: false,
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
		PointID:       27,
		PointSeq:      27,
		PointName:     "有謙家園",
		ScheduledTime: "20:30",
		GISX:          121.02032,
		GISY:          24.748448,
	}
}
