// Package mcp serves OpenSysML to coding agents over the Model Context
// Protocol: one server over a workspace root, its tools in three groups. The
// workspace group is the language server (diagnostics, navigation, editing,
// rendering), the runtime group is the one-shot service (evaluate, instantiate,
// execute, verify, query, convert), and the session group is named REPL sessions
// whose model and runs persist across calls.
package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/lsptools"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/runtimetools"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/sessiontools"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Group names a set of tools a server registers together.
type Group = catalog.Group

// The groups a server can serve; see the catalog for the tools of each.
const (
	GroupLSP     = catalog.GroupLSP
	GroupRuntime = catalog.GroupRuntime
	GroupSession = catalog.GroupSession
)

// AllGroups is every group, the default selection.
var AllGroups = []Group{GroupLSP, GroupRuntime, GroupSession}

// ParseGroups reads a comma-separated group list as the -tools flag takes it;
// "all" and the empty string are every group.
func ParseGroups(spec string) ([]Group, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "all" {
		return slices.Clone(AllGroups), nil
	}
	var groups []Group
	for _, name := range strings.Split(spec, ",") {
		g := Group(strings.TrimSpace(name))
		if !slices.Contains(AllGroups, g) {
			return nil, fmt.Errorf("unknown tool group %q; the groups are lsp, runtime and session", name)
		}
		if !slices.Contains(groups, g) {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

// Config is what a server is built from.
type Config struct {
	// Root is the workspace folder the language server indexes.
	Root string
	// Groups are the tool groups to register; nil is every group, and a group
	// named twice is registered once.
	Groups []Group
	// Version is what the server reports as its version.
	Version string
	// Workspace is the language server's workspace; nil makes a default one.
	Workspace *model.Workspace
	// CacheSize bounds the runtime service's parsed-model cache; 0 is the
	// service's default of 100.
	CacheSize int
	// Records is the record cache the runtime service keeps the analysis of the
	// models it parses in; nil keeps none.
	Records *libs.Cache
	// UnavailableCapabilities are capabilities the runtime service deliberately
	// lacks, for conformance testing of a client's fallbacks; nil lacks none.
	UnavailableCapabilities []string
	// NewSession makes a REPL session for the session group.
	NewSession sessiontools.Factory
	// Logger, if set, receives the server's log of protocol activity.
	Logger *slog.Logger
	// Instructions is what the server tells a client about using it.
	Instructions string
}

// Server is an MCP server over the OpenSysML surfaces its groups need.
type Server struct {
	sdk      *sdk.Server
	logger   *slog.Logger
	served   map[string]catalog.Entry
	lsp      *lsptools.Host
	runtime  *sysmlgrpc.Service
	sessions *sessiontools.Manager
	// prewarmed closes when the runtime's library index is built; closed from
	// the start when there is no runtime group.
	prewarmed <-chan struct{}

	cfg    Config
	groups []Group
	// mu guards tenants: the protocol server and workspace of each principal other
	// than the operator's, keyed by guard.Principal.ID and made on its first session.
	mu      sync.Mutex
	tenants map[string]*tenant
}

// tenant is one principal's protocol server over its own language server
// workspace, so a document it holds open is no other principal's to read.
type tenant struct {
	sdk *sdk.Server
	lsp *lsptools.Host
}

// MaxPrincipalWorkspaces bounds the principals holding a workspace of their
// own at once; a further principal's session is refused until the server restarts.
const MaxPrincipalWorkspaces = 32

// nothingToWait is Prewarmed for a server without the runtime group.
var nothingToWait = func() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}()

// Prewarmed returns the channel that closes once the runtime's library index is
// built, when a first runtime call does not pay for it; closed without the runtime group.
func (s *Server) Prewarmed() <-chan struct{} { return s.prewarmed }

// New builds a server from cfg, indexing the workspace and preparing the
// runtime before it returns, so a client's first call is answered in full.
func New(ctx context.Context, cfg Config) (*Server, error) {
	groups := cfg.Groups
	if groups == nil {
		groups = AllGroups
	}
	groups = slices.Compact(slices.Sorted(slices.Values(groups)))
	s := &Server{logger: cfg.Logger, prewarmed: nothingToWait, cfg: cfg, groups: groups, tenants: map[string]*tenant{}}
	s.served = map[string]catalog.Entry{}
	for _, e := range catalog.Entries(groups...) {
		s.served[e.Name] = e
	}

	for _, g := range groups {
		switch g {
		case GroupLSP:
		case GroupRuntime:
			size := cfg.CacheSize
			if size == 0 {
				size = 100
			}
			var svc *sysmlgrpc.Service
			var err error
			if len(cfg.UnavailableCapabilities) == 0 {
				svc, err = sysmlgrpc.NewService(size, cfg.Version, sysmlgrpc.WithRecordCache(cfg.Records))
			} else {
				svc, err = sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(size, cfg.Version, cfg.UnavailableCapabilities,
					sysmlgrpc.WithRecordCache(cfg.Records))
			}
			if err != nil {
				s.Close(ctx)
				return nil, fmt.Errorf("starting the runtime service: %w", err)
			}
			s.prewarmed = svc.Prewarm()
			s.runtime = svc
		case GroupSession:
			if cfg.NewSession == nil {
				s.Close(ctx)
				return nil, fmt.Errorf("the session group needs a session factory")
			}
			sessions, err := sessiontools.NewManager(cfg.Root, cfg.NewSession)
			if err != nil {
				s.Close(ctx)
				return nil, err
			}
			s.sessions = sessions
		default:
			s.Close(ctx)
			return nil, fmt.Errorf("unknown tool group %q", g)
		}
	}
	ws := cfg.Workspace
	if ws == nil {
		ws = model.NewWorkspace()
	}
	own, err := s.build(ctx, ws)
	if err != nil {
		s.Close(ctx)
		return nil, err
	}
	s.sdk, s.lsp = own.sdk, own.lsp
	return s, nil
}

// build is a protocol server with the groups' tools registered over the server's
// runtime service and sessions, and over a language server of its own on ws.
func (s *Server) build(ctx context.Context, ws *model.Workspace) (*tenant, error) {
	t := &tenant{sdk: sdk.NewServer(&sdk.Implementation{
		Name:       "sysml-mcp",
		Title:      "OpenSysML",
		Version:    s.cfg.Version,
		WebsiteURL: "https://github.com/Open-MBEE/OpenSysML",
	}, &sdk.ServerOptions{Logger: s.cfg.Logger, Instructions: s.cfg.Instructions})}
	t.sdk.AddReceivingMiddleware(s.authorize)
	for _, g := range s.groups {
		switch g {
		case GroupLSP:
			host, err := lsptools.NewHost(ctx, s.cfg.Root, ws, s.cfg.Version)
			if err != nil {
				return nil, err
			}
			t.lsp = host
			lsptools.Register(t.sdk, host)
		case GroupRuntime:
			if err := runtimetools.Register(t.sdk, s.runtime, s.cfg.Root); err != nil {
				t.close(ctx)
				return nil, err
			}
		case GroupSession:
			sessiontools.Register(t.sdk, s.sessions)
		}
	}
	registerCatalog(t.sdk, s.groups)
	return t, nil
}

func (t *tenant) close(ctx context.Context) {
	if t.lsp != nil {
		_ = t.lsp.Close(ctx)
	}
}

// operates reports whether p is the operator's own principal, served by the server built
// at start: the local caller, or the static token's holder; one transport has only one.
func operates(p guard.Principal) bool {
	return p.Source == toolkit.LocalSource || p.Source == guard.SourceStaticToken
}

// serverFor is the protocol server for p's new session: with the workspace group, one per
// authenticated principal, made on first use; nil when it cannot be indexed or the bound is met.
func (s *Server) serverFor(ctx context.Context, p guard.Principal) *sdk.Server {
	if s.lsp == nil || operates(p) {
		return s.sdk
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tenants[p.ID()]; ok {
		return t.sdk
	}
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	if len(s.tenants) >= MaxPrincipalWorkspaces {
		logger.Warn("refusing a session: the bound on principals holding a workspace is reached", "principal", p.Subject, "source", p.Source, "bound", MaxPrincipalWorkspaces)
		return nil
	}
	t, err := s.build(context.WithoutCancel(ctx), model.NewWorkspace())
	if err != nil {
		logger.Error("refusing a session: the principal's workspace could not be indexed", "principal", p.Subject, "source", p.Source, "error", err)
		return nil
	}
	s.tenants[p.ID()] = t
	return t.sdk
}

// protocolServers are the protocol servers built so far: the operator's, then
// each other principal's.
func (s *Server) protocolServers() []*sdk.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	servers := []*sdk.Server{s.sdk}
	for _, t := range s.tenants {
		servers = append(servers, t.sdk)
	}
	return servers
}

// Run serves one client over t until the client ends the session or ctx is
// cancelled; ServeHTTP serves many over a listener.
func (s *Server) Run(ctx context.Context, t sdk.Transport) error {
	return s.sdk.Run(ctx, t)
}

// Close releases what the groups hold: the language server session, the
// runtime service and every REPL session.
func (s *Server) Close(ctx context.Context) {
	if s.sessions != nil {
		s.sessions.CloseAll()
	}
	if s.runtime != nil {
		s.runtime.Close()
	}
	if s.lsp != nil {
		_ = s.lsp.Close(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.tenants {
		t.close(ctx)
		delete(s.tenants, id)
	}
}
