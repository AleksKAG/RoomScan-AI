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
	httpSwagger "github.com/swaggo/http-swagger"
	
	"roomscan-ai/internal/api"
	_ "roomscan-ai/docs" // ВАЖНО: подключает сгенерированную swagger-документацию
	"roomscan-ai/internal/config"
	"roomscan-ai/internal/db"
	"roomscan-ai/internal/queue"
	"roomscan-ai/internal/worker"
)

// @title RoomScan-AI API
// @version 1.0
// @description API for room scanning, geometry detection, and AI design generation.
// @host localhost:8080
// @BasePath /
func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	os.MkdirAll(cfg.UploadDir, 0755)
	os.MkdirAll(cfg.TempDir, 0755)

	// 1. Инициализация БД
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "postgres://roomscan:secret@localhost:5432/roomscan?sslmode=disable"
	}
	
	store, err := db.NewStore(context.Background(), dbDSN)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.InitSchema(context.Background()); err != nil {
		slog.Error("Failed to init DB schema", "error", err)
		os.Exit(1)
	}

	// 2. Инициализация очереди
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	
	qClient := queue.NewClient(redisAddr)
	defer qClient.Close()

	processor := worker.NewProcessor(cfg, store)
	qServer := queue.NewServer(redisAddr, processor)
	defer qServer.Stop()

	// 3. API Handler
	apiHandler := api.NewHandler(cfg, qClient, store)

	// 4. Роутер с middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Handle("/metrics", promhttp.Handler())
	
	// Swagger UI: http://localhost:8080/swagger/index.html
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), 
	))

	r.Route("/api/v1", func(r chi.Router) {
		// Rate Limiting применяется только к загрузке
		r.With(api.RealIPMiddleware, api.RateLimiter()).Post("/upload", apiHandler.HandleUpload)
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
	qServer.Stop()
	slog.Info("Server exited properly")
}
