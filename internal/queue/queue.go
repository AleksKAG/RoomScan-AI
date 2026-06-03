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

type Client struct {
	redis *asynq.Client
}

func NewClient(redisAddr string) *Client {
	return &Client{
		redis: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

// Ping проверяет подключение к Redis
func (c *Client) Ping(ctx context.Context) error {
	// Создаем временный Redis клиент для проверки
	rdb := redis.NewClient(&redis.Options{Addr: c.redis.Addr()})
	defer rdb.Close()
	return rdb.Ping(ctx).Err()
}

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

func (c *Client) Close() {
	c.redis.Close()
}

type VideoProcessor interface {
	HandleProcessVideo(ctx context.Context, t *asynq.Task) error
}

type Server struct {
	srv *asynq.Server
}

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

func (s *Server) Stop() {
	s.srv.Stop()
	s.srv.Shutdown()
}
