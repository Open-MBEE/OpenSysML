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

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/lsptools"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/runtimetools"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/sessiontools"
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
	lsp      *lsptools.Host
	runtime  *sysmlgrpc.Service
	sessions *sessiontools.Manager
}

// New builds a server from cfg, indexing the workspace and preparing the
// runtime before it returns, so a client's first call is answered in full.
func New(ctx context.Context, cfg Config) (*Server, error) {
	groups := cfg.Groups
	if groups == nil {
		groups = AllGroups
	}
	groups = slices.Compact(slices.Sorted(slices.Values(groups)))
	s := &Server{sdk: sdk.NewServer(&sdk.Implementation{
		Name:       "sysml-mcp",
		Title:      "OpenSysML",
		Version:    cfg.Version,
		WebsiteURL: "https://github.com/Open-MBEE/OpenSysML",
	}, &sdk.ServerOptions{Logger: cfg.Logger, Instructions: cfg.Instructions}), logger: cfg.Logger}

	for _, g := range groups {
		switch g {
		case GroupLSP:
			ws := cfg.Workspace
			if ws == nil {
				ws = model.NewWorkspace()
			}
			host, err := lsptools.NewHost(ctx, cfg.Root, ws, cfg.Version)
			if err != nil {
				s.Close(ctx)
				return nil, err
			}
			s.lsp = host
			lsptools.Register(s.sdk, host)
		case GroupRuntime:
			size := cfg.CacheSize
			if size == 0 {
				size = 100
			}
			var svc *sysmlgrpc.Service
			var err error
			if len(cfg.UnavailableCapabilities) == 0 {
				svc, err = sysmlgrpc.NewService(size, cfg.Version)
			} else {
				svc, err = sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(size, cfg.Version, cfg.UnavailableCapabilities)
			}
			if err != nil {
				s.Close(ctx)
				return nil, fmt.Errorf("starting the runtime service: %w", err)
			}
			svc.Prewarm()
			s.runtime = svc
			if err := runtimetools.Register(s.sdk, svc, cfg.Root); err != nil {
				s.Close(ctx)
				return nil, err
			}
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
			sessiontools.Register(s.sdk, s.sessions)
		default:
			s.Close(ctx)
			return nil, fmt.Errorf("unknown tool group %q", g)
		}
	}
	registerCatalog(s.sdk, groups)
	return s, nil
}

// Run serves one client over t until the client ends the session or ctx is
// cancelled; ServeStreamableHTTP serves many over a listener.
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
}
