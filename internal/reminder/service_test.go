package reminder

import (
	"context"
	"strings"
	"testing"
	"time"

	"telegram-garbage-reminder/internal/config"
	"telegram-garbage-reminder/internal/eupfin"
	"telegram-garbage-reminder/internal/state"
)

func TestCheckOnceDeduplicatesReminder(t *testing.T) {
	cfg := testConfig()
	client := &fakeEupfinClient{
		district: &eupfin.DistrictConfig{CustID: 5005808, TeamID: 5005609},
		target: &eupfin.TargetStop{
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
		},
	}
	notifier := &fakeNotifier{}
	store := newFakeStore()
	service := NewService(cfg, client, notifier, store)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 20, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}
	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() second run returned error: %v", err)
	}

	if len(notifier.messages) != 1 {
		t.Fatalf("expected 1 reminder message after dedupe, got %d", len(notifier.messages))
	}
}

func TestCheckOnceFallsBackToStationWhenGPSUnavailable(t *testing.T) {
	cfg := testConfig()
	client := &fakeEupfinClient{
		district: &eupfin.DistrictConfig{CustID: 5005808, TeamID: 5005609},
		target: &eupfin.TargetStop{
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
		},
	}
	notifier := &fakeNotifier{}
	store := newFakeStore()
	service := NewService(cfg, client, notifier, store)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 29, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	if len(notifier.messages) != 2 {
		t.Fatalf("expected both overdue reminders to be sent, got %d", len(notifier.messages))
	}

	message := notifier.messages[len(notifier.messages)-1]
	if !strings.Contains(message, "GPS 暫不可用") {
		t.Fatalf("expected fallback text in message, got %s", message)
	}
	if !strings.Contains(message, "origin=24.748448,121.020320") {
		t.Fatalf("expected fallback map origin to use station coordinates, got %s", message)
	}
}

func TestCheckOnceUsesActualGPSWhenAvailable(t *testing.T) {
	cfg := testConfig()
	client := &fakeEupfinClient{
		district: &eupfin.DistrictConfig{CustID: 5005808, TeamID: 5005609},
		target: &eupfin.TargetStop{
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
		},
		cars: []eupfin.CarStatus{
			{
				RouteID: 461,
				GISX:    121.011111,
				GISY:    24.744444,
			},
		},
	}
	notifier := &fakeNotifier{}
	store := newFakeStore()
	service := NewService(cfg, client, notifier, store)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 20, 29, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() returned error: %v", err)
	}

	message := notifier.messages[len(notifier.messages)-1]
	if !strings.Contains(message, "即時 GPS") {
		t.Fatalf("expected actual gps label in message, got %s", message)
	}
	if !strings.Contains(message, "origin=24.744444,121.011111") {
		t.Fatalf("expected actual vehicle origin in map link, got %s", message)
	}
}

func TestSendStartupTestMessageIncludesCoordinatesAndCooldown(t *testing.T) {
	cfg := testConfig()
	client := &fakeEupfinClient{
		district: &eupfin.DistrictConfig{CustID: 5005808, TeamID: 5005609},
		target: &eupfin.TargetStop{
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
		},
		cars: []eupfin.CarStatus{
			{
				RouteID: 461,
				GISX:    121.011111,
				GISY:    24.744444,
			},
		},
	}
	notifier := &fakeNotifier{}
	store := newFakeStore()
	service := NewService(cfg, client, notifier, store)
	service.now = func() time.Time {
		return time.Date(2026, 4, 13, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	}

	if err := service.SendStartupTestMessage(context.Background()); err != nil {
		t.Fatalf("SendStartupTestMessage() returned error: %v", err)
	}

	if len(notifier.messages) != 1 {
		t.Fatalf("expected 1 startup test message, got %d", len(notifier.messages))
	}

	message := notifier.messages[0]
	if !strings.Contains(message, "🧪 啟動測試提醒") {
		t.Fatalf("expected startup test header, got %s", message)
	}
	if !strings.Contains(message, "垃圾車座標：24.744444, 121.011111") {
		t.Fatalf("expected vehicle coordinates in message, got %s", message)
	}
	if !strings.Contains(message, "有謙家園座標：24.748448, 121.020320") {
		t.Fatalf("expected target coordinates in message, got %s", message)
	}
	if !strings.Contains(message, "GPS 查詢冷卻：5m0s") {
		t.Fatalf("expected cooldown text in message, got %s", message)
	}
}

func TestResolveVehicleSnapshotUsesCooldownCache(t *testing.T) {
	cfg := testConfig()
	cfg.GPSRefreshInterval = 5 * time.Minute
	client := &fakeEupfinClient{
		district: &eupfin.DistrictConfig{CustID: 5005808, TeamID: 5005609},
		target: &eupfin.TargetStop{
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
		},
		cars: []eupfin.CarStatus{
			{
				RouteID: 461,
				GISX:    121.011111,
				GISY:    24.744444,
			},
		},
	}
	notifier := &fakeNotifier{}
	store := newFakeStore()
	service := NewService(cfg, client, notifier, store)
	baseTime := time.Date(2026, 4, 13, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	service.now = func() time.Time { return baseTime }

	target := &state.CachedTarget{
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

	first := service.resolveVehicleSnapshot(context.Background(), target)
	client.cars = []eupfin.CarStatus{
		{
			RouteID: 461,
			GISX:    121.099999,
			GISY:    24.799999,
		},
	}
	second := service.resolveVehicleSnapshot(context.Background(), target)

	if first.Lat != second.Lat || first.Lng != second.Lng {
		t.Fatalf("expected cached snapshot within cooldown, got first=%+v second=%+v", first, second)
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

func (f *fakeEupfinClient) ResolveTargetStop(context.Context, int, int, int, string, string) (*eupfin.TargetStop, error) {
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

func testConfig() *config.Config {
	return &config.Config{
		TelegramBotToken: "token",
		TelegramChatID:   "chat",
		EupfinBaseURL:    "https://example.com",
		StateFile:        "state.json",
		CheckInterval:    time.Minute,
		GPSRefreshInterval: 5 * time.Minute,
		SendTestMessageOnStart: true,
		TargetCustID:     5005808,
		TargetRouteID:    461,
		TargetPointSeq:   27,
		TargetPointName:  "有謙家園",
		TargetTime:       "20:30",
		TargetDays: []time.Weekday{
			time.Monday,
			time.Tuesday,
			time.Thursday,
			time.Friday,
		},
		ReminderOffsets: []int{10, 1},
	}
}
