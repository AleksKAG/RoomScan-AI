package geometry

import (
	"fmt"
	"image"
	"math"

	"gocv.io/x/gocv"
)

// DetectLines detects wall edges using Canny and Probabilistic Hough Transform.
// Returns lines as [][]float64 {x1, y1, x2, y2}.
func DetectLines(framePath string) ([][]float64, error) {
	img := gocv.IMRead(framePath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("could not read image: %s", framePath)
	}
	defer img.Close()

	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)

	// Reduce noise
	gocv.GaussianBlur(gray, &gray, image.Point{X: 5, Y: 5}, 0, 0, gocv.BorderDefault)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(gray, &edges, 50, 150) // Thresholds can be tuned

	linesMat := gocv.NewMat()
	defer linesMat.Close()
	gocv.HoughLinesP(edges, &linesMat, 1, math.Pi/180, 100, 30, 10) // rho=1, theta=1 deg, thresh=100, minLen=30, maxGap=10

	var lines [][]float64
	for i := 0; i < linesMat.Rows(); i++ {
		l := linesMat.GetVeciAt(i, 0)
		lines = append(lines, []float64{float64(l[0]), float64(l[1]), float64(l[2]), float64(l[3])})
	}

	return lines, nil
}

// DetectCorners detects room corners using Shi-Tomasi (GoodFeaturesToTrack).
// Returns []Point.
func DetectCorners(framePath string) ([]Point, error) {
	img := gocv.IMRead(framePath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("could not read image: %s", framePath)
	}
	defer img.Close()

	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)

	cornersMat := gocv.NewMat()
	defer cornersMat.Close()
	gocv.GoodFeaturesToTrack(gray, &cornersMat, 50, 0.01, 10) // maxCorners=50, quality=0.01, minDist=10

	var corners []Point
	for i := 0; i < cornersMat.Rows(); i++ {
		p := cornersMat.GetVecfAt(i, 0)
		corners = append(corners, Point{X: float64(p[0]), Y: float64(p[1])})
	}

	return corners, nil
}

// FindIntersections computes intersection points between lines.
// Simple pairwise check; optimize for production.
func FindIntersections(lines [][]float64) []Point {
	var points []Point
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			pt, intersects := lineIntersection(lines[i], lines[j])
			if intersects {
				points = append(points, pt)
			}
		}
	}
	return points
}

// lineIntersection calculates intersection of two lines [x1,y1,x2,y2].
// Returns Point and bool (true if intersects).
func lineIntersection(l1, l2 []float64) (Point, bool) {
	x1, y1, x2, y2 := l1[0], l1[1], l1[2], l1[3]
	x3, y3, x4, y4 := l2[0], l2[1], l2[2], l2[3]

	den := (x1-x2)*(y3-y4) - (y1-y2)*(x3-x4)
	if den == 0 {
		return Point{}, false // parallel
	}

	t := ((x1-x3)*(y3-y4) - (y1-y3)*(x3-x4)) / den
	u := -((x1-x2)*(y1-y3) - (y1-y2)*(x1-x3)) / den

	if t >= 0 && t <= 1 && u >= 0 && u <= 1 {
		px := x1 + t*(x2-x1)
		py := y1 + t*(y2-y1)
		return Point{X: px, Y: py}, true
	}
	return Point{}, false
}
