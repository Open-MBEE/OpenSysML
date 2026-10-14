package sessiontools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// Factory makes a fresh REPL session, configured as the command starting the
// server configures one.
type Factory func() (*repl.Session, error)

// DefaultSession is the session a tool uses when the call names none.
const DefaultSession = "default"

// Manager holds the sessions by name; the default one is made on first use.
// Each session answers one call at a time: a second call on it waits for the
// first, and a call whose context ends interrupts the command it is running.
type Manager struct {
	root     string
	factory  Factory
	mu       sync.Mutex
	sessions map[string]*held
}

// held is one open session and the lock serializing the calls made on it.
// closed is set under that lock when the session is closed, so a call that
// was waiting for the lock finds the session gone rather than cleared; it is
// atomic so a lookup can read it without the lock, from inside a call too.
type held struct {
	name   string
	mu     sync.Mutex
	sess   *repl.Session
	closed atomic.Bool
}

// NewManager returns a manager with no sessions open; root is the workspace
// folder relative paths in tool arguments are resolved against.
func NewManager(root string, factory Factory) *Manager {
	return &Manager{root: root, factory: factory, sessions: map[string]*held{}}
}

// ErrNoSession reports a name no open session has.
var ErrNoSession = errors.New("no such session")

// Open makes a session under name; an empty name is the default session. It
// fails if the name is taken.
func (m *Manager) Open(name string) (string, error) {
	if name == "" {
		name = DefaultSession
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.sessions[name]; taken {
		return name, fmt.Errorf("session %q is already open", name)
	}
	sess, err := m.factory()
	if err != nil {
		return name, err
	}
	m.sessions[name] = &held{name: name, sess: sess}
	return name, nil
}

// Get returns the session called name, opening the default session on first
// use; any other name must have been opened and not closed since. It does not
// wait for the calls on the session, so it can be made from inside one; Use is
// how a caller has the session to itself.
func (m *Manager) Get(name string) (*repl.Session, error) {
	h, err := m.get(name)
	if err != nil {
		return nil, err
	}
	if h.closed.Load() {
		return nil, h.closedErr()
	}
	return h.sess, nil
}

// closedErr reports a session closed while a call waited for it.
func (h *held) closedErr() error {
	return fmt.Errorf("%w: %q was closed while the call waited for it", ErrNoSession, h.name)
}

func (m *Manager) get(name string) (*held, error) {
	if name == "" {
		name = DefaultSession
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.sessions[name]; ok {
		return h, nil
	}
	if name != DefaultSession {
		return nil, fmt.Errorf("%w: %q (session_list names the open ones; session_open opens another)", ErrNoSession, name)
	}
	sess, err := m.factory()
	if err != nil {
		return nil, err
	}
	h := &held{name: name, sess: sess}
	m.sessions[name] = h
	return h, nil
}

// Use runs fn on the session called name, after any call already running on
// it has finished; a session closed meanwhile is ErrNoSession. A ctx that ends
// while fn runs interrupts the session's command under way, so a run abandoned
// by the client stops at its next step.
func (m *Manager) Use(ctx context.Context, name string, fn func(*repl.Session) error) error {
	h, err := m.get(name)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed.Load() {
		return h.closedErr()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Expected before the callback is armed, so a cancellation that lands
	// before fn's first command begins still stops that command.
	defer h.sess.Expecting()()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		h.sess.Interrupt()
	})
	err = fn(h.sess)
	// A callback already under way finishes before the lock is released, or it
	// would interrupt the next call's command.
	if !stop() {
		<-interrupted
	}
	return err
}

// Close ends the session called name, releasing what it holds; a command it is
// running is interrupted first, and a call still waiting for the session fails
// rather than running on the cleared one.
func (m *Manager) Close(name string) error {
	if name == "" {
		name = DefaultSession
	}
	m.mu.Lock()
	h, ok := m.sessions[name]
	if ok {
		delete(m.sessions, name)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoSession, name)
	}
	h.sess.Interrupt()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed.Store(true)
	h.sess.Clear()
	return nil
}

// Names lists the open sessions, sorted.
func (m *Manager) Names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.sessions))
	for name := range m.sessions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CloseAll ends every session, as the server does on exit.
func (m *Manager) CloseAll() {
	for _, name := range m.Names() {
		_ = m.Close(name)
	}
}

// Resolve makes a path absolute: against the workspace root unless it is
// absolute already or under the user's home (`~`), which is expanded, so the
// path answered names the file touched. Standard input is no file a tool can
// read, so a lone "-" is refused.
func (m *Manager) Resolve(path string) (string, error) {
	path = strings.TrimSpace(path)
	switch {
	case path == "":
		return "", errors.New("a path is required")
	case path == "-":
		return "", errors.New("standard input is not available to a tool; name a file")
	}
	path = repl.ExpandHome(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.root, path)
	}
	return filepath.Clean(path), nil
}
