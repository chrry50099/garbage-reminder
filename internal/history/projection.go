package history

import (
	"fmt"
	"math"
	"sort"
	"time"

	"telegram-garbage-reminder/internal/eupfin"
	"telegram-garbage-reminder/internal/geo"
)

const (
	defaultAmbiguousHeadingDelta = 15.0
	metersPerDegreeLatitude      = 111320.0
)

type RouteShape struct {
	segments             []routeSegment
	TargetProgressMeters float64
}

type ProjectionConfig struct {
	ProgressWindowMeters          float64
	LateralOffsetLimitMeters      float64
	BacktrackToleranceMeters      float64
	AmbiguousSegmentEpsilonMeters float64
}

type ProjectionResult struct {
	ProgressMeters      float64
	SegmentIndex        int
	LateralOffsetMeters float64
	CandidateCount      int
	HeadingDegrees      float64
}

type ProjectionCandidate struct {
	ProgressMeters      float64
	SegmentIndex        int
	LateralOffsetMeters float64
	HeadingDegrees      float64
}

type RecentSample struct {
	TruckLat            float64
	TruckLng            float64
	ProgressMeters      *float64
	SegmentIndex        *int
	LateralOffsetMeters *float64
	CollectedAt         time.Time
}

type routeSegment struct {
	Index                int
	StartLat             float64
	StartLng             float64
	EndLat               float64
	EndLng               float64
	StartProgressMeters  float64
	EndProgressMeters    float64
	HeadingDegrees       float64
	ReferenceLatitudeRad float64
	LengthMeters         float64
}

type candidateScore struct {
	candidate     ProjectionCandidate
	advanceMeters float64
	headingDelta  float64
}

func BuildRouteShape(route eupfin.Route, targetPointSeq int) (*RouteShape, error) {
	if len(route.Points) < 2 {
		return nil, fmt.Errorf("route %d has fewer than 2 points", route.RouteID)
	}

	points := append([]eupfin.Point(nil), route.Points...)
	sort.Slice(points, func(i, j int) bool {
		return points[i].Seq < points[j].Seq
	})

	segments := make([]routeSegment, 0, len(points)-1)
	var cumulative float64
	var targetProgress float64
	targetFound := false

	for i := 0; i < len(points)-1; i++ {
		start := points[i]
		end := points[i+1]
		length := geo.CalculateDistance(start.GISY, start.GISX, end.GISY, end.GISX)
		if start.Seq == targetPointSeq {
			targetProgress = cumulative
			targetFound = true
		}
		if length == 0 {
			continue
		}

		segment := routeSegment{
			Index:                len(segments),
			StartLat:             start.GISY,
			StartLng:             start.GISX,
			EndLat:               end.GISY,
			EndLng:               end.GISX,
			StartProgressMeters:  cumulative,
			EndProgressMeters:    cumulative + length,
			HeadingDegrees:       bearingDegrees(start.GISY, start.GISX, end.GISY, end.GISX),
			ReferenceLatitudeRad: ((start.GISY + end.GISY) / 2) * math.Pi / 180,
			LengthMeters:         length,
		}
		segments = append(segments, segment)
		cumulative += length
	}

	if !targetFound {
		lastPoint := points[len(points)-1]
		if lastPoint.Seq == targetPointSeq {
			targetProgress = cumulative
			targetFound = true
		}
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("route %d has no usable segments", route.RouteID)
	}
	if !targetFound {
		return nil, fmt.Errorf("target point seq %d not found in route %d", targetPointSeq, route.RouteID)
	}

	return &RouteShape{
		segments:             segments,
		TargetProgressMeters: targetProgress,
	}, nil
}

func (r *RouteShape) ProjectCandidates(lat, lng float64) []ProjectionCandidate {
	if r == nil {
		return nil
	}

	candidates := make([]ProjectionCandidate, 0, len(r.segments))
	for _, segment := range r.segments {
		projection := projectOntoSegment(segment, lat, lng)
		candidates = append(candidates, projection)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].LateralOffsetMeters == candidates[j].LateralOffsetMeters {
			return candidates[i].ProgressMeters < candidates[j].ProgressMeters
		}
		return candidates[i].LateralOffsetMeters < candidates[j].LateralOffsetMeters
	})
	return candidates
}

func (r *RouteShape) Project(lat, lng float64, recent []RecentSample, cfg ProjectionConfig) (*ProjectionResult, bool) {
	candidates := r.ProjectCandidates(lat, lng)
	if len(candidates) == 0 {
		return nil, false
	}
	if candidates[0].LateralOffsetMeters > cfg.LateralOffsetLimitMeters {
		return nil, false
	}

	ambiguous := ambiguousCandidates(candidates, cfg.AmbiguousSegmentEpsilonMeters)
	if len(ambiguous) == 1 {
		selected := ambiguous[0]
		return &ProjectionResult{
			ProgressMeters:      selected.ProgressMeters,
			SegmentIndex:        selected.SegmentIndex,
			LateralOffsetMeters: selected.LateralOffsetMeters,
			CandidateCount:      1,
			HeadingDegrees:      selected.HeadingDegrees,
		}, true
	}

	selected, ok := r.selectCandidate(ambiguous, recent, cfg)
	if !ok {
		return nil, false
	}

	return &ProjectionResult{
		ProgressMeters:      selected.ProgressMeters,
		SegmentIndex:        selected.SegmentIndex,
		LateralOffsetMeters: selected.LateralOffsetMeters,
		CandidateCount:      len(ambiguous),
		HeadingDegrees:      selected.HeadingDegrees,
	}, true
}

func (r *RouteShape) selectCandidate(candidates []ProjectionCandidate, recent []RecentSample, cfg ProjectionConfig) (ProjectionCandidate, bool) {
	lastProgress, ok := latestProgress(recent)
	if !ok {
		return ProjectionCandidate{}, false
	}

	filtered := make([]ProjectionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ProgressMeters < lastProgress-cfg.BacktrackToleranceMeters {
			continue
		}
		filtered = append(filtered, candidate)
	}
	if len(filtered) == 0 {
		return ProjectionCandidate{}, false
	}
	if len(filtered) == 1 {
		return filtered[0], true
	}

	heading, hasHeading := r.inferHeading(recent)
	scored := make([]candidateScore, 0, len(filtered))
	for _, candidate := range filtered {
		advance := candidate.ProgressMeters - lastProgress
		if advance < 0 {
			advance = 0
		}
		score := candidateScore{
			candidate:     candidate,
			advanceMeters: advance,
			headingDelta:  180,
		}
		if hasHeading {
			score.headingDelta = headingDifferenceDegrees(heading, candidate.HeadingDegrees)
		}
		scored = append(scored, score)
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].advanceMeters == scored[j].advanceMeters {
			if scored[i].headingDelta == scored[j].headingDelta {
				return scored[i].candidate.LateralOffsetMeters < scored[j].candidate.LateralOffsetMeters
			}
			return scored[i].headingDelta < scored[j].headingDelta
		}
		return scored[i].advanceMeters < scored[j].advanceMeters
	})

	if len(scored) > 1 && isSelectionAmbiguous(scored[0], scored[1], hasHeading, cfg.AmbiguousSegmentEpsilonMeters) {
		return ProjectionCandidate{}, false
	}

	return scored[0].candidate, true
}

func (r *RouteShape) inferHeading(recent []RecentSample) (float64, bool) {
	if len(recent) >= 2 {
		prev := recent[len(recent)-2]
		last := recent[len(recent)-1]
		if prev.TruckLat != last.TruckLat || prev.TruckLng != last.TruckLng {
			return bearingDegrees(prev.TruckLat, prev.TruckLng, last.TruckLat, last.TruckLng), true
		}
	}

	if len(recent) == 0 {
		return 0, false
	}
	last := recent[len(recent)-1]
	if last.SegmentIndex != nil && *last.SegmentIndex >= 0 && *last.SegmentIndex < len(r.segments) {
		return r.segments[*last.SegmentIndex].HeadingDegrees, true
	}

	return 0, false
}

func latestProgress(recent []RecentSample) (float64, bool) {
	for i := len(recent) - 1; i >= 0; i-- {
		if recent[i].ProgressMeters != nil {
			return *recent[i].ProgressMeters, true
		}
	}
	return 0, false
}

func ambiguousCandidates(candidates []ProjectionCandidate, epsilon float64) []ProjectionCandidate {
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0].LateralOffsetMeters
	limit := best + epsilon
	ambiguous := make([]ProjectionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.LateralOffsetMeters > limit {
			break
		}
		ambiguous = append(ambiguous, candidate)
	}
	return ambiguous
}

func isSelectionAmbiguous(best, second candidateScore, hasHeading bool, epsilon float64) bool {
	if math.Abs(best.advanceMeters-second.advanceMeters) > epsilon {
		return false
	}
	if !hasHeading {
		return true
	}
	return math.Abs(best.headingDelta-second.headingDelta) <= defaultAmbiguousHeadingDelta
}

func projectOntoSegment(segment routeSegment, lat, lng float64) ProjectionCandidate {
	px, py := latLngToMeters(segment.ReferenceLatitudeRad, segment.StartLat, segment.StartLng, lat, lng)
	sx, sy := 0.0, 0.0
	ex, ey := latLngToMeters(segment.ReferenceLatitudeRad, segment.StartLat, segment.StartLng, segment.EndLat, segment.EndLng)
	vx := ex - sx
	vy := ey - sy
	lengthSquared := vx*vx + vy*vy
	if lengthSquared == 0 {
		return ProjectionCandidate{
			ProgressMeters:      segment.StartProgressMeters,
			SegmentIndex:        segment.Index,
			LateralOffsetMeters: geo.CalculateDistance(lat, lng, segment.StartLat, segment.StartLng),
			HeadingDegrees:      segment.HeadingDegrees,
		}
	}

	t := ((px-sx)*vx + (py-sy)*vy) / lengthSquared
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	projX := sx + t*vx
	projY := sy + t*vy
	lateralOffset := math.Hypot(px-projX, py-projY)
	progress := segment.StartProgressMeters + t*segment.LengthMeters

	return ProjectionCandidate{
		ProgressMeters:      progress,
		SegmentIndex:        segment.Index,
		LateralOffsetMeters: lateralOffset,
		HeadingDegrees:      segment.HeadingDegrees,
	}
}

func latLngToMeters(referenceLatitudeRad, originLat, originLng, lat, lng float64) (float64, float64) {
	x := (lng - originLng) * metersPerDegreeLatitude * math.Cos(referenceLatitudeRad)
	y := (lat - originLat) * metersPerDegreeLatitude
	return x, y
}

func bearingDegrees(lat1, lng1, lat2, lng2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	y := math.Sin(deltaLng) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(deltaLng)
	angle := math.Atan2(y, x) * 180 / math.Pi
	if angle < 0 {
		angle += 360
	}
	return angle
}

func headingDifferenceDegrees(a, b float64) float64 {
	diff := math.Abs(a - b)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}
