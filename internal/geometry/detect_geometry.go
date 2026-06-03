package geometry

import (
	"encoding/json"
	"fmt"
	"os"

	"gocv.io/x/gocv"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Polygon []Point

// DetectLines теперь принимает параметры явно, без магических чисел
func DetectLines(imagePath string, thresh1, thresh2, houghThresh int, minLineLen, maxLineGap float64) ([][]int, error) {
	img := gocv.IMRead(imagePath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("failed to read image: %s", imagePath)
	}
	defer img.Close()

	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(gray, &edges, thresh1, thresh2)

	lines := gocv.NewMat()
	defer lines.Close()
	gocv.HoughLinesP(edges, &lines, 1, gocv.PI/180.0, houghThresh, minLineLen, maxLineGap)

	var result [][]int
	for i := 0; i < lines.Rows(); i++ {
		result = append(result, []int{
			int(lines.GetFloatAt(i, 0)),
			int(lines.GetFloatAt(i, 1)),
			int(lines.GetFloatAt(i, 2)),
			int(lines.GetFloatAt(i, 3)),
		})
	}

	return result, nil
}

func FitPolygonFromLines(lines [][]int) Polygon {
	// Упрощенная заглушка. В реальности здесь алгоритм объединения линий в замкнутый контур
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
