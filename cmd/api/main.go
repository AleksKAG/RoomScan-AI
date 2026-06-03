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
	_ "github.com/AleksKAG/RoomScan-AI/docs" // Подключение сгенерированной Swagger-документации
	"github.com/AleksKAG/RoomScan-AI/internal/cleanup"
	"github.com/AleksKAG/RoomScan-AI/internal/config"
	"github.com/AleksKAG/RoomScan-AI/internal/db"
	"github.com/AleksKAG/RoomScan-AI/internal/queue"
	"github.com/AleksKAG/RoomScan-AI/internal/worker"
)

// @title RoomScan-AI API
// @version 1.0
// @description API for room scanning, geometry detection, and AI design generation.
// @host localhost:8080
// @BasePath /
func main() {
	// Глобальный перехват паник, чтобы приложение не падало молча
	defer func() {
		if r := recover(); r != nil {
			slog.Error("PANIC RECOVERED", "panic", r)
			os.Exit(1)
		}
	}()

	// 1. Конфигурация и логирование
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	slog.Info("Starting RoomScan-AI", "version", "1.0.0")

	// 2. Создание необходимых директорий
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		slog.Error("Failed to create upload dir", "error", err, "path", cfg.UploadDir)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		slog.Error("Failed to create temp dir", "error", err, "path", cfg.TempDir)
		os.Exit(1)
	}

	// 3. Инициализация БД
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "postgres://roomscan:secret@localhost:5432/roomscan?sslmode=disable"
		slog.Warn("DATABASE_URL not set, using default", "dsn", dbDSN)
	}

	slog.Info("Connecting to database...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	
	store, err := db.NewStore(ctx, dbDSN)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err, "dsn", dbDSN)
		cancel()
		os.Exit(1)
	}
	defer store.Close()
	slog.Info("Database connected successfully")

	if err := store.InitSchema(ctx); err != nil {
		slog.Error("Failed to init DB schema", "error", err)
		cancel()
		os.Exit(1)
	}
	cancel() // Освобождаем контекст инициализации
	slog.Info("Database schema initialized")

	// 4. Инициализация очереди (Redis)
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
		slog.Warn("REDIS_ADDR not set, using default", "addr", redisAddr)
	}

	slog.Info("Connecting to Redis...")
	qClient := queue.NewClient(redisAddr)
	defer qClient.Close()

	// Проверка реального подключения к Redis
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := qClient.Ping(pingCtx); err != nil {
		slog.Error("Failed to connect to Redis", "error", err, "addr", redisAddr)
		pingCancel()
		os.Exit(1)
	}
	pingCancel()
	slog.Info("Redis connected successfully")

	// 5. Инициализация AI генератора
	aiGen := ai.NewGenerator(
		os.Getenv("KANDINSKY_URL"),
		os.Getenv("KANDINSKY_KEY"),
		os.Getenv("GIGACHAT_URL"),
		os.Getenv("GIGACHAT_KEY"),
	)
	slog.Info("AI generator initialized")

	// 6. Инициализация воркера и сервера очереди
	processor := worker.NewProcessor(cfg, store, aiGen)
	slog.Info("Worker processor created")

	slog.Info("Starting queue server...")
	qServer := queue.NewServer(redisAddr, processor)
	defer qServer.Stop()
	slog.Info("Queue server started")

	// 7. Запуск фоновой задачи очистки старых файлов
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	go cleanup.StartRoutine(cleanupCtx, []string{cfg.UploadDir, cfg.TempDir}, 24*time.Hour)
	slog.Info("Cleanup routine started")

	// 8. Настройка HTTP роутера
	apiHandler := api.NewHandler(cfg, qClient, store)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer) // Перехват паник в HTTP-обработчиках
	r.Use(middleware.Timeout(60 * time.Second))

	// Базовые маршруты
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	// API маршруты v1
	r.Route("/api/v1", func(r chi.Router) {
		r.With(api.RealIPMiddleware, api.RateLimiter()).Post("/upload", apiHandler.HandleUpload)
		r.Get("/status/{id}", apiHandler.HandleStatus)
		r.Get("/ws/ar/{id}", apiHandler.HandleARStream)
	})

	// Раздача статических файлов (результатов обработки)
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir))))

	// 9. Настройка и запуск HTTP сервера
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
			os.Exit(1)
		}
	}()

	slog.Info("Application started successfully", "port", cfg.ServerPort)

	// 10. Graceful Shutdown (корректное завершение работы)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	qServer.Stop()
	slog.Info("Server exited properly")
}
