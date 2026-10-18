// Package sessiontools is the session group of sysml-mcp tools: named REPL
// sessions that keep a model, its instances, a running action or state machine
// and the solver's state across calls, as the sysml prompt does.
package sessiontools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// Factory makes a fresh REPL session, configured as the command starting the
// server configures one.
type Factory func() (*repl.Session, error)

// DefaultSession is the session a tool uses when the call names none.
const DefaultSession = "default"

// Manager holds the sessions by name; the default one is made on first use.
type Manager struct {
	factory  Factory
	mu       sync.Mutex
	sessions map[string]*repl.Session
}

// NewManager returns a manager with no sessions open.
func NewManager(factory Factory) *Manager {
	return &Manager{factory: factory, sessions: map[string]*repl.Session{}}
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
	m.sessions[name] = sess
	return name, nil
}

// Get returns the session called name, opening the default session on first
// use; any other name must have been opened.
func (m *Manager) Get(name string) (*repl.Session, error) {
	if name == "" {
		name = DefaultSession
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[name]; ok {
		return sess, nil
	}
	if name != DefaultSession {
		return nil, fmt.Errorf("%w: %q", ErrNoSession, name)
	}
	sess, err := m.factory()
	if err != nil {
		return nil, err
	}
	m.sessions[name] = sess
	return sess, nil
}

// Close ends the session called name, releasing what it holds.
func (m *Manager) Close(name string) error {
	if name == "" {
		name = DefaultSession
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[name]
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoSession, name)
	}
	sess.Clear()
	delete(m.sessions, name)
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
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, sess := range m.sessions {
		sess.Clear()
		delete(m.sessions, name)
	}
}

// SessionName names a session; empty is the default session.
type SessionName struct {
	Session string `json:"session,omitempty" jsonschema:"session name; omit for the default session"`
}

// SessionOpened reports the session a call opened or closed.
type SessionOpened struct {
	Session string `json:"session" jsonschema:"the session's name"`
}

// SessionListInput takes no arguments.
type SessionListInput struct{}

// SessionList is the open sessions.
type SessionList struct {
	Sessions []string `json:"sessions" jsonschema:"names of the open sessions, sorted"`
}

// Register adds the session tools to s over m.
func Register(s *mcp.Server, m *Manager) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_open",
		Description: "Open a named REPL session: an empty model that declarations, instances and runs accumulate into across calls. The default session opens itself on first use, so this is only needed for a second, independent session.",
		Annotations: toolkit.Mutating("Open session", false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SessionName) (*mcp.CallToolResult, SessionOpened, error) {
		name, err := m.Open(in.Session)
		return nil, SessionOpened{Session: name}, err
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_close",
		Description: "Close a REPL session, discarding its model, instances and any run in progress.",
		Annotations: toolkit.Mutating("Close session", false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SessionName) (*mcp.CallToolResult, SessionOpened, error) {
		name := in.Session
		if name == "" {
			name = DefaultSession
		}
		return nil, SessionOpened{Session: name}, m.Close(in.Session)
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_list",
		Description: "List the open REPL sessions.",
		Annotations: toolkit.ReadOnly("List sessions"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ SessionListInput) (*mcp.CallToolResult, SessionList, error) {
		return nil, SessionList{Sessions: m.Names()}, nil
	})
}
