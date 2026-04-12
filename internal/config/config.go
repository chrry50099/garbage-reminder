package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultEupfinBaseURL = "https://customer-tw.eupfin.com/Eup_Servlet_Nuser_SOAP/Eup_Servlet_Nuser_SOAP"

type Config struct {
	TelegramBotToken string
	TelegramChatID   string
	EupfinBaseURL    string
	StateFile        string
	CheckInterval    time.Duration
	GPSRefreshInterval time.Duration
	SendTestMessageOnStart bool

	TargetCustID    int
	TargetRouteID   int
	TargetPointSeq  int
	TargetPointName string
	TargetTime      string
	TargetDays      []time.Weekday

	ReminderOffsets []int
}

func Load() (*Config, error) {
	checkInterval, err := parseDurationEnv("CHECK_INTERVAL", time.Minute)
	if err != nil {
		return nil, err
	}

	gpsRefreshInterval, err := parseDurationEnv("GPS_REFRESH_INTERVAL", 5*time.Minute)
	if err != nil {
		return nil, err
	}

	targetDays, err := parseWeekdays(os.Getenv("TARGET_DAYS"))
	if err != nil {
		return nil, err
	}

	reminderOffsets, err := parseReminderOffsets(os.Getenv("REMINDER_MINUTES"))
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		EupfinBaseURL:    getEnvOrDefault("EUPFIN_BASE_URL", defaultEupfinBaseURL),
		StateFile:        getEnvOrDefault("STATE_FILE", "data/state.json"),
		CheckInterval:    checkInterval,
		GPSRefreshInterval: gpsRefreshInterval,
		SendTestMessageOnStart: parseBoolEnv("SEND_TEST_MESSAGE_ON_START", true),
		TargetPointName:  strings.TrimSpace(os.Getenv("TARGET_POINT_NAME")),
		TargetTime:       strings.TrimSpace(os.Getenv("TARGET_TIME")),
		TargetDays:       targetDays,
		ReminderOffsets:  reminderOffsets,
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
	if c.TargetTime == "" {
		missing = append(missing, "TARGET_TIME")
	}
	if len(c.TargetDays) == 0 {
		missing = append(missing, "TARGET_DAYS")
	}
	if len(c.ReminderOffsets) == 0 {
		missing = append(missing, "REMINDER_MINUTES")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if _, err := time.Parse("15:04", c.TargetTime); err != nil {
		return fmt.Errorf("TARGET_TIME must use HH:MM format: %w", err)
	}
	if c.GPSRefreshInterval <= 0 {
		return fmt.Errorf("GPS_REFRESH_INTERVAL must be greater than 0")
	}

	return nil
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

func parseReminderOffsets(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("REMINDER_MINUTES is required")
	}

	parts := strings.Split(raw, ",")
	offsets := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		value, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid REMINDER_MINUTES value %q: %w", trimmed, err)
		}
		if value <= 0 {
			return nil, fmt.Errorf("REMINDER_MINUTES values must be positive: %d", value)
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
