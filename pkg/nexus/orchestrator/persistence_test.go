package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeDB records every SQL call and returns canned responses.
type fakeDB struct {
	mu      sync.Mutex
	execs   []call
	queries []call
	execErr error
}

type call struct {
	query string
	args  []any
}

func (f *fakeDB) ExecContext(_ context.Context, q string, args ...any) (sql.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.execErr != nil {
		return nil, f.execErr
	}
	f.execs = append(f.execs, call{q, args})
	return fakeResult{}, nil
}
func (f *fakeDB) QueryContext(_ context.Context, q string, args ...any) (*sql.Rows, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, call{q, args})
	return nil, nil
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

func TestAuditPersister_SaveRecordsEntry(t *testing.T) {
	db := &fakeDB{}
	p, _ := NewAuditPersister(db)
	entry := AuditEntry{
		User:     User{ID: "u1", Email: "e@x", Team: "t", Role: RoleOperator},
		Action:   ActionEditBank,
		Resource: "bank/1",
		Allowed:  true,
		At:       time.Now(),
	}
	if err := p.Save(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	if len(db.execs) != 1 {
		t.Fatalf("exec count = %d", len(db.execs))
	}
	call := db.execs[0]
	if call.args[0] != "u1" || call.args[5] != "bank/1" || call.args[6] != 1 {
		t.Errorf("unexpected args: %v", call.args)
	}
}

func TestAuditPersister_Flush(t *testing.T) {
	db := &fakeDB{}
	p, _ := NewAuditPersister(db)
	log := NewAuditLog()
	log.Record(AuditEntry{User: User{Role: RoleViewer}, Action: ActionViewReport, Allowed: true})
	log.Record(AuditEntry{User: User{Role: RoleViewer}, Action: ActionEditBank, Allowed: false, Reason: "forbidden"})
	if err := p.Flush(context.Background(), log); err != nil {
		t.Fatal(err)
	}
	if len(db.execs) != 2 {
		t.Fatalf("expected 2 inserts, got %d", len(db.execs))
	}
}

func TestAuditPersister_FlushNilLogSafe(t *testing.T) {
	db := &fakeDB{}
	p, _ := NewAuditPersister(db)
	if err := p.Flush(context.Background(), nil); err != nil {
		t.Errorf("nil log should no-op, got %v", err)
	}
}

func TestAuditPersister_SavePropagatesError(t *testing.T) {
	db := &fakeDB{execErr: errors.New("db down")}
	p, _ := NewAuditPersister(db)
	if err := p.Save(context.Background(), AuditEntry{User: User{Role: RoleAdmin}}); err == nil {
		t.Fatal("expected error from db")
	}
}

func TestFlowPersister_LifecycleCalls(t *testing.T) {
	db := &fakeDB{}
	p, err := NewFlowPersister(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.StartFlow(context.Background(), "f1", "Checkout"); err != nil {
		t.Fatal(err)
	}
	if err := p.Step(context.Background(), "f1", 1, "Login", "web", "pass", "", time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := p.EndFlow(context.Background(), "f1", "pass", 0, ""); err != nil {
		t.Fatal(err)
	}
	if len(db.execs) != 3 {
		t.Fatalf("expected 3 exec calls, got %d", len(db.execs))
	}
}

func TestFlowPersister_NilDBRejected(t *testing.T) {
	if _, err := NewFlowPersister(nil); err == nil {
		t.Fatal("nil db should fail")
	}
}
