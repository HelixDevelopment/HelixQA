package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PersistenceDB is the narrow database surface the persistence layer
// requires. Any driver that implements the stdlib sql.DB methods works,
// which keeps our SQL code portable across SQLite and PostgreSQL.
type PersistenceDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// AuditPersister writes AuditLog entries to helixqa_audit_log.
type AuditPersister struct {
	db PersistenceDB
}

// NewAuditPersister returns a persister bound to db.
func NewAuditPersister(db PersistenceDB) (*AuditPersister, error) {
	if db == nil {
		return nil, errors.New("audit persister: nil db")
	}
	return &AuditPersister{db: db}, nil
}

// Save writes entry to the helixqa_audit_log table.
func (p *AuditPersister) Save(ctx context.Context, entry AuditEntry) error {
	at := entry.At
	if at.IsZero() {
		at = time.Now()
	}
	allowed := 0
	if entry.Allowed {
		allowed = 1
	}
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO helixqa_audit_log
			(user_id, user_email, team, role, action, resource, allowed, reason, at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.User.ID, entry.User.Email, entry.User.Team, string(entry.User.Role),
		string(entry.Action), entry.Resource, allowed, entry.Reason, at)
	if err != nil {
		return fmt.Errorf("audit save: %w", err)
	}
	return nil
}

// Flush drains every entry from log into the db. Entries are written
// in order; a failed entry aborts the flush so callers can retry with
// their own backoff.
func (p *AuditPersister) Flush(ctx context.Context, log *AuditLog) error {
	if log == nil {
		return nil
	}
	for _, e := range log.Entries() {
		if err := p.Save(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// FlowPersister writes cross-platform flow runs to
// helixqa_cross_flows + helixqa_flow_steps.
type FlowPersister struct {
	db PersistenceDB
}

// NewFlowPersister returns a persister bound to db.
func NewFlowPersister(db PersistenceDB) (*FlowPersister, error) {
	if db == nil {
		return nil, errors.New("flow persister: nil db")
	}
	return &FlowPersister{db: db}, nil
}

// StartFlow records the beginning of a flow and returns the persisted
// flow id so callers can associate subsequent step rows.
func (p *FlowPersister) StartFlow(ctx context.Context, id, name string) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO helixqa_cross_flows (flow_id, name, started_at) VALUES (?, ?, ?)`,
		id, name, time.Now())
	if err != nil {
		return fmt.Errorf("flow start: %w", err)
	}
	return nil
}

// EndFlow records the outcome of a flow.
func (p *FlowPersister) EndFlow(ctx context.Context, id, result string, failedStep int, notes string) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE helixqa_cross_flows
		 SET ended_at = ?, result = ?, failed_step = ?, notes = ?
		 WHERE flow_id = ?`,
		time.Now(), result, failedStep, notes, id)
	if err != nil {
		return fmt.Errorf("flow end: %w", err)
	}
	return nil
}

// Step records a single flow step's outcome.
func (p *FlowPersister) Step(ctx context.Context, flowID string, index int, name, platform, result, reason string, start, end time.Time) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO helixqa_flow_steps
			(flow_id, step_index, name, platform, started_at, ended_at, result, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, flowID, index, name, platform, start, end, result, reason)
	if err != nil {
		return fmt.Errorf("flow step: %w", err)
	}
	return nil
}
