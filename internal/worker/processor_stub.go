//go:build !opencv

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/hibiken/asynq"

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

	slog.Info("Processing started (STUB MODE)", "id", payload.ID)
	p.db.UpdateStatus(ctx, payload.ID, db.StatusProcessing, "", "")

	lines, _ := geometry.DetectLines(payload.Path, 50, 150, 100, 30, 10)
	poly := geometry.FitPolygonFromLines(lines)

	resultPath := filepath.Join(p.cfg.UploadDir, payload.ID+"_result.json")
	if err := geometry.SavePolygon(resultPath, poly); err != nil {
		p.db.UpdateStatus(ctx, payload.ID, db.StatusFailed, "", err.Error())
		return err
	}

	resultURL := "/uploads/" + payload.ID + "_result.json"
	p.db.UpdateStatus(ctx, payload.ID, db.StatusCompleted, resultURL, "")
	slog.Info("Processing completed (STUB MODE)", "id", payload.ID)
	return nil
}
