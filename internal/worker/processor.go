package worker

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"roomscan-ai/internal/config"
	"roomscan-ai/internal/geometry"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type Processor struct {
	cfg     *config.Config
	statuses map[string]Status
	mu      sync.RWMutex
}

func NewProcessor(cfg *config.Config) *Processor {
	return &Processor{
		cfg:      cfg,
		statuses: make(map[string]Status),
	}
}

func (p *Processor) ProcessVideo(id, srcPath string) error {
	p.updateStatus(id, StatusProcessing)
	slog.Info("Started processing video", "id", id, "path", srcPath)

	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in worker", "id", id, "panic", r)
			p.updateStatus(id, StatusFailed)
		}
	}()

	// 1. Извлечение кадров (заглушка, здесь должна быть логика ffmpeg или gocv.VideoCapture)
	framePath := filepath.Join(p.cfg.TempDir, id+"_frame.jpg")
	
	// потом... здесь должен извлекаеться ключевой кадр из видео
	// Для примера просто копируем или создаем заглушку
	if err := p.extractKeyFrame(srcPath, framePath); err != nil {
		p.updateStatus(id, StatusFailed)
		return fmt.Errorf("extract frame: %w", err)
	}
	defer os.Remove(framePath) // Очистка временного кадра

	// 2. Детекция линий с использованием конфигурационных порогов
	lines, err := geometry.DetectLines(framePath, p.cfg.CannyThreshold1, p.cfg.CannyThreshold2, p.cfg.HoughThreshold, p.cfg.MinLineLength, p.cfg.MaxLineGap)
	if err != nil {
		p.updateStatus(id, StatusFailed)
		return fmt.Errorf("detect lines: %w", err)
	}

	// 3. Построение полигона
	poly := geometry.FitPolygonFromLines(lines)
	
	// 4. Сохранение результата
	resultPath := filepath.Join(p.cfg.UploadDir, id+"_result.json")
	if err := geometry.SavePolygon(resultPath, poly); err != nil {
		p.updateStatus(id, StatusFailed)
		return fmt.Errorf("save polygon: %w", err)
	}

	p.updateStatus(id, StatusCompleted)
	slog.Info("Processing completed successfully", "id", id)
	return nil
}

func (p *Processor) GetStatus(id string) (Status, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	status, exists := p.statuses[id]
	if !exists {
		return "", fmt.Errorf("status not found")
	}
	return status, nil
}

func (p *Processor) updateStatus(id string, status Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statuses[id] = status
}

func (p *Processor) CleanupTemp() {
	slog.Info("Cleaning up temporary files...")
	// Простая реализация: удаление всех файлов в TempDir
	entries, err := os.ReadDir(p.cfg.TempDir)
	if err != nil {
		slog.Error("Failed to read temp dir", "error", err)
		return
	}
	for _, entry := range entries {
		os.Remove(filepath.Join(p.cfg.TempDir, entry.Name()))
	}
}

// extractKeyFrame - заглушка для извлечения кадра. 
// В продакшене здесь должен быть вызов ffmpeg или gocv.VideoCapture
func (p *Processor) extractKeyFrame(src, dst string) error {
	// TODO: Реализовать через gocv.VideoCapture или exec.Command("ffmpeg", ...)
	slog.Warn("extractKeyFrame is a stub, using dummy file")
	
	// Создаем пустой файл для прохождения тестов геометрии
	return os.WriteFile(dst, []byte("dummy"), 0644)
}
