//go:build opencv

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

	"github.com/AleksKAG/RoomScan-AI/internal/ai"
	"github.com/AleksKAG/RoomScan-AI/internal/config"
	"github.com/AleksKAG/RoomScan-AI/internal/db"
	"github.com/AleksKAG/RoomScan-AI/internal/geometry"
)

type Processor struct {
	cfg *config.Config
	db  *db.Store
	ai  *ai.Generator
}

func NewProcessor(cfg *config.Config, db *db.Store, aiGen *ai.Generator) *Processor {
	return &Processor{cfg: cfg, db: db, ai: aiGen}
}

func (p *Processor) HandleProcessVideo(ctx context.Context, t *asynq.Task) error {
	var payload struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	p.db.UpdateStatus(ctx, payload.ID, db.StatusProcessing, "", "")
	slog.Info("Processing started", "id", payload.ID)

	framePath := filepath.Join(p.cfg.TempDir, payload.ID+"_frame.jpg")
	
	var processErr error
	defer func() {
		if r := recover(); r != nil {
			processErr = fmt.Errorf("panic: %v", r)
		}
		if processErr != nil {
			slog.Error("Processing failed", "id", payload.ID, "error", processErr)
			p.db.UpdateStatus(ctx, payload.ID, db.StatusFailed, "", processErr.Error())
		}
	}()

	if err := p.extractKeyFrame(payload.Path, framePath); err != nil {
		processErr = err
		return processErr
	}
	defer os.Remove(framePath)

	// 1. Геометрия
	lines, err := geometry.DetectLines(framePath, p.cfg.CannyThreshold1, p.cfg.CannyThreshold2, p.cfg.HoughThreshold, p.cfg.MinLineLength, p.cfg.MaxLineGap)
	if err != nil {
		processErr = err
		return processErr
	}

	poly := geometry.FitPolygonFromLines(lines)
	resultPath := filepath.Join(p.cfg.UploadDir, payload.ID+"_result.json")
	
	if err := geometry.SavePolygon(resultPath, poly); err != nil {
		processErr = err
		return processErr
	}

	// 2. AI Генерация дизайна (Новый этап)
	polyJSON, _ := json.Marshal(poly)
	prompt, err := p.ai.GenerateDesignPrompt(ctx, "guest_room", string(polyJSON))
	if err != nil {
		slog.Warn("Failed to generate prompt, using default", "error", err)
		prompt = "Modern minimalist room interior, bright colors, high quality, 4k"
	}

	imageData, err := p.ai.GenerateImage(ctx, prompt)
	if err != nil {
		processErr = fmt.Errorf("ai generate image: %w", err)
		return processErr
	}

	// 3. Сохранение сгенерированного изображения
	designPath := filepath.Join(p.cfg.UploadDir, payload.ID+"_design.jpg")
	if err := os.WriteFile(designPath, imageData, 0644); err != nil {
		processErr = fmt.Errorf("save design image: %w", err)
		return processErr
	}

	resultURL := "/uploads/" + payload.ID + "_result.json"
	designURL := "/uploads/" + payload.ID + "_design.jpg"
	
	// Обновляем статус и сохраняем URL дизайна в БД (предполагается, что вы добавите поле design_url в таблицу scans, или можно сохранить в result_url как JSON)
	p.db.UpdateStatus(ctx, payload.ID, db.StatusCompleted, resultURL+"|"+designURL, "")
	
	slog.Info("Processing and AI generation completed", "id", payload.ID)
	return nil
}

func (p *Processor) extractKeyFrame(src, dst string) error {
	cap, err := gocv.OpenVideoCapture(src)
	if err != nil {
		return fmt.Errorf("failed to open video capture: %w", err)
	}
	defer cap.Close()

	mat := gocv.NewMat()
	defer mat.Close()

	targetFrame := 30
	currentFrame := 0

	for {
		if ok := cap.Read(&mat); !ok {
			break
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
