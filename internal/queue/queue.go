package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	TypeProcessVideo = "video:process"
)

// Client для отправки задач в очередь
type Client struct {
	redis *asynq.Client
	addr  string // Сохраняем адрес для проверки здоровья (health check)
}

// NewClient создает новый клиент очереди
func NewClient(redisAddr string) *Client {
	return &Client{
		redis: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
		addr:  redisAddr,
	}
}

// Ping проверяет подключение к Redis
func (c *Client) Ping(ctx context.Context) error {
	// Создаем временный клиент go-redis только для проверки пинга
	rdb := redis.NewClient(&redis.Options{Addr: c.addr})
	defer rdb.Close()
	
	return rdb.Ping(ctx).Err()
}

// EnqueueProcessVideo добавляет задачу обработки видео в очередь
func (c *Client) EnqueueProcessVideo(taskID, filePath string) error {
	payload := fmt.Sprintf(`{"id":"%s","path":"%s"}`, taskID, filePath)
	task := asynq.NewTask(TypeProcessVideo, []byte(payload))
	
	info, err := c.redis.EnqueueContext(context.Background(), task, asynq.MaxRetry(3), asynq.Timeout(5*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	
	slog.Info("Task enqueued", "task_id", info.ID, "queue", info.Queue)
	return nil
}

// Close закрывает клиент
func (c *Client) Close() {
	c.redis.Close()
}

// VideoProcessor интерфейс для обработчика видео
type VideoProcessor interface {
	HandleProcessVideo(ctx context.Context, t *asynq.Task) error
}

// Server сервер очереди задач
type Server struct {
	srv *asynq.Server
}

// NewServer создает и запускает сервер очереди
func NewServer(redisAddr string, processor VideoProcessor) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"default": 10,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeProcessVideo, processor.HandleProcessVideo)

	srv.Start(mux)
	return &Server{srv: srv}
}

// Stop останавливает сервер
func (s *Server) Stop() {
	s.srv.Stop()
	s.srv.Shutdown()
}
