package geometry

// geometry.go contains primitives for geometry processing of lines and corners.

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Polygon struct {
	Points []Point `json:"points"`
}

// FitPolygonFromLines takes detected lines and returns room polygon.
// Uses intersections as corners; simplistic for MVP (assumes convex room, no duplicates).
func FitPolygonFromLines(lines [][]float64) Polygon {
	intersections := FindIntersections(lines)

	// TODO: Cluster/deduplicate points, sort by angle or convex hull (e.g., Graham scan).
	// For MVP: just return unsorted points as polygon.
	return Polygon{Points: intersections}
}
