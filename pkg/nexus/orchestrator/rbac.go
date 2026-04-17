package orchestrator

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Role is a named permission bundle.
type Role string

const (
	RoleViewer   Role = "viewer"
	RoleRunner   Role = "runner"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

// Action names a gated capability.
type Action string

const (
	ActionViewReport    Action = "view_report"
	ActionStartSession  Action = "start_session"
	ActionStopSession   Action = "stop_session"
	ActionEditBank      Action = "edit_bank"
	ActionManageUsers   Action = "manage_users"
	ActionConfigureBudget Action = "configure_budget"
)

// defaultPolicy maps each action to the minimum role required.
var defaultPolicy = map[Action]Role{
	ActionViewReport:       RoleViewer,
	ActionStartSession:     RoleRunner,
	ActionStopSession:      RoleOperator,
	ActionEditBank:         RoleOperator,
	ActionManageUsers:      RoleAdmin,
	ActionConfigureBudget:  RoleAdmin,
}

var roleRank = map[Role]int{
	RoleViewer: 1, RoleRunner: 2, RoleOperator: 3, RoleAdmin: 4,
}

// User is an authenticated identity.
type User struct {
	ID    string
	Email string
	Role  Role
	Team  string
}

// AccessControl enforces the RBAC policy + logs every check to the
// AuditLog for later inspection. When a sink is configured via
// SetSink, every entry is also pushed to the sink (typically
// *AuditPersister.Save so the `helixqa_audit_log` SQL table captures
// the full history in addition to the in-memory AuditLog). Sink
// errors never block the access decision so an outage of the
// persistence layer cannot open nor close the RBAC gate.
type AccessControl struct {
	audit *AuditLog
	sink  AuditSink
}

// AuditSink is the hook the RBAC middleware calls on every decision.
// A nil AuditSink is a no-op. *AuditPersister satisfies this via an
// adapter so AccessControl.SetSink(persister.AsSink()) wires the
// decision stream into `helixqa_audit_log`.
type AuditSink func(entry AuditEntry)

// NewAccessControl returns a policy enforcer that appends audit entries
// to log.
func NewAccessControl(log *AuditLog) *AccessControl {
	if log == nil {
		log = NewAuditLog()
	}
	return &AccessControl{audit: log}
}

// SetSink attaches a post-record hook. Passing nil clears the hook.
// Safe to call at any time; the hook is invoked after every Check.
func (ac *AccessControl) SetSink(sink AuditSink) *AccessControl {
	ac.sink = sink
	return ac
}

// Check returns nil when u may perform action against resource; an
// error otherwise. Every call (pass or fail) is appended to the audit
// log and mirrored to the configured AuditSink (when set).
func (ac *AccessControl) Check(u User, action Action, resource string) error {
	required, ok := defaultPolicy[action]
	if !ok {
		entry := AuditEntry{User: u, Action: action, Resource: resource, At: time.Now(), Allowed: false, Reason: fmt.Sprintf("rbac: unknown action %q", action)}
		ac.record(entry)
		return fmt.Errorf("%s", entry.Reason)
	}
	if roleRank[u.Role] < roleRank[required] {
		entry := AuditEntry{User: u, Action: action, Resource: resource, At: time.Now(), Allowed: false, Reason: fmt.Sprintf("rbac: %s role cannot perform %s (requires %s)", u.Role, action, required)}
		ac.record(entry)
		return fmt.Errorf("%s", entry.Reason)
	}
	ac.record(AuditEntry{User: u, Action: action, Resource: resource, At: time.Now(), Allowed: true})
	return nil
}

func (ac *AccessControl) record(entry AuditEntry) {
	ac.audit.Record(entry)
	if ac.sink != nil {
		ac.sink(entry)
	}
}

// AuditLog is the append-only history of access-control decisions.
type AuditLog struct {
	mu      sync.Mutex
	entries []AuditEntry
}

// AuditEntry is a single audit record.
type AuditEntry struct {
	User     User
	Action   Action
	Resource string
	At       time.Time
	Allowed  bool
	Reason   string
}

// NewAuditLog returns an empty log.
func NewAuditLog() *AuditLog { return &AuditLog{} }

// Record appends e.
func (l *AuditLog) Record(e AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
}

// Entries returns a copy of all audit rows.
func (l *AuditLog) Entries() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]AuditEntry, len(l.entries))
	copy(cp, l.entries)
	return cp
}

// Len reports the size of the log.
func (l *AuditLog) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// ErrForbidden is returned when a user lacks the required role.
var ErrForbidden = errors.New("rbac: forbidden")
