package reminder

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"telegram-garbage-reminder/internal/config"
	"telegram-garbage-reminder/internal/eupfin"
	"telegram-garbage-reminder/internal/state"
	"telegram-garbage-reminder/internal/utils"
)

const (
	deliveryStatusSent = "sent"
	gpsModeActual      = "actual"
	gpsModeFallback    = "fallback_station"
)

type eupfinClient interface {
	GetDistrictByCustID(ctx context.Context, custID int) (*eupfin.DistrictConfig, error)
	ResolveTargetStop(ctx context.Context, custID, routeID, pointSeq int, pointName, targetTime string) (*eupfin.TargetStop, error)
	GetCarStatusGarbage(ctx context.Context, custID, teamID int) ([]eupfin.CarStatus, error)
	GetAllRouteStatusData(ctx context.Context, custID int) ([]eupfin.RouteStatus, error)
}

type telegramNotifier interface {
	SendMessage(ctx context.Context, text string) error
}

type localStore interface {
	SaveCachedTarget(target state.CachedTarget) error
	GetCachedTarget() *state.CachedTarget
	HasDelivery(deliveryKey string) bool
	RecordDelivery(deliveryKey string, record state.DeliveryRecord) error
}

type Service struct {
	cfg      *config.Config
	client   eupfinClient
	notifier telegramNotifier
	store    localStore
	now      func() time.Time
	mu       sync.Mutex
	lastVehicleSnapshot liveVehicleSnapshot
	lastVehicleFetchedAt time.Time
}

type liveVehicleSnapshot struct {
	Lat        float64
	Lng        float64
	GPSMode    string
	StatusNote string
	FetchedAt  time.Time
}

func NewService(cfg *config.Config, client eupfinClient, notifier telegramNotifier, store localStore) *Service {
	offsets := append([]int(nil), cfg.ReminderOffsets...)
	slices.Sort(offsets)
	slices.Reverse(offsets)
	cfg.ReminderOffsets = offsets

	return &Service{
		cfg:      cfg,
		client:   client,
		notifier: notifier,
		store:    store,
		now:      utils.NowInTaiwan,
	}
}

func (s *Service) Initialize(ctx context.Context) error {
	target, err := s.refreshTarget(ctx)
	if err != nil {
		return err
	}

	log.Printf("Validated target stop: route=%s route_id=%d seq=%d point=%s time=%s",
		target.RouteName, target.RouteID, target.PointSeq, target.PointName, target.ScheduledTime)
	return nil
}

func (s *Service) SendStartupTestMessage(ctx context.Context) error {
	target, err := s.resolveTarget(ctx)
	if err != nil {
		return err
	}

	now := s.now().In(utils.GetTaiwanTimezone())
	scheduledAt, err := combineDateAndClock(now, target.ScheduledTime)
	if err != nil {
		return err
	}

	liveSnapshot := s.resolveVehicleSnapshot(ctx, target)
	message := s.buildStartupTestMessage(now, scheduledAt, target, liveSnapshot)
	if err := s.notifier.SendMessage(ctx, message); err != nil {
		return fmt.Errorf("send startup test message: %w", err)
	}

	log.Printf("Startup test message sent: gps_mode=%s", liveSnapshot.GPSMode)
	return nil
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()

	log.Printf("Reminder scheduler started, interval=%s", s.cfg.CheckInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Reminder scheduler stopped")
			return
		case <-ticker.C:
			if err := s.CheckOnce(ctx); err != nil {
				log.Printf("Reminder check failed: %v", err)
			}
		}
	}
}

func (s *Service) CheckOnce(ctx context.Context) error {
	now := s.now().In(utils.GetTaiwanTimezone())
	if !s.isTargetDay(now.Weekday()) {
		return nil
	}

	target, err := s.resolveTarget(ctx)
	if err != nil {
		return err
	}

	scheduledAt, err := combineDateAndClock(now, target.ScheduledTime)
	if err != nil {
		return err
	}

	if !now.Before(scheduledAt) {
		return nil
	}

	for _, offset := range s.cfg.ReminderOffsets {
		reminderAt := scheduledAt.Add(-time.Duration(offset) * time.Minute)
		if now.Before(reminderAt) {
			continue
		}

		deliveryKey := buildDeliveryKey(now.Format("2006-01-02"), offset)
		if s.store.HasDelivery(deliveryKey) {
			continue
		}

		if err := s.dispatchReminder(ctx, now, scheduledAt, offset, target, deliveryKey); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) dispatchReminder(ctx context.Context, now, scheduledAt time.Time, offset int, target *state.CachedTarget, deliveryKey string) error {
	liveSnapshot := s.resolveVehicleSnapshot(ctx, target)
	message := s.buildReminderMessage(now, scheduledAt, offset, target, liveSnapshot)

	if err := s.notifier.SendMessage(ctx, message); err != nil {
		return fmt.Errorf("send telegram reminder: %w", err)
	}

	record := state.DeliveryRecord{
		ScheduledDate:         now.Format("2006-01-02"),
		ReminderOffsetMinutes: offset,
		DeliveryStatus:        deliveryStatusSent,
		GPSMode:               liveSnapshot.GPSMode,
		SentAt:                s.now(),
	}

	if err := s.store.RecordDelivery(deliveryKey, record); err != nil {
		return fmt.Errorf("record delivery state: %w", err)
	}

	log.Printf("Reminder sent: offset=%d gps_mode=%s", offset, liveSnapshot.GPSMode)
	return nil
}

func (s *Service) refreshTarget(ctx context.Context) (*state.CachedTarget, error) {
	district, err := s.client.GetDistrictByCustID(ctx, s.cfg.TargetCustID)
	if err != nil {
		return nil, fmt.Errorf("resolve district config: %w", err)
	}

	target, err := s.client.ResolveTargetStop(
		ctx,
		s.cfg.TargetCustID,
		s.cfg.TargetRouteID,
		s.cfg.TargetPointSeq,
		s.cfg.TargetPointName,
		s.cfg.TargetTime,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve target stop: %w", err)
	}

	target.CustID = s.cfg.TargetCustID
	target.TeamID = district.TeamID

	cachedTarget := &state.CachedTarget{
		CustID:        target.CustID,
		TeamID:        target.TeamID,
		RouteID:       target.RouteID,
		RouteName:     target.RouteName,
		PointID:       target.PointID,
		PointSeq:      target.PointSeq,
		PointName:     target.PointName,
		ScheduledTime: target.ScheduledTime,
		GISX:          target.GISX,
		GISY:          target.GISY,
	}

	if err := s.store.SaveCachedTarget(*cachedTarget); err != nil {
		return nil, fmt.Errorf("save cached target: %w", err)
	}

	return cachedTarget, nil
}

func (s *Service) resolveTarget(ctx context.Context) (*state.CachedTarget, error) {
	target, err := s.refreshTarget(ctx)
	if err == nil {
		return target, nil
	}

	cached := s.store.GetCachedTarget()
	if cached == nil {
		return nil, err
	}

	log.Printf("Using cached target after refresh failure: %v", err)
	return cached, nil
}

func (s *Service) resolveVehicleSnapshot(ctx context.Context, target *state.CachedTarget) liveVehicleSnapshot {
	s.mu.Lock()
	if !s.lastVehicleFetchedAt.IsZero() && s.now().Sub(s.lastVehicleFetchedAt) < s.cfg.GPSRefreshInterval {
		cached := s.lastVehicleSnapshot
		s.mu.Unlock()
		return cached
	}
	s.mu.Unlock()

	fallback := liveVehicleSnapshot{
		Lat:        target.GISY,
		Lng:        target.GISX,
		GPSMode:    gpsModeFallback,
		StatusNote: fmt.Sprintf("GPS 暫不可用，已改用 %s 站點位置。", target.PointName),
		FetchedAt:  s.now(),
	}

	cars, err := s.client.GetCarStatusGarbage(ctx, target.CustID, target.TeamID)
	if err == nil {
		for _, car := range cars {
			if car.RouteID == target.RouteID && car.GISY != 0 && car.GISX != 0 {
				snapshot := liveVehicleSnapshot{
					Lat:        car.GISY,
					Lng:        car.GISX,
					GPSMode:    gpsModeActual,
					StatusNote: "已使用即時 GPS 車輛位置。",
					FetchedAt:  s.now(),
				}
				s.cacheVehicleSnapshot(snapshot)
				return snapshot
			}
		}
	}

	statuses, err := s.client.GetAllRouteStatusData(ctx, target.CustID)
	if err == nil {
		for _, routeStatus := range statuses {
			if routeStatus.RouteID != target.RouteID {
				continue
			}
			for _, pointStatus := range routeStatus.Points {
				if pointStatus.PointID != target.PointID {
					continue
				}
				if pointStatus.EstimatedTime != "" {
					fallback.StatusNote = fmt.Sprintf("GPS 暫不可用，已改用 %s 站點位置。API 估計到站：%s。", target.PointName, pointStatus.EstimatedTime)
					s.cacheVehicleSnapshot(fallback)
					return fallback
				}
				if pointStatus.WaitingTime >= 0 {
					fallback.StatusNote = fmt.Sprintf("GPS 暫不可用，已改用 %s 站點位置。API 等待狀態：%s。", target.PointName, waitingTimeText(pointStatus.WaitingTime))
					s.cacheVehicleSnapshot(fallback)
					return fallback
				}
			}
		}
	}

	s.cacheVehicleSnapshot(fallback)
	return fallback
}

func (s *Service) buildReminderMessage(now, scheduledAt time.Time, offset int, target *state.CachedTarget, vehicle liveVehicleSnapshot) string {
	reminderLabel := fmt.Sprintf("%d 分鐘前", offset)
	if offset == 1 {
		reminderLabel = "1 分鐘前"
	}

	gpsLabel := "站點備援"
	if vehicle.GPSMode == gpsModeActual {
		gpsLabel = "即時 GPS"
	}

	return fmt.Sprintf(
		"🗑️ 垃圾車提醒（%s）\n路線：%s\n站點：%s（第 %d 站）\n預定到站：%s\n目前時間：%s\n垃圾車定位：%s\n垃圾車座標：%.6f, %.6f\n有謙家園座標：%.6f, %.6f\n%s\n地圖：%s",
		reminderLabel,
		target.RouteName,
		target.PointName,
		target.PointSeq,
		scheduledAt.Format("2006-01-02 15:04"),
		now.Format("2006-01-02 15:04"),
		gpsLabel,
		vehicle.Lat,
		vehicle.Lng,
		target.GISY,
		target.GISX,
		vehicle.StatusNote,
		buildDualMarkerMapURL(vehicle.Lat, vehicle.Lng, target.GISY, target.GISX),
	)
}

func (s *Service) buildStartupTestMessage(now, scheduledAt time.Time, target *state.CachedTarget, vehicle liveVehicleSnapshot) string {
	gpsLabel := "站點備援"
	if vehicle.GPSMode == gpsModeActual {
		gpsLabel = "即時 GPS"
	}

	return fmt.Sprintf(
		"🧪 啟動測試提醒\n路線：%s\n站點：%s（第 %d 站）\n預定到站：%s\n目前時間：%s\n垃圾車定位：%s\n垃圾車座標：%.6f, %.6f\n有謙家園座標：%.6f, %.6f\nGPS 查詢冷卻：%s\n%s\n地圖：%s",
		target.RouteName,
		target.PointName,
		target.PointSeq,
		scheduledAt.Format("2006-01-02 15:04"),
		now.Format("2006-01-02 15:04"),
		gpsLabel,
		vehicle.Lat,
		vehicle.Lng,
		target.GISY,
		target.GISX,
		s.cfg.GPSRefreshInterval,
		vehicle.StatusNote,
		buildDualMarkerMapURL(vehicle.Lat, vehicle.Lng, target.GISY, target.GISX),
	)
}

func (s *Service) cacheVehicleSnapshot(snapshot liveVehicleSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastVehicleSnapshot = snapshot
	s.lastVehicleFetchedAt = snapshot.FetchedAt
}

func (s *Service) isTargetDay(day time.Weekday) bool {
	for _, allowed := range s.cfg.TargetDays {
		if allowed == day {
			return true
		}
	}
	return false
}

func waitingTimeText(waitingTime int) string {
	switch waitingTime {
	case 0:
		return "進站中"
	case -1, -3:
		return "未發車"
	case -4:
		return "清運中"
	default:
		return fmt.Sprintf("%d 分鐘", waitingTime)
	}
}

func buildDeliveryKey(date string, offset int) string {
	return fmt.Sprintf("%s|%d", date, offset)
}

func combineDateAndClock(now time.Time, clock string) (time.Time, error) {
	parsed, err := time.ParseInLocation("15:04", clock, utils.GetTaiwanTimezone())
	if err != nil {
		return time.Time{}, fmt.Errorf("parse target time: %w", err)
	}

	return time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		parsed.Hour(),
		parsed.Minute(),
		0,
		0,
		utils.GetTaiwanTimezone(),
	), nil
}

func buildDualMarkerMapURL(vehicleLat, vehicleLng, targetLat, targetLng float64) string {
	return fmt.Sprintf(
		"https://www.google.com/maps/dir/?api=1&origin=%f,%f&destination=%f,%f&travelmode=driving",
		vehicleLat,
		vehicleLng,
		targetLat,
		targetLng,
	)
}
