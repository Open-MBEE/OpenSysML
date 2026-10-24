package mcp

import (
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
)

// serviceRPCs reads the methods of the sysml-grpc service from the proto
// descriptor, so the gate follows api/proto/sysml.proto without a hand list.
func serviceRPCs(t *testing.T) []string {
	t.Helper()
	services := pb.File_sysml_proto.Services()
	if services.Len() != 1 {
		t.Fatalf("sysml.proto declares %d services, want 1", services.Len())
	}
	methods := services.Get(0).Methods()
	var names []string
	for i := 0; i < methods.Len(); i++ {
		m := methods.Get(i)
		if m.IsStreamingClient() || m.IsStreamingServer() {
			t.Errorf("rpc %s streams; list it in catalog.RPCsWithoutTools with the reason", m.Name())
		}
		names = append(names, string(m.Name()))
	}
	return names
}

// Drift gate: every RPC of the service is served by a tool of the catalog or
// listed, with a reason, among the RPCs that must not be tools; and the
// catalog names no RPC the service lacks.
func TestEveryRPCIsServedByATool(t *testing.T) {
	rpcs := serviceRPCs(t)
	for _, rpc := range rpcs {
		_, served := catalog.ToolFor(rpc)
		why, excluded := catalog.RPCsWithoutTools[rpc]
		switch {
		case served && excluded:
			t.Errorf("rpc %s has a tool and is also listed in catalog.RPCsWithoutTools (%s)", rpc, why)
		case !served && !excluded:
			t.Errorf("rpc %s has no tool: add one to internal/frontend/mcp/runtimetools and list it in internal/frontend/mcp/catalog, or list the RPC in catalog.RPCsWithoutTools with the reason", rpc)
		case excluded && strings.TrimSpace(why) == "":
			t.Errorf("rpc %s is listed in catalog.RPCsWithoutTools without a reason", rpc)
		}
	}
	for _, e := range catalog.Tools {
		for _, rpc := range e.RPCs {
			if !slices.Contains(rpcs, rpc) {
				t.Errorf("catalog tool %s names rpc %s, which sysml.proto does not declare", e.Name, rpc)
			}
		}
	}
	for rpc := range catalog.RPCsWithoutTools {
		if !slices.Contains(rpcs, rpc) {
			t.Errorf("catalog.RPCsWithoutTools names rpc %s, which sysml.proto does not declare", rpc)
		}
	}
}

// Drift gate: the tools a server registers for a group are exactly the
// catalog's entries for that group, so a tool added, renamed or removed in
// code fails here until the catalog follows (and the other way round).
func TestRegisteredToolsMatchTheCatalog(t *testing.T) {
	root := t.TempDir()
	for _, g := range catalog.Groups() {
		got := toolNames(t, connect(t, Config{Root: root, Groups: []Group{g}}))
		if want := catalog.Names(g); !slices.Equal(got, want) {
			t.Errorf("group %s registers %v\nthe catalog lists %v", g, got, want)
		}
	}
}

// Every entry of the catalog is well formed: unique name, a group the server
// serves (or none, for tool_catalog), a one-line summary, no RPC or flag named
// twice.
func TestTheCatalogIsWellFormed(t *testing.T) {
	seenName := map[string]bool{}
	seenRPC := map[string]string{}
	for _, e := range catalog.Tools {
		if seenName[e.Name] {
			t.Errorf("tool %s is listed twice", e.Name)
		}
		seenName[e.Name] = true
		if e.Group != "" && !slices.Contains(catalog.Groups(), e.Group) {
			t.Errorf("tool %s names group %q, which the server does not serve", e.Name, e.Group)
		}
		if strings.TrimSpace(e.Summary) == "" || strings.Contains(e.Summary, "\n") || !strings.HasSuffix(e.Summary, ".") {
			t.Errorf("tool %s needs a one-line summary ending in a period: %q", e.Name, e.Summary)
		}
		for _, rpc := range e.RPCs {
			if other, dup := seenRPC[rpc]; dup {
				t.Errorf("rpc %s is served by both %s and %s", rpc, other, e.Name)
			}
			seenRPC[rpc] = e.Name
		}
		for _, f := range e.Flags {
			if _, err := catalog.ClassifyFlag(f); err != nil {
				t.Error(err)
			}
		}
	}
	for flag := range catalog.Unserved {
		if _, err := catalog.ClassifyFlag(flag); err != nil {
			t.Error(err)
		}
	}
	for flag := range catalog.CLIOnly {
		if _, err := catalog.ClassifyFlag(flag); err != nil {
			t.Error(err)
		}
	}
}
