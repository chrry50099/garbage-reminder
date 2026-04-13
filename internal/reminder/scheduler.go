package reminder

import (
	"context"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"telegram-garbage-reminder/internal/config"
	"telegram-garbage-reminder/internal/eupfin"
	"telegram-garbage-reminder/internal/geo"
	"telegram-garbage-reminder/internal/history"
	"telegram-garbage-reminder/internal/state"
	"telegram-garbage-reminder/internal/utils"
)

const deliveryStatusSent = "sent"

type eupfinClient interface {
	GetDistrictByCustID(ctx context.Context, custID int) (*eupfin.DistrictConfig, error)
	GetAllRouteBasicData(ctx context.Context, custID int) ([]eupfin.Route, error)
	ResolveTargetStop(ctx context.Context, custID, routeID, pointSeq int, pointName string) (*eupfin.TargetStop, error)
	GetCarStatusGarbage(ctx context.Context, custID, teamID int) ([]eupfin.CarStatus, error)
	GetAllRouteStatusData(ctx context.Context, custID int) ([]eupfin.RouteStatus, error)
}

type messageSender interface {
	SendMessage(ctx context.Context, text string) error
}

type localStore interface {
	SaveCachedTarget(target state.CachedTarget) error
	GetCachedTarget() *state.CachedTarget
	HasDelivery(deliveryKey string) bool
	RecordDelivery(deliveryKey string, record state.DeliveryRecord) error
	ListDeliveriesForDate(serviceDate string) []state.DeliveryRecord
}

type historyStore interface {
	EnsureRun(serviceDate string, weekday time.Weekday, routeID, pointID int, targetLat, targetLng float64, startedAt time.Time) (*history.Run, error)
	GetRun(runID string) (*history.Run, error)
	InsertSample(sample history.Sample) error
	MarkRunCompleted(runID string, arrivalAt time.Time, arrivalSource string) error
	PruneBefore(cutoff time.Time) error
	ListHistoricalSamples(weekday time.Weekday, routeID, pointID int, since time.Time) ([]history.HistoricalSample, int, error)
	ListRecentRunSamples(runID string, limit int) ([]history.RecentSample, error)
	ListServiceDates(limit int) ([]string, error)
	ListSamplesByServiceDate(serviceDate string) ([]history.Sample, error)
}

type Service struct {
	cfg             *config.Config
	client          eupfinClient
	alertNotifier   messageSender
	startupNotifier messageSender
	store           localStore
	history         historyStore
	collectorLog    *CollectorLogger
	now             func() time.Time

	statusMu sync.RWMutex
	status   StatusSnapshot

	routeShapeMu sync.RWMutex
	routeShape   *history.RouteShape
}

type StatusSnapshot struct {
	UpdatedAt        time.Time           `json:"updated_at"`
	Active           bool                `json:"active"`
	ServiceDate      string              `json:"service_date,omitempty"`
	Weekday          string              `json:"weekday,omitempty"`
	CollectionWindow string              `json:"collection_window"`
	RunStatus        string              `json:"run_status,omitempty"`
	RouteName        string              `json:"route_name,omitempty"`
	PointName        string              `json:"point_name,omitempty"`
	PointSeq         int                 `json:"point_seq,omitempty"`
	GPSAvailable     bool                `json:"gps_available"`
	TruckLat         float64             `json:"truck_lat,omitempty"`
	TruckLng         float64             `json:"truck_lng,omitempty"`
	TargetLat        float64             `json:"target_lat,omitempty"`
	TargetLng        float64             `json:"target_lng,omitempty"`
	APIEstimatedText string              `json:"api_estimated_text,omitempty"`
	APIWaitingTime   *int                `json:"api_waiting_time,omitempty"`
	Prediction       *history.Prediction `json:"prediction,omitempty"`
	NotifiedOffsets  []int               `json:"notified_offsets,omitempty"`
	TodaySampleCount int                 `json:"today_sample_count"`
	TodayGPSSamples  int                 `json:"today_gps_sample_count"`
	SharedDataPath   string              `json:"shared_data_path,omitempty"`
	CheckInterval    string              `json:"check_interval,omitempty"`
	Message          string              `json:"message,omitempty"`
	LastCollectedAt  *time.Time          `json:"last_collected_at,omitempty"`
}

type liveObservation struct {
	observation      history.Observation
	apiEstimatedText string
	apiWaitingTime   *int
	arrivalSource    string
}

func NewService(
	cfg *config.Config,
	client eupfinClient,
	alertNotifier messageSender,
	startupNotifier messageSender,
	store localStore,
	historyStore historyStore,
	collectorLog *CollectorLogger,
) *Service {
	offsets := append([]int(nil), cfg.AlertOffsets...)
	slices.Sort(offsets)
	slices.Reverse(offsets)
	cfg.AlertOffsets = offsets

	return &Service{
		cfg:             cfg,
		client:          client,
		alertNotifier:   alertNotifier,
		startupNotifier: startupNotifier,
		store:           store,
		history:         historyStore,
		collectorLog:    collectorLog,
		now:             utils.NowInTaiwan,
		status: StatusSnapshot{
			CollectionWindow: fmt.Sprintf("%s-%s", cfg.CollectionStart, cfg.CollectionEnd),
			SharedDataPath:   cfg.SharedDataDir,
			CheckInterval:    cfg.CheckInterval.String(),
			Message:          "service starting",
		},
	}
}

func (s *Service) Initialize(ctx context.Context) error {
	target, err := s.refreshTarget(ctx)
	if err != nil {
		return err
	}

	if err := s.history.PruneBefore(s.now().AddDate(0, 0, -7*s.cfg.HistoryWeeks)); err != nil {
		return fmt.Errorf("prune old history: %w", err)
	}

	log.Printf("Validated target stop: route=%s route_id=%d seq=%d point=%s",
		target.RouteName, target.RouteID, target.PointSeq, target.PointName)
	return nil
}

func (s *Service) SendStartupTestMessage(ctx context.Context) error {
	if s.startupNotifier == nil {
		return nil
	}

	target, err := s.resolveTarget(ctx)
	if err != nil {
		return err
	}

	now := s.now().In(utils.GetTaiwanTimezone())
	message := fmt.Sprintf(
		"🧪 垃圾車服務啟動\n路線：%s\n站點：%s（第 %d 站）\n收集時窗：%s-%s\n提醒門檻：%v 分鐘\n歷史保留：%d 週\nHA 模式：%s\n狀態端點：/status",
		target.RouteName,
		target.PointName,
		target.PointSeq,
		s.cfg.CollectionStart,
		s.cfg.CollectionEnd,
		s.cfg.AlertOffsets,
		s.cfg.HistoryWeeks,
		s.cfg.HANotifyMode,
	)
	if err := s.startupNotifier.SendMessage(ctx, message); err != nil {
		return fmt.Errorf("send startup message: %w", err)
	}

	s.updateStatus(StatusSnapshot{
		UpdatedAt:        now,
		CollectionWindow: fmt.Sprintf("%s-%s", s.cfg.CollectionStart, s.cfg.CollectionEnd),
		RouteName:        target.RouteName,
		PointName:        target.PointName,
		PointSeq:         target.PointSeq,
		Message:          "startup test sent",
	})
	return nil
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()

	log.Printf("Reminder collector started, interval=%s", s.cfg.CheckInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Reminder collector stopped")
			return
		case <-ticker.C:
			if err := s.CheckOnce(ctx); err != nil {
				log.Printf("Collection tick failed: %v", err)
			}
		}
	}
}

func (s *Service) CheckOnce(ctx context.Context) error {
	now := s.now().In(utils.GetTaiwanTimezone())
	serviceDate := now.Format("2006-01-02")

	if err := s.history.PruneBefore(now.AddDate(0, 0, -7*s.cfg.HistoryWeeks)); err != nil {
		s.logCollectorError("prune_history", serviceDate, now, err)
		return err
	}

	if !s.isTargetDay(now.Weekday()) {
		s.updateStatus(s.updateStatusWithDayCounts(StatusSnapshot{
			UpdatedAt:        now,
			Active:           false,
			ServiceDate:      serviceDate,
			Weekday:          now.Weekday().String(),
			CollectionWindow: fmt.Sprintf("%s-%s", s.cfg.CollectionStart, s.cfg.CollectionEnd),
			Message:          "inactive weekday",
		}, serviceDate))
		return nil
	}

	if !s.isWithinCollectionWindow(now) {
		s.updateStatus(s.updateStatusWithDayCounts(StatusSnapshot{
			UpdatedAt:        now,
			Active:           false,
			ServiceDate:      serviceDate,
			Weekday:          now.Weekday().String(),
			CollectionWindow: fmt.Sprintf("%s-%s", s.cfg.CollectionStart, s.cfg.CollectionEnd),
			Message:          "outside collection window",
		}, serviceDate))
		return nil
	}

	target, err := s.resolveTarget(ctx)
	if err != nil {
		s.logCollectorError("resolve_target", serviceDate, now, err)
		return err
	}

	run, err := s.history.EnsureRun(serviceDate, now.Weekday(), target.RouteID, target.PointID, target.GISY, target.GISX, now)
	if err != nil {
		s.logCollectorError("ensure_run", serviceDate, now, err)
		return fmt.Errorf("ensure history run: %w", err)
	}
	if run != nil && run.Status == "completed" {
		s.updateStatus(s.updateStatusWithDayCounts(StatusSnapshot{
			UpdatedAt:        now,
			Active:           false,
			ServiceDate:      serviceDate,
			Weekday:          now.Weekday().String(),
			CollectionWindow: fmt.Sprintf("%s-%s", s.cfg.CollectionStart, s.cfg.CollectionEnd),
			RunStatus:        run.Status,
			RouteName:        target.RouteName,
			PointName:        target.PointName,
			PointSeq:         target.PointSeq,
			TargetLat:        target.GISY,
			TargetLng:        target.GISX,
			NotifiedOffsets:  s.notifiedOffsets(serviceDate),
			Message:          "run already completed",
		}, serviceDate))
		return nil
	}

	live, err := s.collectLiveObservation(ctx, target, now)
	if err != nil {
		s.logCollectorError("collect_live_observation", serviceDate, now, err)
		return err
	}

	sample := history.Sample{
		RunID:               serviceDate,
		ServiceDate:         serviceDate,
		Weekday:             now.Weekday(),
		RouteID:             target.RouteID,
		PointID:             target.PointID,
		TruckLat:            live.observation.TruckLat,
		TruckLng:            live.observation.TruckLng,
		GPSAvailable:        live.observation.GPSAvailable,
		APIEstimatedMinutes: live.observation.APIEstimatedMinutes,
		APIEstimatedText:    live.apiEstimatedText,
		APIWaitingTime:      live.apiWaitingTime,
		ProgressMeters:      live.observation.ProgressMeters,
		SegmentIndex:        live.observation.SegmentIndex,
		LateralOffsetMeters: live.observation.LateralOffsetMeters,
		CollectedAt:         now,
	}
	if err := s.history.InsertSample(sample); err != nil {
		s.logCollectorError("insert_sample", serviceDate, now, err)
		return fmt.Errorf("insert historical sample: %w", err)
	}

	runStatus := "collecting"
	if live.arrivalSource != "" {
		if err := s.history.MarkRunCompleted(serviceDate, now, live.arrivalSource); err != nil {
			s.logCollectorError("mark_run_completed", serviceDate, now, err)
			return fmt.Errorf("mark run completed: %w", err)
		}
		runStatus = "completed"
	}

	historicalSamples, historicalRuns, err := s.history.ListHistoricalSamples(
		now.Weekday(),
		target.RouteID,
		target.PointID,
		now.AddDate(0, 0, -7*s.cfg.HistoryWeeks),
	)
	if err != nil {
		s.logCollectorError("list_historical_samples", serviceDate, now, err)
		return fmt.Errorf("load historical samples: %w", err)
	}

	prediction := history.PredictFromHistory(live.observation, historicalSamples, historicalRuns, history.PredictorConfig{
		ProgressWindowMeters:     s.cfg.ProgressWindowMeters,
		LateralOffsetLimitMeters: s.cfg.LateralOffsetLimitMeters,
		MinHistoryRuns:           s.cfg.MinHistoryRuns,
		TargetProgressMeters:     s.currentTargetProgress(),
	})
	if prediction == nil {
		prediction = history.PredictFromFallback(live.observation)
	}

	if prediction != nil {
		if err := s.dispatchNotifications(ctx, serviceDate, now, target, live, prediction); err != nil {
			s.logCollectorError("dispatch_notifications", serviceDate, now, err)
			return err
		}
	}

	s.logCollectorSample(serviceDate, now, sample, prediction, runStatus)

	lastCollectedAt := now
	snapshot := StatusSnapshot{
		UpdatedAt:        now,
		Active:           true,
		ServiceDate:      serviceDate,
		Weekday:          now.Weekday().String(),
		CollectionWindow: fmt.Sprintf("%s-%s", s.cfg.CollectionStart, s.cfg.CollectionEnd),
		RunStatus:        runStatus,
		RouteName:        target.RouteName,
		PointName:        target.PointName,
		PointSeq:         target.PointSeq,
		GPSAvailable:     live.observation.GPSAvailable,
		TruckLat:         live.observation.TruckLat,
		TruckLng:         live.observation.TruckLng,
		TargetLat:        target.GISY,
		TargetLng:        target.GISX,
		APIEstimatedText: live.apiEstimatedText,
		APIWaitingTime:   live.apiWaitingTime,
		Prediction:       prediction,
		NotifiedOffsets:  s.notifiedOffsets(serviceDate),
		LastCollectedAt:  &lastCollectedAt,
		Message:          s.statusMessage(runStatus, prediction),
	}
	s.updateStatus(s.updateStatusWithDayCounts(snapshot, serviceDate))

	return nil
}

func (s *Service) CurrentStatus() StatusSnapshot {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()

	snapshot := s.status
	if snapshot.Prediction != nil {
		copyPrediction := *snapshot.Prediction
		snapshot.Prediction = &copyPrediction
	}
	if snapshot.APIWaitingTime != nil {
		waiting := *snapshot.APIWaitingTime
		snapshot.APIWaitingTime = &waiting
	}
	if snapshot.LastCollectedAt != nil {
		collectedAt := *snapshot.LastCollectedAt
		snapshot.LastCollectedAt = &collectedAt
	}
	snapshot.NotifiedOffsets = append([]int(nil), snapshot.NotifiedOffsets...)
	return snapshot
}

func (s *Service) refreshTarget(ctx context.Context) (*state.CachedTarget, error) {
	district, err := s.client.GetDistrictByCustID(ctx, s.cfg.TargetCustID)
	if err != nil {
		return nil, fmt.Errorf("resolve district config: %w", err)
	}

	var target *eupfin.TargetStop
	routes, routesErr := s.client.GetAllRouteBasicData(ctx, s.cfg.TargetCustID)
	if routesErr == nil {
		resolved, err := eupfin.FindTargetStop(routes, s.cfg.TargetRouteID, s.cfg.TargetPointSeq, s.cfg.TargetPointName)
		if err != nil {
			return nil, fmt.Errorf("resolve target stop from route basic data: %w", err)
		}
		target = resolved
		if route := findRoute(routes, s.cfg.TargetRouteID); route != nil {
			if shape, err := history.BuildRouteShape(*route, s.cfg.TargetPointSeq); err != nil {
				log.Printf("Route shape unavailable; historical projection disabled for now: %v", err)
			} else {
				s.setRouteShape(shape)
			}
		}
	} else {
		log.Printf("Route basic data unavailable; keeping previous route shape: %v", routesErr)
		resolved, err := s.client.ResolveTargetStop(
			ctx,
			s.cfg.TargetCustID,
			s.cfg.TargetRouteID,
			s.cfg.TargetPointSeq,
			s.cfg.TargetPointName,
		)
		if err != nil {
			return nil, fmt.Errorf("resolve target stop: %w", err)
		}
		target = resolved
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

func (s *Service) collectLiveObservation(ctx context.Context, target *state.CachedTarget, now time.Time) (*liveObservation, error) {
	result := &liveObservation{
		observation: history.Observation{
			CollectedAt: now,
			Weekday:     now.Weekday(),
			RouteID:     target.RouteID,
			PointID:     target.PointID,
		},
	}

	cars, carsErr := s.client.GetCarStatusGarbage(ctx, target.CustID, target.TeamID)
	if carsErr == nil {
		for _, car := range cars {
			if car.RouteID == target.RouteID && car.GISY != 0 && car.GISX != 0 {
				result.observation.GPSAvailable = true
				result.observation.TruckLat = car.GISY
				result.observation.TruckLng = car.GISX
				break
			}
		}
	}

	statuses, statusesErr := s.client.GetAllRouteStatusData(ctx, target.CustID)
	if statusesErr == nil {
		if pointStatus := findTargetPointStatus(statuses, target.RouteID, target.PointID); pointStatus != nil {
			result.apiEstimatedText = strings.TrimSpace(pointStatus.EstimatedTime)
			result.observation.APIEstimatedText = result.apiEstimatedText
			if estimated := parseEstimatedMinutes(now, pointStatus.EstimatedTime); estimated != nil {
				result.observation.APIEstimatedMinutes = estimated
			}

			waiting := pointStatus.WaitingTime
			result.apiWaitingTime = &waiting
			result.observation.APIWaitingTime = &waiting
		}
	}

	if carsErr != nil && statusesErr != nil {
		return nil, fmt.Errorf("fetch live route data failed: gps=%v; status=%v", carsErr, statusesErr)
	}

	s.projectObservation(&result.observation)

	result.arrivalSource = detectArrival(result.observation, target, s.cfg.ArrivalRadiusMeters)
	return result, nil
}

func (s *Service) projectObservation(observation *history.Observation) {
	if observation == nil || !observation.GPSAvailable {
		return
	}

	shape := s.currentRouteShape()
	if shape == nil {
		return
	}

	recent, err := s.history.ListRecentRunSamples(observation.CollectedAt.Format("2006-01-02"), 2)
	if err != nil {
		log.Printf("ListRecentRunSamples failed: %v", err)
		return
	}

	projection, ok := shape.Project(observation.TruckLat, observation.TruckLng, recent, history.ProjectionConfig{
		ProgressWindowMeters:          s.cfg.ProgressWindowMeters,
		LateralOffsetLimitMeters:      s.cfg.LateralOffsetLimitMeters,
		BacktrackToleranceMeters:      s.cfg.BacktrackToleranceMeters,
		AmbiguousSegmentEpsilonMeters: s.cfg.AmbiguousSegmentEpsilonMeters,
	})
	if !ok {
		return
	}

	progress := projection.ProgressMeters
	segment := projection.SegmentIndex
	lateralOffset := projection.LateralOffsetMeters
	observation.ProgressMeters = &progress
	observation.SegmentIndex = &segment
	observation.LateralOffsetMeters = &lateralOffset
}

func (s *Service) dispatchNotifications(
	ctx context.Context,
	serviceDate string,
	now time.Time,
	target *state.CachedTarget,
	live *liveObservation,
	prediction *history.Prediction,
) error {
	for _, offset := range s.cfg.AlertOffsets {
		if prediction.RemainingMinutes > offset {
			continue
		}

		deliveryKey := buildDeliveryKey(serviceDate, offset)
		if s.store.HasDelivery(deliveryKey) {
			continue
		}

		message := s.buildAlertMessage(now, offset, target, live, prediction)
		if err := s.alertNotifier.SendMessage(ctx, message); err != nil {
			return fmt.Errorf("send alert notification: %w", err)
		}

		record := state.DeliveryRecord{
			ScheduledDate:         serviceDate,
			ReminderOffsetMinutes: offset,
			DeliveryStatus:        deliveryStatusSent,
			GPSMode:               sourceGPSMode(live.observation.GPSAvailable),
			PredictionSource:      prediction.Source,
			Confidence:            prediction.Confidence,
			SentAt:                now,
		}

		if err := s.store.RecordDelivery(deliveryKey, record); err != nil {
			return fmt.Errorf("record delivery state: %w", err)
		}

		log.Printf("Alert sent: offset=%d source=%s confidence=%s", offset, prediction.Source, prediction.Confidence)
	}

	return nil
}

func (s *Service) buildAlertMessage(
	now time.Time,
	offset int,
	target *state.CachedTarget,
	live *liveObservation,
	prediction *history.Prediction,
) string {
	gpsLabel := "不可用"
	originLat := target.GISY
	originLng := target.GISX
	if live.observation.GPSAvailable {
		gpsLabel = "可用"
		originLat = live.observation.TruckLat
		originLng = live.observation.TruckLng
	}

	waitingText := "未知"
	if live.apiWaitingTime != nil {
		waitingText = waitingTimeText(*live.apiWaitingTime)
	}

	return fmt.Sprintf(
		"🗑️ 垃圾車提醒（%d 分鐘門檻）\n路線：%s\n站點：%s（第 %d 站）\n目前時間：%s\n預測到站：%s\n剩餘時間：%d 分鐘\n資料來源：%s\n信心：%s\nGPS：%s\n垃圾車座標：%.6f, %.6f\n有謙家園座標：%.6f, %.6f\nAPI EstimatedTime：%s\nAPI WaitingTime：%s\n地圖：%s",
		offset,
		target.RouteName,
		target.PointName,
		target.PointSeq,
		now.Format("2006-01-02 15:04"),
		prediction.PredictedArrivalAt.Format("2006-01-02 15:04"),
		prediction.RemainingMinutes,
		prediction.Source,
		prediction.Confidence,
		gpsLabel,
		originLat,
		originLng,
		target.GISY,
		target.GISX,
		firstNonEmpty(live.apiEstimatedText, "未知"),
		waitingText,
		buildDualMarkerMapURL(originLat, originLng, target.GISY, target.GISX),
	)
}

func (s *Service) isTargetDay(day time.Weekday) bool {
	for _, allowed := range s.cfg.TargetDays {
		if allowed == day {
			return true
		}
	}
	return false
}

func (s *Service) isWithinCollectionWindow(now time.Time) bool {
	start, end, err := s.cfg.CollectionWindowMinutes()
	if err != nil {
		return false
	}
	current := now.Hour()*60 + now.Minute()
	return current >= start && current <= end
}

func (s *Service) statusMessage(runStatus string, prediction *history.Prediction) string {
	if prediction == nil {
		if runStatus == "completed" {
			return "run completed without prediction"
		}
		return "collecting samples; waiting for prediction"
	}
	return fmt.Sprintf("prediction ready via %s", prediction.Source)
}

func (s *Service) notifiedOffsets(serviceDate string) []int {
	offsets := make([]int, 0, len(s.cfg.AlertOffsets))
	for _, offset := range s.cfg.AlertOffsets {
		if s.store.HasDelivery(buildDeliveryKey(serviceDate, offset)) {
			offsets = append(offsets, offset)
		}
	}
	return offsets
}

func (s *Service) updateStatus(snapshot StatusSnapshot) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if snapshot.CollectionWindow == "" {
		snapshot.CollectionWindow = fmt.Sprintf("%s-%s", s.cfg.CollectionStart, s.cfg.CollectionEnd)
	}
	if snapshot.SharedDataPath == "" {
		snapshot.SharedDataPath = s.cfg.SharedDataDir
	}
	if snapshot.CheckInterval == "" {
		snapshot.CheckInterval = s.cfg.CheckInterval.String()
	}
	s.status = snapshot
}

func findTargetPointStatus(statuses []eupfin.RouteStatus, routeID, pointID int) *eupfin.RouteStatusPoint {
	for _, routeStatus := range statuses {
		if routeStatus.RouteID != routeID {
			continue
		}
		for _, pointStatus := range routeStatus.Points {
			if pointStatus.PointID == pointID {
				copy := pointStatus
				return &copy
			}
		}
	}
	return nil
}

func detectArrival(observation history.Observation, target *state.CachedTarget, arrivalRadiusMeters float64) string {
	if observation.GPSAvailable {
		if geo.CalculateDistance(observation.TruckLat, observation.TruckLng, target.GISY, target.GISX) <= arrivalRadiusMeters {
			return "gps_radius"
		}
	}
	if observation.APIWaitingTime != nil && *observation.APIWaitingTime == 0 {
		return "api_waiting_time"
	}
	if observation.APIEstimatedMinutes != nil && *observation.APIEstimatedMinutes <= 1 {
		return "api_estimated_time"
	}
	return ""
}

func parseEstimatedMinutes(now time.Time, raw string) *int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}

	if strings.Contains(value, ":") {
		parsed, err := time.ParseInLocation("15:04", value, utils.GetTaiwanTimezone())
		if err != nil {
			return nil
		}
		estimatedAt := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, utils.GetTaiwanTimezone())
		remaining := int(math.Ceil(estimatedAt.Sub(now).Minutes()))
		if remaining < 0 {
			remaining = 0
		}
		return &remaining
	}

	var minutes int
	if _, err := fmt.Sscanf(value, "%d", &minutes); err == nil && minutes >= 0 {
		return &minutes
	}

	return nil
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

func sourceGPSMode(gpsAvailable bool) string {
	if gpsAvailable {
		return "actual"
	}
	return "fallback_station"
}

func buildDeliveryKey(date string, offset int) string {
	return fmt.Sprintf("%s|%d", date, offset)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func findRoute(routes []eupfin.Route, routeID int) *eupfin.Route {
	for _, route := range routes {
		if route.RouteID == routeID {
			copy := route
			return &copy
		}
	}
	return nil
}

func (s *Service) setRouteShape(shape *history.RouteShape) {
	s.routeShapeMu.Lock()
	defer s.routeShapeMu.Unlock()
	s.routeShape = shape
}

func (s *Service) currentRouteShape() *history.RouteShape {
	s.routeShapeMu.RLock()
	defer s.routeShapeMu.RUnlock()
	return s.routeShape
}

func (s *Service) currentTargetProgress() *float64 {
	shape := s.currentRouteShape()
	if shape == nil {
		return nil
	}
	progress := shape.TargetProgressMeters
	return &progress
}
