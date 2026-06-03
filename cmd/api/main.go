package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	
	"roomscan-ai/internal/api"
	"roomscan-ai/internal/config"
	"roomscan-ai/internal/worker"
)

func main() {
	// 1. Инициализация конфигурации
	cfg := config.Load()

	// 2. Настройка логгера (JSON формат для продакшена)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 3. Создание директорий
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		slog.Error("Failed to create upload dir", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		slog.Error("Failed to create temp dir", "error", err)
		os.Exit(1)
	}

	// 4. Инициализация зависимостей
	workerSvc := worker.NewProcessor(cfg)
	apiHandler := api.NewHandler(cfg, workerSvc)

	// 5. Настройка роутера
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Маршруты
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Handle("/metrics", promhttp.Handler())
	
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/upload", apiHandler.HandleUpload)
		r.Get("/status/{id}", apiHandler.HandleStatus)
		r.Get("/ws/ar/{id}", apiHandler.HandleARStream)
	})

	// 6. Настройка HTTP сервера
	srv := &http.Server{
		Addr:         cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// 7. Запуск сервера в горутине
	go func() {
		slog.Info("Starting server", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
		}
	}()

	// 8. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}
	
	// Очистка временных файлов при выходе
	workerSvc.CleanupTemp()
	
	slog.Info("Server exited properly")
}
