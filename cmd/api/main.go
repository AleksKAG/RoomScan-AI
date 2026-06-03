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
	"roomscan-ai/internal/queue"
	"roomscan-ai/internal/worker"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Директории
	os.MkdirAll(cfg.UploadDir, 0755)
	os.MkdirAll(cfg.TempDir, 0755)

	// Инициализация очереди
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default
	}
	
	qClient := queue.NewClient(redisAddr)
	defer qClient.Close()

	// Инициализация воркера и сервера очереди
	processor := worker.NewProcessor(cfg)
	qServer := queue.NewServer(redisAddr, processor)
	defer qServer.Stop()

	// API Handler
	apiHandler := api.NewHandler(cfg, qClient)

	// Роутер
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	
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

	srv := &http.Server{
		Addr:         cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("Starting HTTP server", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
	
	slog.Info("Server exited properly")
}
