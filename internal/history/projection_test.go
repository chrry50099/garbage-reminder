package history

import (
	"testing"
	"time"

	"telegram-garbage-reminder/internal/eupfin"
)

func TestBuildRouteShapeAndProjectSingleSegment(t *testing.T) {
	shape, err := BuildRouteShape(eupfin.Route{
		RouteID: 1,
		Points: []eupfin.Point{
			{Seq: 1, GISY: 24.000000, GISX: 121.000000},
			{Seq: 2, GISY: 24.000900, GISX: 121.000000},
		},
	}, 2)
	if err != nil {
		t.Fatalf("BuildRouteShape() error: %v", err)
	}

	result, ok := shape.Project(24.000450, 121.000000, nil, ProjectionConfig{
		AmbiguousSegmentEpsilonMeters: 15,
		BacktrackToleranceMeters:      30,
		LateralOffsetLimitMeters:      80,
	})
	if !ok {
		t.Fatal("expected projection")
	}
	if result.ProgressMeters < 45 || result.ProgressMeters > 55 {
		t.Fatalf("unexpected progress: %.2f", result.ProgressMeters)
	}
	if result.LateralOffsetMeters > 1 {
		t.Fatalf("unexpected lateral offset: %.2f", result.LateralOffsetMeters)
	}
}

func TestProjectCandidatesDetectCrossingAmbiguity(t *testing.T) {
	shape := buildFigureEightShape(t)
	candidates := shape.ProjectCandidates(24.000000, 121.000500)
	ambiguous := ambiguousCandidates(candidates, 15)
	if len(ambiguous) < 2 {
		t.Fatalf("expected crossing ambiguity, got %+v", ambiguous)
	}
}

func TestProjectResolvesCrossingUsingRecentProgress(t *testing.T) {
	shape := buildFigureEightShape(t)
	progress := 210.0
	segmentIndex := 1
	result, ok := shape.Project(24.000000, 121.000500, []RecentSample{
		{
			TruckLat:       24.000150,
			TruckLng:       121.000250,
			ProgressMeters: &progress,
			SegmentIndex:   &segmentIndex,
			CollectedAt:    time.Now(),
		},
	}, ProjectionConfig{
		AmbiguousSegmentEpsilonMeters: 15,
		BacktrackToleranceMeters:      30,
		LateralOffsetLimitMeters:      80,
	})
	if !ok {
		t.Fatal("expected resolved projection")
	}
	if result.ProgressMeters > 400 {
		t.Fatalf("expected first crossing branch, got progress %.2f", result.ProgressMeters)
	}
}

func TestProjectFallsBackWhenCrossingHasNoContext(t *testing.T) {
	shape := buildFigureEightShape(t)
	if _, ok := shape.Project(24.000000, 121.000500, nil, ProjectionConfig{
		AmbiguousSegmentEpsilonMeters: 15,
		BacktrackToleranceMeters:      30,
		LateralOffsetLimitMeters:      80,
	}); ok {
		t.Fatal("expected ambiguous projection without context to fail")
	}
}

func TestProjectRejectsLargeLateralOffset(t *testing.T) {
	shape, err := BuildRouteShape(eupfin.Route{
		RouteID: 1,
		Points: []eupfin.Point{
			{Seq: 1, GISY: 24.000000, GISX: 121.000000},
			{Seq: 2, GISY: 24.000900, GISX: 121.000000},
		},
	}, 2)
	if err != nil {
		t.Fatalf("BuildRouteShape() error: %v", err)
	}

	if _, ok := shape.Project(24.000450, 121.000900, nil, ProjectionConfig{
		AmbiguousSegmentEpsilonMeters: 15,
		BacktrackToleranceMeters:      30,
		LateralOffsetLimitMeters:      80,
	}); ok {
		t.Fatal("expected projection to fail when lateral offset exceeds limit")
	}
}

func TestProjectAcceptsModerateLateralOffsets(t *testing.T) {
	shape, err := BuildRouteShape(eupfin.Route{
		RouteID: 1,
		Points: []eupfin.Point{
			{Seq: 1, GISY: 24.000000, GISX: 121.000000},
			{Seq: 2, GISY: 24.000900, GISX: 121.000000},
		},
	}, 2)
	if err != nil {
		t.Fatalf("BuildRouteShape() error: %v", err)
	}

	for _, lng := range []float64{121.000295, 121.000590} {
		if _, ok := shape.Project(24.000450, lng, nil, ProjectionConfig{
			AmbiguousSegmentEpsilonMeters: 15,
			BacktrackToleranceMeters:      30,
			LateralOffsetLimitMeters:      80,
		}); !ok {
			t.Fatalf("expected projection to succeed for offset lng %.6f", lng)
		}
	}
}

func buildFigureEightShape(t *testing.T) *RouteShape {
	t.Helper()
	shape, err := BuildRouteShape(eupfin.Route{
		RouteID: 1,
		Points: []eupfin.Point{
			{Seq: 1, GISY: 24.000000, GISX: 121.000000},
			{Seq: 2, GISY: 24.000300, GISX: 121.000250},
			{Seq: 3, GISY: 24.000000, GISX: 121.000500},
			{Seq: 4, GISY: 23.999700, GISX: 121.000250},
			{Seq: 5, GISY: 24.000000, GISX: 121.000000},
			{Seq: 6, GISY: 24.000300, GISX: 121.000750},
			{Seq: 7, GISY: 24.000000, GISX: 121.000500},
			{Seq: 8, GISY: 23.999700, GISX: 121.000750},
			{Seq: 9, GISY: 24.000000, GISX: 121.001000},
		},
	}, 9)
	if err != nil {
		t.Fatalf("BuildRouteShape() error: %v", err)
	}
	return shape
}
