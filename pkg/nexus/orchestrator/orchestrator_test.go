package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- ExecutionContext ---

func TestExecutionContext_GetSet(t *testing.T) {
	ec := NewExecutionContext()
	ec.Set("token", "abc")
	if v, ok := ec.Get("token"); !ok || v != "abc" {
		t.Errorf("Get = %v, %v", v, ok)
	}
	if _, ok := ec.Get("missing"); ok {
		t.Error("missing key returned ok")
	}
}

func TestExecutionContext_SnapshotIsCopy(t *testing.T) {
	ec := NewExecutionContext()
	ec.Set("a", 1)
	s := ec.Snapshot()
	s["a"] = 99
	if v, _ := ec.Get("a"); v != 1 {
		t.Errorf("Snapshot should return a copy, original mutated to %v", v)
	}
}

// --- Flow ---

func TestFlow_RunExecutesAllSteps(t *testing.T) {
	calls := []string{}
	f := Flow{Steps: []Step{
		{Name: "s1", Platform: PlatformWeb, Action: func(_ context.Context, ec *ExecutionContext) error { calls = append(calls, "s1"); return nil }},
		{Name: "s2", Platform: PlatformAPI, Action: func(_ context.Context, ec *ExecutionContext) error { calls = append(calls, "s2"); return nil }, Verify: func(_ context.Context, _ *ExecutionContext) error { calls = append(calls, "v2"); return nil }},
	}}
	if err := f.Run(context.Background(), NewExecutionContext()); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "s1,s2,v2" {
		t.Errorf("sequence wrong: %v", calls)
	}
}

func TestFlow_RunAbortsOnFirstError(t *testing.T) {
	f := Flow{Steps: []Step{
		{Name: "s1", Action: func(_ context.Context, ec *ExecutionContext) error { return nil }},
		{Name: "s2", Action: func(_ context.Context, ec *ExecutionContext) error { return errors.New("boom") }},
		{Name: "s3", Action: func(_ context.Context, ec *ExecutionContext) error { t.Error("step 3 should not run"); return nil }},
	}}
	err := f.Run(context.Background(), NewExecutionContext())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestFlow_VerifyErrorStopsFlow(t *testing.T) {
	f := Flow{Steps: []Step{
		{Name: "s1", Action: func(_ context.Context, _ *ExecutionContext) error { return nil },
			Verify: func(_ context.Context, _ *ExecutionContext) error { return errors.New("verify fail") }},
	}}
	err := f.Run(context.Background(), NewExecutionContext())
	if err == nil || !strings.Contains(err.Error(), "verify fail") {
		t.Fatalf("expected verify fail, got %v", err)
	}
}

func TestFlow_NilActionReturnsError(t *testing.T) {
	f := Flow{Steps: []Step{{Name: "broken"}}}
	if err := f.Run(context.Background(), NewExecutionContext()); err == nil {
		t.Fatal("nil action must error")
	}
}

func TestFlow_NilContextRejected(t *testing.T) {
	f := Flow{Steps: []Step{{Name: "x", Action: func(_ context.Context, _ *ExecutionContext) error { return nil }}}}
	if err := f.Run(context.Background(), nil); err == nil {
		t.Fatal("nil ExecutionContext must error")
	}
}

// --- Evidence ---

func TestFileEvidenceStore_PutAndList(t *testing.T) {
	s, err := NewFileEvidenceStore(filepath.Join(t.TempDir(), "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	url, err := s.Put("sessions/abc/step1/frame.png", []byte("PNG"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(url); err != nil {
		t.Errorf("file not written: %v", err)
	}
	items, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("list = %d, want 1", len(items))
	}
}

func TestNewFileEvidenceStore_EmptyRootRejected(t *testing.T) {
	if _, err := NewFileEvidenceStore(""); err == nil {
		t.Fatal("empty root should error")
	}
}

func TestEvidence_NoStoreSilentlyAccepts(t *testing.T) {
	e := NewEvidence()
	// nil store should not panic and should return an empty URL.
	url, err := e.Screenshot("sess", "step", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if url != "" {
		t.Errorf("expected empty URL when store is nil, got %q", url)
	}
}

func TestEvidence_StoreAttach(t *testing.T) {
	s, _ := NewFileEvidenceStore(filepath.Join(t.TempDir(), "ev"))
	e := NewEvidence()
	e.SetStore(s)
	if _, err := e.Log("sess-1", "step-1", "hello"); err != nil {
		t.Fatal(err)
	}
	items, _ := s.List()
	if len(items) != 1 || !strings.Contains(items[0].Name, "log.txt") {
		t.Errorf("log not written: %+v", items)
	}
}

// --- RBAC ---

func TestAccessControl_RoleEnforcement(t *testing.T) {
	ac := NewAccessControl(nil)
	viewer := User{ID: "u1", Role: RoleViewer}
	operator := User{ID: "u2", Role: RoleOperator}

	if err := ac.Check(viewer, ActionViewReport, "sessions/1"); err != nil {
		t.Errorf("viewer should view reports: %v", err)
	}
	if err := ac.Check(viewer, ActionEditBank, "bank/1"); err == nil {
		t.Error("viewer should not edit banks")
	}
	if err := ac.Check(operator, ActionEditBank, "bank/1"); err != nil {
		t.Errorf("operator should edit banks: %v", err)
	}
	if err := ac.Check(operator, ActionManageUsers, "users"); err == nil {
		t.Error("operator should not manage users")
	}
}

func TestAccessControl_UnknownActionRejected(t *testing.T) {
	ac := NewAccessControl(nil)
	err := ac.Check(User{Role: RoleAdmin}, "made_up", "x")
	if err == nil {
		t.Fatal("unknown action should error")
	}
}

func TestAuditLog_RecordsEveryCheck(t *testing.T) {
	log := NewAuditLog()
	ac := NewAccessControl(log)
	_ = ac.Check(User{Role: RoleViewer}, ActionViewReport, "x")
	_ = ac.Check(User{Role: RoleViewer}, ActionEditBank, "y")
	if log.Len() != 2 {
		t.Errorf("audit len = %d", log.Len())
	}
	entries := log.Entries()
	if !entries[0].Allowed || entries[1].Allowed {
		t.Errorf("audit entries wrong: %+v", entries)
	}
}

func TestAuditLog_EntriesCopy(t *testing.T) {
	log := NewAuditLog()
	log.Record(AuditEntry{User: User{ID: "u"}, At: time.Now()})
	entries := log.Entries()
	entries[0].User.ID = "mutated"
	if log.Entries()[0].User.ID != "u" {
		t.Error("audit entries must be a copy")
	}
}
