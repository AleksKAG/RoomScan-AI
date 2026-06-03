package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ScanStatus определяет возможные статусы задачи
type ScanStatus string

const (
	StatusQueued     ScanStatus = "queued"
	StatusProcessing ScanStatus = "processing"
	StatusCompleted  ScanStatus = "completed"
	StatusFailed     ScanStatus = "failed"
)

type Scan struct {
	ID        string     `json:"id"`
	Status    ScanStatus `json:"status"`
	FilePath  string     `json:"file_path"`
	ResultURL string     `json:"result_url,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	
	// Настройки пула соединений
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

// InitSchema создает таблицу, если её нет (для простоты MVP без миграционных инструментов)
func (s *Store) InitSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS scans (
			id VARCHAR(36) PRIMARY KEY,
			status VARCHAR(20) NOT NULL DEFAULT 'queued',
			file_path TEXT NOT NULL,
			result_url TEXT,
			error TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_scans_status ON scans(status);
	`
	_, err := s.pool.Exec(ctx, query)
	return err
}

func (s *Store) CreateScan(ctx context.Context, scan *Scan) error {
	query := `
		INSERT INTO scans (id, status, file_path) 
		VALUES ($1, $2, $3)
	`
	_, err := s.pool.Exec(ctx, query, scan.ID, scan.Status, scan.FilePath)
	return err
}

func (s *Store) UpdateStatus(ctx context.Context, id string, status ScanStatus, resultURL, errMsg string) error {
	query := `
		UPDATE scans 
		SET status = $2, result_url = COALESCE($3, result_url), error = COALESCE($4, error), updated_at = CURRENT_TIMESTAMP 
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query, id, status, resultURL, errMsg)
	return err
}

func (s *Store) GetScan(ctx context.Context, id string) (*Scan, error) {
	query := `SELECT id, status, file_path, result_url, error, created_at, updated_at FROM scans WHERE id = $1`
	var scan Scan
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&scan.ID, &scan.Status, &scan.FilePath, &scan.ResultURL, &scan.Error, &scan.CreatedAt, &scan.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &scan, nil
}
