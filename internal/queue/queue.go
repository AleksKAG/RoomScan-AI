package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
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

func (c *Client) EnqueueProcessVideo(taskID, filePath string) error {
	payload := map[string]string{
		"id":   taskID,
		"path": filePath,
	}
	
	task := asynq.NewTask(TypeProcessVideo, []byte(fmt.Sprintf(`{"id":"%s","path":"%s"}`, taskID, filePath)))
	
	// Настройки задачи: 3 попытки, задержка между попытками увеличивается
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

// --- Server & Handler ---

type Server struct {
	srv *asynq.Server
}

func NewServer(redisAddr string, handler asynq.Handler) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 5, // Максимум 5 видео обрабатываются одновременно
			Queues: map[string]int{
				"default": 10,
			},
		
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeProcessVideo, handler.HandleProcessVideo)

	srv.Start(mux)
	return &Server{srv: srv}
}

func (s *Server) Stop() {
	s.srv.Stop()
	s.srv.Shutdown()
}
