package history

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type DayData struct {
	ServiceDate      string      `json:"service_date"`
	RouteName        string      `json:"route_name,omitempty"`
	PointName        string      `json:"point_name,omitempty"`
	PointSeq         int         `json:"point_seq,omitempty"`
	TargetLat        float64     `json:"target_lat,omitempty"`
	TargetLng        float64     `json:"target_lng,omitempty"`
	CollectionWindow string      `json:"collection_window,omitempty"`
	RunStatus        string      `json:"run_status,omitempty"`
	ArrivalAt        *time.Time  `json:"arrival_at,omitempty"`
	NotifiedOffsets  []int       `json:"notified_offsets,omitempty"`
	SampleCount      int         `json:"sample_count"`
	GPSSampleCount   int         `json:"gps_sample_count"`
	FirstCollectedAt *time.Time  `json:"first_collected_at,omitempty"`
	LastCollectedAt  *time.Time  `json:"last_collected_at,omitempty"`
	SharedDataPath   string      `json:"shared_data_path,omitempty"`
	CheckInterval    string      `json:"check_interval,omitempty"`
	JSONExportPath   string      `json:"json_export_path,omitempty"`
	CSVExportPath    string      `json:"csv_export_path,omitempty"`
	Samples          []DaySample `json:"samples"`
}

type DaySample struct {
	CollectedAt         time.Time `json:"collected_at"`
	GPSAvailable        bool      `json:"gps_available"`
	TruckLat            float64   `json:"truck_lat,omitempty"`
	TruckLng            float64   `json:"truck_lng,omitempty"`
	APIEstimatedMinutes *int      `json:"api_estimated_minutes,omitempty"`
	APIEstimatedText    string    `json:"api_estimated_text,omitempty"`
	APIWaitingTime      *int      `json:"api_waiting_time,omitempty"`
	ProgressMeters      *float64  `json:"progress_meters,omitempty"`
	SegmentIndex        *int      `json:"segment_index,omitempty"`
	LateralOffsetMeters *float64  `json:"lateral_offset_meters,omitempty"`
}

func (d *DayData) Finalize() {
	if d == nil {
		return
	}

	d.SampleCount = len(d.Samples)
	d.GPSSampleCount = 0
	d.NotifiedOffsets = uniqueSortedInts(d.NotifiedOffsets)

	for index, sample := range d.Samples {
		if sample.GPSAvailable {
			d.GPSSampleCount++
		}
		collectedAt := sample.CollectedAt
		if index == 0 || (d.FirstCollectedAt != nil && collectedAt.Before(*d.FirstCollectedAt)) {
			value := collectedAt
			d.FirstCollectedAt = &value
		}
		if index == 0 || (d.LastCollectedAt != nil && collectedAt.After(*d.LastCollectedAt)) {
			value := collectedAt
			d.LastCollectedAt = &value
		}
	}
}

func (d DayData) MarshalJSONBytes() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

func (d DayData) MarshalCSV() ([]byte, error) {
	records := [][]string{
		{
			"service_date",
			"collected_at",
			"gps_available",
			"truck_lat",
			"truck_lng",
			"api_estimated_minutes",
			"api_estimated_text",
			"api_waiting_time",
			"progress_meters",
			"segment_index",
			"lateral_offset_meters",
		},
	}
	for _, sample := range d.Samples {
		records = append(records, []string{
			d.ServiceDate,
			sample.CollectedAt.Format(time.RFC3339),
			strconv.FormatBool(sample.GPSAvailable),
			formatFloat(sample.TruckLat),
			formatFloat(sample.TruckLng),
			formatNullableInt(sample.APIEstimatedMinutes),
			sample.APIEstimatedText,
			formatNullableInt(sample.APIWaitingTime),
			formatNullableFloat(sample.ProgressMeters),
			formatNullableInt(sample.SegmentIndex),
			formatNullableFloat(sample.LateralOffsetMeters),
		})
	}

	buffer := csvBuffer{}
	writer := csv.NewWriter(&buffer)
	if err := writer.WriteAll(records); err != nil {
		return nil, fmt.Errorf("write csv: %w", err)
	}
	return buffer.Bytes(), nil
}

func BuildExportPaths(exportsDir, serviceDate string) (string, string) {
	return filepath.Join(exportsDir, serviceDate+".json"), filepath.Join(exportsDir, serviceDate+".csv")
}

type csvBuffer struct {
	data []byte
}

func (b *csvBuffer) Write(value []byte) (int, error) {
	b.data = append(b.data, value...)
	return len(value), nil
}

func (b *csvBuffer) Bytes() []byte {
	return append([]byte(nil), b.data...)
}

func uniqueSortedInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(values))
	unique := make([]int, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Ints(unique)
	return unique
}

func formatNullableInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func formatNullableFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', 3, 64)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}
