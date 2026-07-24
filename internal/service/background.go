package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type BackgroundService struct {
	db      *pgxpool.Pool
	logger  *zap.Logger
	workers int
	pollInt time.Duration
}

func NewBackgroundService(db *pgxpool.Pool, logger *zap.Logger, workers int, pollInterval time.Duration) *BackgroundService {
	return &BackgroundService{db: db, logger: logger, workers: workers, pollInt: pollInterval}
}

func (s *BackgroundService) Start(ctx context.Context) {
	for i := 0; i < s.workers; i++ {
		go s.worker(ctx, i)
	}
}

func (s *BackgroundService) Enqueue(ctx context.Context, taskType string, payload interface{}, priority int16) (uuid.UUID, error) {
	id := uuid.New()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO background_tasks (id, type, payload, status, priority, next_run_at)
		 VALUES ($1, $2, $3, 'pending', $4, NOW())`,
		id, taskType, payloadBytes, priority,
	)
	return id, err
}

func (s *BackgroundService) worker(ctx context.Context, id int) {
	s.logger.Info("starting background worker", zap.Int("worker_id", id))
	for {
		select {
		case <-ctx.Done():
			return
		default:
			task, err := s.claimTask(ctx)
			if err != nil || task == nil {
				time.Sleep(s.pollInt)
				continue
			}
			if err := s.processTask(ctx, task); err != nil {
				s.logger.Error("task failed", zap.String("task_id", task.ID.String()), zap.Error(err))
			}
		}
	}
}

type backgroundTaskRow struct {
	ID       uuid.UUID
	Type     string
	Payload  []byte
	Priority int16
	Attempts int16
}

func (s *BackgroundService) claimTask(ctx context.Context) (*backgroundTaskRow, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var task backgroundTaskRow
	err = tx.QueryRow(ctx,
		`UPDATE background_tasks SET status = 'running', locked_at = NOW(), attempts = attempts + 1
		 WHERE id = (
			 SELECT id FROM background_tasks
			 WHERE status = 'pending' AND next_run_at <= NOW()
			 ORDER BY priority DESC, next_run_at ASC
			 LIMIT 1
			 FOR UPDATE SKIP LOCKED
		 ) RETURNING id, type, payload, priority, attempts`,
	).Scan(&task.ID, &task.Type, &task.Payload, &task.Priority, &task.Attempts)
	if err != nil {
		return nil, err
	}
	return &task, tx.Commit(ctx)
}

func (s *BackgroundService) processTask(ctx context.Context, task *backgroundTaskRow) error {
	s.logger.Info("processing task", zap.String("task_id", task.ID.String()), zap.String("type", task.Type))
	// Mark as completed - actual task type dispatch will be added in later phases
	_, err := s.db.Exec(ctx,
		`UPDATE background_tasks SET status = 'completed', updated_at = NOW() WHERE id = $1`,
		task.ID,
	)
	return err
}
