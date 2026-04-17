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
// AuditLog for later inspection.
type AccessControl struct {
	audit *AuditLog
}

// NewAccessControl returns a policy enforcer that appends audit entries
// to log.
func NewAccessControl(log *AuditLog) *AccessControl {
	if log == nil {
		log = NewAuditLog()
	}
	return &AccessControl{audit: log}
}

// Check returns nil when u may perform action against resource; an
// error otherwise. Every call (pass or fail) is appended to the audit
// log.
func (ac *AccessControl) Check(u User, action Action, resource string) error {
	required, ok := defaultPolicy[action]
	if !ok {
		err := fmt.Errorf("rbac: unknown action %q", action)
		ac.audit.Record(AuditEntry{User: u, Action: action, Resource: resource, At: time.Now(), Allowed: false, Reason: err.Error()})
		return err
	}
	if roleRank[u.Role] < roleRank[required] {
		err := fmt.Errorf("rbac: %s role cannot perform %s (requires %s)", u.Role, action, required)
		ac.audit.Record(AuditEntry{User: u, Action: action, Resource: resource, At: time.Now(), Allowed: false, Reason: err.Error()})
		return err
	}
	ac.audit.Record(AuditEntry{User: u, Action: action, Resource: resource, At: time.Now(), Allowed: true})
	return nil
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
