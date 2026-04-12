package eupfin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type responseEnvelope struct {
	Status int             `json:"status"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

type CountryConfig struct {
	Country   string           `json:"Country"`
	CountryID int              `json:"CountryCode"`
	Districts []DistrictConfig `json:"District"`
}

type DistrictConfig struct {
	FromGovData         bool   `json:"FromGovData"`
	Address             string `json:"Address"`
	IsEnabled           bool   `json:"IsEnabled"`
	GisX                int    `json:"GisX"`
	GisY                int    `json:"GisY"`
	TeamID              int    `json:"Team_ID"`
	CustID              int    `json:"Cust_ID"`
	Phone               string `json:"Phone"`
	IsSetReminder       bool   `json:"IsSetReminder"`
	CustIMID            string `json:"CustIMID"`
	RemovalType         int    `json:"RemovalType"`
	IsShowEstimatedTime bool   `json:"IsShowEstimatedTime"`
	District            string `json:"District"`
}

type Route struct {
	RouteID    int     `json:"Route_ID"`
	RouteName  string  `json:"RouteName"`
	StartTime  string  `json:"StartTime"`
	EndTime    string  `json:"EndTime"`
	CarNumber  string  `json:"Car_Number"`
	CarUnicode string  `json:"CarUnicode"`
	Points     []Point `json:"Points"`
}

type Point struct {
	PointID   int           `json:"Point_ID"`
	Seq       int           `json:"Seq"`
	PointName string        `json:"PointName"`
	GISX      float64       `json:"GISX"`
	GISY      float64       `json:"GISY"`
	Details   []PointDetail `json:"Details"`
}

type PointDetail struct {
	PointDetailID int    `json:"PointDetailID"`
	Time          string `json:"Time"`
	Week          string `json:"Week"`
}

type CarStatus struct {
	RouteID    int     `json:"Route_ID"`
	CarUnicode string  `json:"Car_Unicode"`
	GISX       float64 `json:"GISX"`
	GISY       float64 `json:"GISY"`
	UseState   int     `json:"UseState"`
}

type RouteStatus struct {
	RouteID          int                `json:"Route_ID"`
	RouteWaitingTime int                `json:"RouteWaitingTime"`
	Points           []RouteStatusPoint `json:"Points"`
}

type RouteStatusPoint struct {
	PointID       int    `json:"Point_ID"`
	EstimatedTime string `json:"EstimatedTime"`
	WaitingTime   int    `json:"WaitingTime"`
}

type TargetStop struct {
	CustID        int
	TeamID        int
	RouteID       int
	RouteName     string
	PointID       int
	PointSeq      int
	PointName     string
	ScheduledTime string
	GISX          float64
	GISY          float64
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimSpace(baseURL),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) GetDistrictByCustID(ctx context.Context, custID int) (*DistrictConfig, error) {
	var countries []CountryConfig
	if err := c.call(ctx, "GetCountryRemovalIsEnable", map[string]any{}, &countries, true); err != nil {
		return nil, err
	}

	for _, country := range countries {
		for _, district := range country.Districts {
			if district.CustID == custID {
				copy := district
				return &copy, nil
			}
		}
	}

	return nil, fmt.Errorf("cust id %d not found in district list", custID)
}

func (c *Client) GetAllRouteBasicData(ctx context.Context, custID int) ([]Route, error) {
	var routes []Route
	if err := c.call(ctx, "GetAllRouteBasicData", map[string]any{"Cust_ID": custID}, &routes, true); err != nil {
		return nil, err
	}
	return routes, nil
}

func (c *Client) GetCarStatusGarbage(ctx context.Context, custID, teamID int) ([]CarStatus, error) {
	var cars []CarStatus
	err := c.call(ctx, "GetCarStatusGarbage", map[string]any{
		"Cust_ID": custID,
		"Team_ID": teamID,
	}, &cars, false)
	if err != nil {
		return nil, err
	}
	return cars, nil
}

func (c *Client) GetAllRouteStatusData(ctx context.Context, custID int) ([]RouteStatus, error) {
	var statuses []RouteStatus
	if err := c.call(ctx, "GetAllRouteStatusData", map[string]any{"Cust_ID": custID}, &statuses, true); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) ResolveTargetStop(ctx context.Context, custID, routeID, pointSeq int, pointName string) (*TargetStop, error) {
	routes, err := c.GetAllRouteBasicData(ctx, custID)
	if err != nil {
		return nil, err
	}

	return FindTargetStop(routes, routeID, pointSeq, pointName)
}

func FindTargetStop(routes []Route, routeID, pointSeq int, pointName string) (*TargetStop, error) {
	for _, route := range routes {
		if route.RouteID != routeID {
			continue
		}

		for _, point := range route.Points {
			if point.Seq != pointSeq {
				continue
			}
			if point.PointName != pointName {
				return nil, fmt.Errorf("target point mismatch: expected %q, got %q", pointName, point.PointName)
			}

			return &TargetStop{
				RouteID:       route.RouteID,
				RouteName:     route.RouteName,
				PointID:       point.PointID,
				PointSeq:      point.Seq,
				PointName:     point.PointName,
				ScheduledTime: firstPointTime(point),
				GISX:          point.GISX,
				GISY:          point.GISY,
			}, nil
		}

		return nil, fmt.Errorf("point seq %d not found in route %d", pointSeq, routeID)
	}

	return nil, fmt.Errorf("route %d not found", routeID)
}

func firstPointTime(point Point) string {
	for _, detail := range point.Details {
		if strings.TrimSpace(detail.Time) != "" {
			return detail.Time
		}
	}
	return ""
}

func (c *Client) call(ctx context.Context, methodName string, payload map[string]any, out any, strictStatus bool) error {
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["MethodName"] = methodName

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Eupfin payload: %w", err)
	}

	form := url.Values{}
	form.Set("Param", string(jsonPayload))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return fmt.Errorf("create Eupfin request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call Eupfin API %s: %w", methodName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Eupfin API %s returned status %d: %s", methodName, resp.StatusCode, string(body))
	}

	var envelope responseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode Eupfin response %s: %w", methodName, err)
	}

	if strictStatus && envelope.Status != 1 {
		return fmt.Errorf("Eupfin API %s returned status %d: %s", methodName, envelope.Status, envelope.Error)
	}

	if !strictStatus && (envelope.Status == 3 || len(envelope.Result) == 0) {
		return nil
	}

	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}

	if err := json.Unmarshal(envelope.Result, out); err != nil {
		return fmt.Errorf("decode Eupfin result %s: %w", methodName, err)
	}

	return nil
}
