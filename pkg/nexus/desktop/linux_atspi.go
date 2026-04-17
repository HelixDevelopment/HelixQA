package desktop

import (
	"context"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
)

// ATSPIBackend is the native (DBus) implementation of the three
// atspi-* shell helpers invoked by LinuxEngine. Operators opt in by
// calling LinuxEngine.WithATSPIBackend(NewATSPIBackend()).
//
// The backend is intentionally minimal: it exposes only the queries
// the Nexus Linux engine actually needs (find-by-name, find-by-role,
// invoke action, insert text). Callers who want richer a11y tree
// inspection should use the godbus/dbus/v5 package directly.
type ATSPIBackend struct {
	mu   sync.Mutex
	conn *dbus.Conn
}

// NewATSPIBackend returns a backend lazily connected to the session
// bus. The connection is established on first use so tests that mock
// the runner never open a DBus socket.
func NewATSPIBackend() *ATSPIBackend { return &ATSPIBackend{} }

// Close releases the underlying DBus connection, if any.
func (b *ATSPIBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn == nil {
		return nil
	}
	err := b.conn.Close()
	b.conn = nil
	return err
}

// connect lazily opens the session bus.
func (b *ATSPIBackend) connect() (*dbus.Conn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil {
		return b.conn, nil
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("atspi: session bus: %w", err)
	}
	b.conn = conn
	return conn, nil
}

// FindByName asks the AT-SPI Registry for the first accessible whose
// role-name or accessible-name matches target. Returns the DBus
// object path as the element handle.
func (b *ATSPIBackend) FindByName(ctx context.Context, process, target string) (string, error) {
	_ = ctx
	conn, err := b.connect()
	if err != nil {
		return "", err
	}
	obj := conn.Object("org.a11y.atspi.Registry", dbus.ObjectPath("/org/a11y/atspi/registry"))
	var handle dbus.ObjectPath
	if err := obj.Call("org.a11y.atspi.Registry.FindByName", 0, process, target).Store(&handle); err != nil {
		return "", fmt.Errorf("atspi find-by-name: %w", err)
	}
	return string(handle), nil
}

// DoAction invokes the named action ("click", "press", etc.) on the
// element identified by handle.
func (b *ATSPIBackend) DoAction(ctx context.Context, handle, action string) error {
	_ = ctx
	conn, err := b.connect()
	if err != nil {
		return err
	}
	obj := conn.Object("org.a11y.atspi.Registry", dbus.ObjectPath(handle))
	if err := obj.Call("org.a11y.atspi.Action.DoActionByName", 0, action).Store(); err != nil {
		return fmt.Errorf("atspi do-action: %w", err)
	}
	return nil
}

// InsertText uses AT-SPI's EditableText interface to send text to the
// currently focused element of process.
func (b *ATSPIBackend) InsertText(ctx context.Context, process, text string) error {
	_ = ctx
	conn, err := b.connect()
	if err != nil {
		return err
	}
	obj := conn.Object("org.a11y.atspi.Registry", dbus.ObjectPath("/org/a11y/atspi/registry"))
	if err := obj.Call("org.a11y.atspi.EditableText.InsertText", 0, process, text).Store(); err != nil {
		return fmt.Errorf("atspi insert-text: %w", err)
	}
	return nil
}

// WithATSPIBackend lets operators opt into the native AT-SPI path on
// LinuxEngine. When set, Click / FindByName / Type bypass the shell
// helpers and talk to DBus directly. Returns the receiver so calls
// chain with the other `With*` methods.
func (l *LinuxEngine) WithATSPIBackend(b *ATSPIBackend) *LinuxEngine {
	l.commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		switch name {
		case "atspi-find":
			return l.atspiFind(ctx, b, args...)
		case "atspi-action":
			return l.atspiAction(ctx, b, args...)
		case "atspi-type":
			return l.atspiType(ctx, b, args...)
		}
		return defaultCommandRunner(ctx, name, args...)
	}
	return l
}

func (l *LinuxEngine) atspiFind(ctx context.Context, b *ATSPIBackend, args ...string) ([]byte, error) {
	flags := parseFlags(args)
	name := flags["--name"]
	process := flags["--process"]
	if name == "" || process == "" {
		return nil, fmt.Errorf("atspi-find: --name and --process required")
	}
	handle, err := b.FindByName(ctx, process, name)
	if err != nil {
		return nil, err
	}
	return []byte(handle), nil
}

func (l *LinuxEngine) atspiAction(ctx context.Context, b *ATSPIBackend, args ...string) ([]byte, error) {
	flags := parseFlags(args)
	handle := flags["--handle"]
	action := flags["--action"]
	if handle == "" {
		return nil, fmt.Errorf("atspi-action: --handle required")
	}
	if action == "" {
		action = "click"
	}
	return nil, b.DoAction(ctx, handle, action)
}

func (l *LinuxEngine) atspiType(ctx context.Context, b *ATSPIBackend, args ...string) ([]byte, error) {
	flags := parseFlags(args)
	process := flags["--process"]
	text := flags["--text"]
	if process == "" || text == "" {
		return nil, fmt.Errorf("atspi-type: --process and --text required")
	}
	return nil, b.InsertText(ctx, process, text)
}

func parseFlags(args []string) map[string]string {
	out := map[string]string{}
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "" || args[i][0] != '-' {
			continue
		}
		out[args[i]] = args[i+1]
		i++
	}
	return out
}
