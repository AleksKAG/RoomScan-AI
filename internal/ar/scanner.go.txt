package ar

  import (
      "gocv.io/x/gocv"
      "github.com/yourname/roomscan/internal/geometry"
  )

  func ScanRoom(frame gocv.Mat) geometry.Polygon {
      // Детекция линий и углов (как в detect_geometry.go)
      lines, _ := geometry.DetectLinesFromMat(frame);
      poly := geometry.FitPolygonFromLines(lines);
      // Добавьте SLAM: используйте ORB-SLAM binding или simple feature matching
      return poly;
  }
