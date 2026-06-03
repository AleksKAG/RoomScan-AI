//go:build !opencv

package geometry

import (
	"encoding/json"
	"fmt"
	"os"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Polygon []Point

func DetectLines(imagePath string, thresh1, thresh2, houghThresh int, minLineLen, maxLineGap float64) ([][]int, error) {
	fmt.Println("WARNING: OpenCV not available, using stub")
	return [][]int{
		{100, 100, 500, 100},
		{500, 100, 500, 400},
		{500, 400, 100, 400},
		{100, 400, 100, 100},
	}, nil
}

func FitPolygonFromLines(lines [][]int) Polygon {
	var poly Polygon
	seen := make(map[string]bool)
	for _, line := range lines {
		p1 := Point{X: float64(line[0]), Y: float64(line[1])}
		p2 := Point{X: float64(line[2]), Y: float64(line[3])}
		k1 := fmt.Sprintf("%.0f_%.0f", p1.X, p1.Y)
		k2 := fmt.Sprintf("%.0f_%.0f", p2.X, p2.Y)
		if !seen[k1] { poly = append(poly, p1); seen[k1] = true }
		if !seen[k2] { poly = append(poly, p2); seen[k2] = true }
	}
	return poly
}

func SavePolygon(path string, poly Polygon) error {
	data, err := json.MarshalIndent(poly, "", "  ")
	if err != nil { return err }
	return os.WriteFile(path, data, 0644)
}
