package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/hibiken/asynq"
	"gocv.io/x/gocv"

	"roomscan-ai/internal/config"
	"roomscan-ai/internal/geometry"
	"roomscan-ai/internal/queue"
)

type Processor struct {
	cfg *config.Config
}

func NewProcessor(cfg *config.Config) *Processor {
	return &Processor{cfg: cfg}
}

// HandleProcessVideo реализует интерфейс asynq.Handler
func (p *Processor) HandleProcessVideo(ctx context.Context, t *asynq.Task) error {
	var payload struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	slog.Info("Processing video task started", "id", payload.ID, "path", payload.Path)

	// 1. Извлечение ключевого кадра
	framePath := filepath.Join(p.cfg.TempDir, payload.ID+"_frame.jpg")
	if err := p.extractKeyFrame(payload.Path, framePath); err != nil {
		return fmt.Errorf("extract frame: %w", err) // Ошибка вернет задачу в очередь на retry
	}
	defer os.Remove(framePath) // Гарантированная очистка

	// 2. Детекция линий
	lines, err := geometry.DetectLines(
		framePath, 
		p.cfg.CannyThreshold1, 
		p.cfg.CannyThreshold2, 
		p.cfg.HoughThreshold, 
		p.cfg.MinLineLength, 
		p.cfg.MaxLineGap,
	)
	if err != nil {
		return fmt.Errorf("detect lines: %w", err)
	}

	// 3. Построение и сохранение полигона
	poly := geometry.FitPolygonFromLines(lines)
	resultPath := filepath.Join(p.cfg.UploadDir, payload.ID+"_result.json")
	
	if err := geometry.SavePolygon(resultPath, poly); err != nil {
		return fmt.Errorf("save polygon: %w", err)
	}

	slog.Info("Video processing completed successfully", "id", payload.ID)
	return nil
}

// extractKeyFrame РЕАЛЬНАЯ реализация через OpenCV
func (p *Processor) extractKeyFrame(src, dst string) error {
	cap, err := gocv.OpenVideoCapture(src)
	if err != nil {
		return fmt.Errorf("failed to open video capture: %w", err)
	}
	defer cap.Close()

	mat := gocv.NewMat()
	defer mat.Close()

	// Берем 30-й кадр (примерно 1 секунда при 30fps), 
	// чтобы дать пользователю время стабилизировать камеру после начала записи.
	targetFrame := 30
	currentFrame := 0

	for {
		if ok := cap.Read(&mat); !ok {
			break // Конец видео или ошибка чтения
		}
		currentFrame++
		if currentFrame >= targetFrame {
			break
		}
	}

	if mat.Empty() {
		return fmt.Errorf("video is empty or too short to extract frame")
	}

	if !gocv.IMWrite(dst, mat) {
		return fmt.Errorf("failed to write frame to %s", dst)
	}
	
	return nil
}
