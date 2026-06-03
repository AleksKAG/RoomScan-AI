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
	task := asynq.NewTask(TypeProcessVideo, []byte(fmt.Sprintf(`{"id":"%s","path":"%s"}`, taskID, filePath)))
	
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

// VideoProcessor определяет интерфейс для обработки видео
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
