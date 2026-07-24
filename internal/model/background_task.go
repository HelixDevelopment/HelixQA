package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusDead      TaskStatus = "dead"
)

type BackgroundTask struct {
	ID          uuid.UUID       `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Status      TaskStatus      `json:"status"`
	Priority    int16           `json:"priority"`
	Attempts    int16           `json:"attempts"`
	MaxAttempts int16           `json:"max_attempts"`
	LastError   string          `json:"last_error"`
	NextRunAt   *time.Time      `json:"next_run_at"`
	LockedBy    string          `json:"locked_by"`
	LockedAt    *time.Time      `json:"locked_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
