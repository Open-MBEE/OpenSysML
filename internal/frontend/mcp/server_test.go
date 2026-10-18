package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// connect builds a server from cfg and a client talking to it in memory.
func connect(t *testing.T, cfg Config) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	if cfg.NewSession == nil {
		cfg.NewSession = func() (*repl.Session, error) { return repl.NewSession(), nil }
	}
	srv, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { srv.Close(ctx) })
	serverT, clientT := sdk.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverT) }()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *sdk.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil {
			t.Errorf("tool %s declares no annotations", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("tool %s has no description", tool.Name)
		}
	}
	slices.Sort(names)
	return names
}

func call(t *testing.T, cs *sdk.ClientSession, name string, args any, out any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if out != nil && !res.IsError {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("%s: marshal structured content: %v", name, err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s: structured content %s: %v", name, raw, err)
		}
	}
	return res
}

func writeModel(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTheServerRegistersTheGroupsAskedFor(t *testing.T) {
	root := t.TempDir()
	all := catalog.Names(GroupLSP, GroupRuntime, GroupSession)
	cases := []struct {
		groups []Group
		want   []string
	}{
		{nil, all},
		{[]Group{GroupLSP}, catalog.Names(GroupLSP)},
		{[]Group{GroupRuntime}, catalog.Names(GroupRuntime)},
		{[]Group{GroupSession}, catalog.Names(GroupSession)},
		{[]Group{GroupSession, GroupRuntime, GroupSession}, catalog.Names(GroupRuntime, GroupSession)},
	}
	if len(all) != len(catalog.Tools) {
		t.Fatalf("the catalog has %d tools but the three groups serve %d", len(catalog.Tools), len(all))
	}
	for _, tc := range cases {
		cs := connect(t, Config{Root: root, Groups: tc.groups, Version: "test"})
		if got := toolNames(t, cs); !slices.Equal(got, tc.want) {
			t.Errorf("groups %v: tools = %v, want %v", tc.groups, got, tc.want)
		}
		if info := cs.InitializeResult().ServerInfo; info.Name != "sysml-mcp" || info.Version != "test" {
			t.Errorf("server info = %+v", info)
		}
	}
}

func TestWorkspaceInfoListsTheIndexedDocuments(t *testing.T) {
	root := t.TempDir()
	writeModel(t, root, "a.sysml", "package A { part def P; }\n")
	writeModel(t, root, "b.kerml", "package B { class C; }\n")
	cs := connect(t, Config{Root: root, Groups: []Group{GroupLSP}, Version: "test"})

	var info struct {
		Root      string   `json:"root"`
		Documents []string `json:"documents"`
	}
	if res := call(t, cs, "workspace_info", map[string]any{}, &info); res.IsError {
		t.Fatalf("workspace_info failed: %+v", res.Content)
	}
	if info.Root != root {
		t.Errorf("root = %q, want %q", info.Root, root)
	}
	want := []string{filepath.Join(root, "a.sysml"), filepath.Join(root, "b.kerml")}
	if !slices.Equal(info.Documents, want) {
		t.Errorf("documents = %v, want %v", info.Documents, want)
	}

	// Files the agent creates or deletes with its own tools, with no watcher to
	// report them, show up on the next call.
	writeModel(t, root, "c.sysml", "package C;\n")
	if err := os.Remove(filepath.Join(root, "a.sysml")); err != nil {
		t.Fatal(err)
	}
	call(t, cs, "workspace_info", map[string]any{}, &info)
	want = []string{filepath.Join(root, "b.kerml"), filepath.Join(root, "c.sysml")}
	if !slices.Equal(info.Documents, want) {
		t.Errorf("documents after edits = %v, want %v", info.Documents, want)
	}
}

func TestServerInfoReportsTheRuntimeCapabilities(t *testing.T) {
	cs := connect(t, Config{Root: t.TempDir(), Groups: []Group{GroupRuntime}, Version: "1.2.3"})
	var info struct {
		Version      string   `json:"version"`
		Capabilities []string `json:"capabilities"`
	}
	if res := call(t, cs, "server_info", map[string]any{}, &info); res.IsError {
		t.Fatalf("server_info failed: %+v", res.Content)
	}
	if info.Version != "1.2.3" {
		t.Errorf("version = %q", info.Version)
	}
	if !slices.Contains(info.Capabilities, "parse_sources") {
		t.Errorf("capabilities = %v, want parse_sources among them", info.Capabilities)
	}
}

func TestSessionsOpenListAndClose(t *testing.T) {
	cs := connect(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"})
	var list struct {
		Sessions []string `json:"sessions"`
	}
	call(t, cs, "session_list", map[string]any{}, &list)
	if len(list.Sessions) != 0 {
		t.Fatalf("sessions before any open = %v", list.Sessions)
	}
	var opened struct {
		Session string `json:"session"`
	}
	call(t, cs, "session_open", map[string]any{}, &opened)
	if opened.Session != "default" {
		t.Errorf("opened %q, want default", opened.Session)
	}
	call(t, cs, "session_open", map[string]any{"session": "scratch"}, &opened)
	if opened.Session != "scratch" {
		t.Errorf("opened %q, want scratch", opened.Session)
	}
	if res := call(t, cs, "session_open", map[string]any{"session": "scratch"}, nil); !res.IsError {
		t.Error("opening a session twice succeeded")
	}
	call(t, cs, "session_list", map[string]any{}, &list)
	if !slices.Equal(list.Sessions, []string{"default", "scratch"}) {
		t.Errorf("sessions = %v", list.Sessions)
	}
	if res := call(t, cs, "session_close", map[string]any{"session": "scratch"}, nil); res.IsError {
		t.Errorf("session_close failed: %+v", res.Content)
	}
	if res := call(t, cs, "session_close", map[string]any{"session": "scratch"}, nil); !res.IsError {
		t.Error("closing a closed session succeeded")
	}
	call(t, cs, "session_list", map[string]any{}, &list)
	if !slices.Equal(list.Sessions, []string{"default"}) {
		t.Errorf("sessions = %v", list.Sessions)
	}
}

func TestParseGroups(t *testing.T) {
	for spec, want := range map[string][]Group{
		"":                 AllGroups,
		"all":              AllGroups,
		"lsp":              {GroupLSP},
		"session, runtime": {GroupSession, GroupRuntime},
		"lsp,lsp":          {GroupLSP},
	} {
		got, err := ParseGroups(spec)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("ParseGroups(%q) = %v, %v; want %v", spec, got, err, want)
		}
	}
	if _, err := ParseGroups("lsp,debugger"); err == nil {
		t.Error("ParseGroups accepted an unknown group")
	}
}

// With every group on, the catalogue is the sum of the groups' catalogues
// plus tool_catalog, which every server serves once: the SDK replaces a
// same-name tool silently, so two groups must never share a name.
func TestGroupsShareNoToolName(t *testing.T) {
	sum := 0
	for _, g := range []Group{GroupLSP, GroupRuntime, GroupSession} {
		cs := connect(t, Config{Root: t.TempDir(), Groups: []Group{g}, Version: "test"})
		names := toolNames(t, cs)
		if !slices.Contains(names, "tool_catalog") {
			t.Errorf("group %s serves no tool_catalog", g)
		}
		sum += len(names) - 1
	}
	cs := connect(t, Config{Root: t.TempDir(), Groups: []Group{GroupLSP, GroupRuntime, GroupSession}, Version: "test"})
	if all := len(toolNames(t, cs)); all != sum+1 {
		t.Fatalf("all groups register %d tools, the groups alone %d plus tool_catalog: a name is shared", all, sum)
	}
}

func TestToolCatalogOverNoGroupsListsNoGroup(t *testing.T) {
	cs := connect(t, Config{Root: t.TempDir(), Groups: []Group{}})
	if names := toolNames(t, cs); !slices.Equal(names, []string{"tool_catalog"}) {
		t.Fatalf("no groups serve %v", names)
	}
	var out CatalogOutput
	if res := call(t, cs, "tool_catalog", map[string]any{}, &out); res.IsError {
		t.Fatalf("tool_catalog: %v", res.Content)
	}
	if len(out.Groups) != 0 || len(out.Tools) != 1 || out.Tools[0].Name != "tool_catalog" {
		t.Errorf("groups %q, tools %v", out.Groups, out.Tools)
	}
}
