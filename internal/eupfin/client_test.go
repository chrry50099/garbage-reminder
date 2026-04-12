package eupfin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveTargetStopParsesExpectedRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		if !strings.Contains(string(body), "GetAllRouteBasicData") {
			t.Fatalf("expected GetAllRouteBasicData payload, got %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": 1,
			"result": [
				{
					"Route_ID": 461,
					"RouteName": "雙溪線(每周一、二、四、五資源回收)",
					"Points": [
						{
							"Point_ID": 27,
							"Seq": 27,
							"PointName": "有謙家園",
							"GISX": 121.02032,
							"GISY": 24.748448,
							"Details": [
								{
									"PointDetailID": 1001,
									"Time": "20:30",
									"Week": "每周一、二、四、五"
								}
							]
						}
					]
				}
			],
			"error": ""
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	target, err := client.ResolveTargetStop(context.Background(), 5005808, 461, 27, "有謙家園", "20:30")
	if err != nil {
		t.Fatalf("ResolveTargetStop() returned error: %v", err)
	}

	if target.PointName != "有謙家園" {
		t.Fatalf("unexpected point name: %s", target.PointName)
	}
	if target.GISX != 121.02032 || target.GISY != 24.748448 {
		t.Fatalf("unexpected coordinates: %f,%f", target.GISX, target.GISY)
	}
}

func TestFindTargetStopRejectsMismatchedName(t *testing.T) {
	routes := []Route{
		{
			RouteID:   461,
			RouteName: "雙溪線",
			Points: []Point{
				{
					PointID:   27,
					Seq:       27,
					PointName: "其他站點",
					Details: []PointDetail{
						{Time: "20:30"},
					},
				},
			},
		},
	}

	if _, err := FindTargetStop(routes, 461, 27, "有謙家園", "20:30"); err == nil {
		t.Fatal("expected FindTargetStop to reject mismatched point name")
	}
}
