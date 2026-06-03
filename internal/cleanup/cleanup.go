package cleanup

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// StartRoutine запускает фоновую очистку директорий
func StartRoutine(ctx context.Context, dirs []string, retention time.Duration) {
	slog.Info("Starting cleanup routine", "retention", retention.String())

	ticker := time.NewTicker(1 * time.Hour) // Проверять каждый час
	defer ticker.Stop()

	// Запускаем первую очистку сразу
	cleanupDirs(dirs, retention)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Cleanup routine stopped")
			return
		case <-ticker.C:
			cleanupDirs(dirs, retention)
		}
	}
}

func cleanupDirs(dirs []string, retention time.Duration) {
	now := time.Now()
	cutoff := now.Add(-retention)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			slog.Error("Failed to read directory for cleanup", "dir", dir, "error", err)
			continue
		}

		deletedCount := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				path := filepath.Join(dir, entry.Name())
				if err := os.Remove(path); err != nil {
					slog.Warn("Failed to delete old file", "path", path, "error", err)
				} else {
					deletedCount++
				}
			}
		}
		
		if deletedCount > 0 {
			slog.Info("Cleanup completed", "dir", dir, "deleted_files", deletedCount)
		}
	}
}
