//go:build !opencv

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/hibiken/asynq"

	"roomscan-ai/internal/config"
	"roomscan-ai/internal/db"
	"roomscan-ai/internal/geometry"
)

type Processor struct {
	cfg *config.Config
	db  *db.Store
}

func NewProcessor(cfg *config.Config, db *db.Store) *Processor {
	return &Processor{cfg: cfg, db: db}
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

	// Создаем фиктивный результат
	poly := geometry.Polygon{
		{X: 100, Y: 100},
		{X: 500, Y: 100},
		{X: 500, Y: 400},
		{X: 100, Y: 400},
	}

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
