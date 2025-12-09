package worker

import (
	"fmt"
	"path/filepath"

	"github.com/phpdave11/gofpdf"
	"github.com/yourname/roomscan/internal/geometry"
	"github.com/yourname/roomscan/internal/render"
)

func ProcessVideo(id, videoPath string) error {
	framesDir := filepath.Join(uploadDir, id+"_frames")
	resultsDir := filepath.Join(uploadDir, id+"_results")
	os.MkdirAll(resultsDir, 0755)

	// Extract frames
	ExtractKeyFrames(nil, videoPath, framesDir)

	// Select frame (MVP: first)
	framePath := filepath.Join(framesDir, "frame_0001.jpg")

	// Detect & fit
	lines, _ := geometry.DetectLines(framePath)
	poly := geometry.FitPolygonFromLines(lines)

	// Render 2D
	planPath := filepath.Join(resultsDir, "plan.png")
	render.RenderPlan(poly, planPath)

	// Generate 3D
	gltfPath := filepath.Join(resultsDir, "model.gltf")
	render.GenerateGLTF(poly, gltfPath, 3.0)

	// PDF report (новый)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "RoomScan Report")
	pdf.Image(planPath, 10, 20, 180, 0, false, "", 0, "")
	pdf.OutputFileAndClose(filepath.Join(resultsDir, "report.pdf"))

	// Mark done
	os.WriteFile(filepath.Join(resultsDir, "status.json"), []byte(`{"status":"done"}`), 0644)

	return nil
}
