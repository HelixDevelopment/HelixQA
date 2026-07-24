package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type TaskHandler interface {
	HandleTask(ctx context.Context, payload []byte) error
	Type() string
}

const (
	maxRetries   int16 = 5
	retryBackoff       = 30 * time.Second
)

type BackgroundService struct {
	db       *pgxpool.Pool
	logger   *zap.Logger
	workers  int
	pollInt  time.Duration
	handlers map[string]TaskHandler
}

func NewBackgroundService(db *pgxpool.Pool, logger *zap.Logger, workers int, pollInterval time.Duration) *BackgroundService {
	return &BackgroundService{
		db:       db,
		logger:   logger,
		workers:  workers,
		pollInt:  pollInterval,
		handlers: make(map[string]TaskHandler),
	}
}

func (s *BackgroundService) RegisterHandler(handler TaskHandler) {
	s.handlers[handler.Type()] = handler
	s.logger.Info("registered background task handler", zap.String("type", handler.Type()))
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

	handler, ok := s.handlers[task.Type]
	if !ok {
		s.logger.Warn("no handler registered for task type, marking as dead",
			zap.String("type", task.Type),
			zap.String("task_id", task.ID.String()),
		)
		_, err := s.db.Exec(ctx,
			`UPDATE background_tasks SET status = 'dead', last_error = $2, updated_at = NOW() WHERE id = $1`,
			task.ID, "no handler registered for type: "+task.Type,
		)
		return err
	}

	if err := handler.HandleTask(ctx, task.Payload); err != nil {
		s.logger.Error("task handler failed",
			zap.String("task_id", task.ID.String()),
			zap.String("type", task.Type),
			zap.Error(err),
		)
		return s.handleTaskError(ctx, task, err)
	}

	_, err := s.db.Exec(ctx,
		`UPDATE background_tasks SET status = 'completed', updated_at = NOW() WHERE id = $1`,
		task.ID,
	)
	return err
}

func (s *BackgroundService) handleTaskError(ctx context.Context, task *backgroundTaskRow, taskErr error) error {
	errStr := taskErr.Error()
	if task.Attempts >= maxRetries {
		_, err := s.db.Exec(ctx,
			`UPDATE background_tasks SET status = 'dead', last_error = $2, updated_at = NOW() WHERE id = $1`,
			task.ID, fmt.Sprintf("max retries exceeded (%d): %s", maxRetries, errStr),
		)
		return err
	}

	nextRun := time.Now().Add(retryBackoff * time.Duration(task.Attempts+1))
	_, err := s.db.Exec(ctx,
		`UPDATE background_tasks SET status = 'pending', last_error = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1`,
		task.ID, errStr, nextRun,
	)
	return err
}
