package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEupfinBaseURL   = "https://customer-tw.eupfin.com/Eup_Servlet_Nuser_SOAP/Eup_Servlet_Nuser_SOAP"
	defaultTargetDays      = "MON,TUE,WED,THU,FRI,SAT"
	defaultCollectionStart = "19:00"
	defaultCollectionEnd   = "21:30"
)

type Config struct {
	TelegramBotToken string
	TelegramChatID   string

	EupfinBaseURL          string
	StateFile              string
	DatabaseFile           string
	CheckInterval          time.Duration
	SendTestMessageOnStart bool
	HTTPPort               string

	TargetCustID    int
	TargetRouteID   int
	TargetPointSeq  int
	TargetPointName string
	TargetTime      string
	TargetDays      []time.Weekday

	CollectionStart string
	CollectionEnd   string
	AlertOffsets    []int
	HistoryWeeks    int

	ArrivalRadiusMeters           float64
	MatchRadiusMeters             float64
	MinHistoryRuns                int
	ProgressWindowMeters          float64
	LateralOffsetLimitMeters      float64
	BacktrackToleranceMeters      float64
	AmbiguousSegmentEpsilonMeters float64

	HABaseURL    string
	HAToken      string
	HANotifyMode string
	HATTSTarget  string
}

func Load() (*Config, error) {
	supervisorToken := strings.TrimSpace(os.Getenv("SUPERVISOR_TOKEN"))

	checkInterval, err := parseDurationEnv("CHECK_INTERVAL", time.Minute)
	if err != nil {
		return nil, err
	}

	targetDays, err := parseWeekdays(getEnvOrDefault("TARGET_DAYS", defaultTargetDays))
	if err != nil {
		return nil, err
	}

	alertOffsets, err := parseAlertOffsets(
		firstNonEmpty(os.Getenv("ALERT_OFFSETS"), os.Getenv("REMINDER_MINUTES")),
	)
	if err != nil {
		return nil, err
	}

	historyWeeks, err := parsePositiveIntEnv("HISTORY_WEEKS", 8)
	if err != nil {
		return nil, err
	}

	arrivalRadiusMeters, err := parsePositiveFloatEnv("ARRIVAL_RADIUS_METERS", 80)
	if err != nil {
		return nil, err
	}

	matchRadiusMeters, err := parsePositiveFloatEnv("MATCH_RADIUS_METERS", 250)
	if err != nil {
		return nil, err
	}

	minHistoryRuns, err := parsePositiveIntEnv("MIN_HISTORY_RUNS", 3)
	if err != nil {
		return nil, err
	}
	progressWindowMeters, err := parsePositiveFloatEnv("PROGRESS_WINDOW_METERS", 150)
	if err != nil {
		return nil, err
	}
	lateralOffsetLimitMeters, err := parsePositiveFloatEnv("LATERAL_OFFSET_LIMIT_METERS", 80)
	if err != nil {
		return nil, err
	}
	backtrackToleranceMeters, err := parsePositiveFloatEnv("BACKTRACK_TOLERANCE_METERS", 30)
	if err != nil {
		return nil, err
	}
	ambiguousSegmentEpsilonMeters, err := parsePositiveFloatEnv("AMBIGUOUS_SEGMENT_EPSILON_METERS", 15)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),

		EupfinBaseURL:          getEnvOrDefault("EUPFIN_BASE_URL", defaultEupfinBaseURL),
		StateFile:              getEnvOrDefault("STATE_FILE", "data/state.json"),
		DatabaseFile:           getEnvOrDefault("DATABASE_FILE", "data/history.db"),
		CheckInterval:          checkInterval,
		SendTestMessageOnStart: parseBoolEnv("SEND_TEST_MESSAGE_ON_START", false),
		HTTPPort:               getEnvOrDefault("PORT", "8080"),

		TargetPointName: strings.TrimSpace(os.Getenv("TARGET_POINT_NAME")),
		TargetTime:      strings.TrimSpace(os.Getenv("TARGET_TIME")),
		TargetDays:      targetDays,

		CollectionStart: getEnvOrDefault("COLLECTION_START", defaultCollectionStart),
		CollectionEnd:   getEnvOrDefault("COLLECTION_END", defaultCollectionEnd),
		AlertOffsets:    alertOffsets,
		HistoryWeeks:    historyWeeks,

		ArrivalRadiusMeters:           arrivalRadiusMeters,
		MatchRadiusMeters:             matchRadiusMeters,
		MinHistoryRuns:                minHistoryRuns,
		ProgressWindowMeters:          progressWindowMeters,
		LateralOffsetLimitMeters:      lateralOffsetLimitMeters,
		BacktrackToleranceMeters:      backtrackToleranceMeters,
		AmbiguousSegmentEpsilonMeters: ambiguousSegmentEpsilonMeters,

		HABaseURL:    resolveHABaseURL(supervisorToken),
		HAToken:      firstNonEmpty(os.Getenv("HA_TOKEN"), supervisorToken),
		HANotifyMode: strings.TrimSpace(os.Getenv("HA_NOTIFY_MODE")),
		HATTSTarget:  strings.TrimSpace(os.Getenv("HA_TTS_TARGET")),
	}

	cfg.TargetCustID, err = parseRequiredIntEnv("TARGET_CUST_ID")
	if err != nil {
		return nil, err
	}

	cfg.TargetRouteID, err = parseRequiredIntEnv("TARGET_ROUTE_ID")
	if err != nil {
		return nil, err
	}

	cfg.TargetPointSeq, err = parseRequiredIntEnv("TARGET_POINT_SEQ")
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var missing []string

	if c.TelegramBotToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if c.TelegramChatID == "" {
		missing = append(missing, "TELEGRAM_CHAT_ID")
	}
	if c.TargetPointName == "" {
		missing = append(missing, "TARGET_POINT_NAME")
	}
	if len(c.TargetDays) == 0 {
		missing = append(missing, "TARGET_DAYS")
	}
	if len(c.AlertOffsets) == 0 {
		missing = append(missing, "ALERT_OFFSETS")
	}
	if c.HABaseURL == "" {
		missing = append(missing, "HA_BASE_URL")
	}
	if c.HAToken == "" {
		missing = append(missing, "HA_TOKEN")
	}
	if c.HANotifyMode == "" {
		missing = append(missing, "HA_NOTIFY_MODE")
	}
	if c.HATTSTarget == "" {
		missing = append(missing, "HA_TTS_TARGET")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if c.TargetTime != "" {
		if _, err := time.Parse("15:04", c.TargetTime); err != nil {
			return fmt.Errorf("TARGET_TIME must use HH:MM format: %w", err)
		}
	}

	if _, err := parseClock(c.CollectionStart); err != nil {
		return fmt.Errorf("COLLECTION_START must use HH:MM format: %w", err)
	}
	if _, err := parseClock(c.CollectionEnd); err != nil {
		return fmt.Errorf("COLLECTION_END must use HH:MM format: %w", err)
	}
	start, _ := parseClock(c.CollectionStart)
	end, _ := parseClock(c.CollectionEnd)
	if start >= end {
		return fmt.Errorf("COLLECTION_START must be before COLLECTION_END")
	}

	switch c.HANotifyMode {
	case "webhook", "service_call":
	default:
		return fmt.Errorf("HA_NOTIFY_MODE must be one of webhook or service_call")
	}

	if c.CheckInterval <= 0 {
		return fmt.Errorf("CHECK_INTERVAL must be greater than 0")
	}
	if c.HistoryWeeks <= 0 {
		return fmt.Errorf("HISTORY_WEEKS must be greater than 0")
	}
	if c.ArrivalRadiusMeters <= 0 {
		return fmt.Errorf("ARRIVAL_RADIUS_METERS must be greater than 0")
	}
	if c.MatchRadiusMeters <= 0 {
		return fmt.Errorf("MATCH_RADIUS_METERS must be greater than 0")
	}
	if c.MinHistoryRuns <= 0 {
		return fmt.Errorf("MIN_HISTORY_RUNS must be greater than 0")
	}
	if c.ProgressWindowMeters <= 0 {
		return fmt.Errorf("PROGRESS_WINDOW_METERS must be greater than 0")
	}
	if c.LateralOffsetLimitMeters <= 0 {
		return fmt.Errorf("LATERAL_OFFSET_LIMIT_METERS must be greater than 0")
	}
	if c.BacktrackToleranceMeters <= 0 {
		return fmt.Errorf("BACKTRACK_TOLERANCE_METERS must be greater than 0")
	}
	if c.AmbiguousSegmentEpsilonMeters <= 0 {
		return fmt.Errorf("AMBIGUOUS_SEGMENT_EPSILON_METERS must be greater than 0")
	}

	return nil
}

func (c *Config) CollectionWindowMinutes() (int, int, error) {
	start, err := parseClock(c.CollectionStart)
	if err != nil {
		return 0, 0, err
	}
	end, err := parseClock(c.CollectionEnd)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func resolveHABaseURL(supervisorToken string) string {
	if value := strings.TrimSpace(os.Getenv("HA_BASE_URL")); value != "" {
		return value
	}
	if strings.TrimSpace(supervisorToken) != "" {
		return "http://supervisor/core"
	}
	return ""
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}

func parseRequiredIntEnv(key string) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	return parsed, nil
}

func parsePositiveIntEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}
	return parsed, nil
}

func parsePositiveFloatEnv(key string, fallback float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}
	return parsed, nil
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}

	return duration, nil
}

func parseBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func parseAlertOffsets(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("ALERT_OFFSETS is required")
	}

	parts := strings.Split(raw, ",")
	offsets := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		value, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid ALERT_OFFSETS value %q: %w", trimmed, err)
		}
		if value <= 0 {
			return nil, fmt.Errorf("ALERT_OFFSETS values must be positive: %d", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		offsets = append(offsets, value)
	}

	return offsets, nil
}

func parseWeekdays(raw string) ([]time.Weekday, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("TARGET_DAYS is required")
	}

	lookup := map[string]time.Weekday{
		"SUN": time.Sunday,
		"MON": time.Monday,
		"TUE": time.Tuesday,
		"WED": time.Wednesday,
		"THU": time.Thursday,
		"FRI": time.Friday,
		"SAT": time.Saturday,
	}

	parts := strings.Split(raw, ",")
	days := make([]time.Weekday, 0, len(parts))
	seen := make(map[time.Weekday]struct{}, len(parts))

	for _, part := range parts {
		key := strings.ToUpper(strings.TrimSpace(part))
		day, ok := lookup[key]
		if !ok {
			return nil, fmt.Errorf("unsupported TARGET_DAYS value: %s", part)
		}
		if _, exists := seen[day]; exists {
			continue
		}
		seen[day] = struct{}{}
		days = append(days, day)
	}

	return days, nil
}

func parseClock(raw string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}
