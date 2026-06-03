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
	
	"github.com/AleksKAG/RoomScan-AI/internal/ai"
	"github.com/AleksKAG/RoomScan-AI/internal/api"
	_ "github.com/AleksKAG/RoomScan-AI/docs"
	"github.com/AleksKAG/RoomScan-AI/internal/cleanup"
	"github.com/AleksKAG/RoomScan-AI/internal/config"
	"github.com/AleksKAG/RoomScan-AI/internal/db"
	"github.com/AleksKAG/RoomScan-AI/internal/queue"
	"github.com/AleksKAG/RoomScan-AI/internal/worker"
)

// @title github.com/AleksKAG/RoomScan-AI API
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

	// 3. Инициализация AI генератора
	aiGen := ai.NewGenerator(
		os.Getenv("KANDINSKY_URL"),
		os.Getenv("KANDINSKY_KEY"),
		os.Getenv("GIGACHAT_URL"),
		os.Getenv("GIGACHAT_KEY"),
	)

	// 4. Инициализация воркера
	processor := worker.NewProcessor(cfg, store, aiGen)
	qServer := queue.NewServer(redisAddr, processor)
	defer qServer.Stop()

	// 5. Запуск фоновой очистки (удаляет файлы старше 24 часов)
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	go cleanup.StartRoutine(cleanupCtx, []string{cfg.UploadDir, cfg.TempDir}, 24*time.Hour)

	// 6. API Handler
	apiHandler := api.NewHandler(cfg, qClient, store)

	// 7. Роутер
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	r.Route("/api/v1", func(r chi.Router) {
		r.With(api.RealIPMiddleware, api.RateLimiter()).Post("/upload", apiHandler.HandleUpload)
		r.Get("/status/{id}", apiHandler.HandleStatus)
		r.Get("/ws/ar/{id}", apiHandler.HandleARStream)
	})

	// Раздача статических файлов (результатов)
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir))))

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
