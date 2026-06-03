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
	fmt.Printf("WARNING: OpenCV not available, using stub implementation\n")
	return [][]int{
		{100, 100, 500, 100},
		{500, 100, 500, 400},
		{500, 400, 100, 400},
		{100, 400, 100, 100},
	}, nil
}

func FitPolygonFromLines(lines [][]int) Polygon {
	var poly Polygon
	for _, line := range lines {
		poly = append(poly, Point{X: float64(line[0]), Y: float64(line[1])})
		poly = append(poly, Point{X: float64(line[2]), Y: float64(line[3])})
	}
	return poly
}

func SavePolygon(path string, poly Polygon) error {
	data, err := json.MarshalIndent(poly, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
