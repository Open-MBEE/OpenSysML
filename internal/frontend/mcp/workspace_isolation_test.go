package mcp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// editors are two principals who read and write the workspace, and one who
// holds only the session scope.
var editors = BearerToken{Authenticator: tokens{
	"key-alice":   {Subject: "alice", Source: "api-key", Scopes: []string{string(catalog.ScopeRead), string(catalog.ScopeWrite)}},
	"key-alice-2": {Subject: "alice", Source: "api-key", Scopes: []string{string(catalog.ScopeRead), string(catalog.ScopeWrite)}},
	"key-bob":     {Subject: "bob", Source: "api-key", Scopes: []string{string(catalog.ScopeRead), string(catalog.ScopeWrite)}},
	"key-carol":   {Subject: "carol", Source: "api-key", Scopes: []string{string(catalog.ScopeSession)}},
}}

// readWorkspace reads a workspace document as a resource.
func readWorkspace(cs *sdk.ClientSession, rel string) (string, error) {
	res, err := cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "sysml://workspace/" + rel})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range res.Contents {
		b.WriteString(c.Text)
	}
	return b.String(), nil
}

// TestTwoPrincipalsDoNotShareOpenBuffers checks a document one principal holds open is
// read by that principal alone, by resource and by tool: another reads the disk.
func TestTwoPrincipalsDoNotShareOpenBuffers(t *testing.T) {
	root := t.TempDir()
	const onDisk = "package Shared;\n"
	if err := os.WriteFile(filepath.Join(root, "shared.sysml"), []byte(onDisk), 0o600); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	endpoint, stop := serveHTTPWith(t, Config{Root: root, Groups: []Group{GroupLSP}, Version: "test"}, &logs, "127.0.0.1:0", editors)
	alice, bob := connectAs(t, endpoint, "key-alice"), connectAs(t, endpoint, "key-bob")

	const unsaved = "package Shared { part def AliceOnlyDraft; }\n"
	const scratch = "package Scratch { part def AliceOnlyScratch; }\n"
	for path, text := range map[string]string{"shared.sysml": unsaved, "scratch.sysml": scratch} {
		if res := call(t, alice, "open_document", map[string]any{"path": path, "text": text}, nil); res.IsError {
			t.Fatalf("alice open_document %s: %s", path, resultText(res))
		}
	}

	// The opener reads its buffers, in this session and in another of its own.
	for name, cs := range map[string]*sdk.ClientSession{"alice": alice, "alice's second session": connectAs(t, endpoint, "key-alice-2")} {
		if got, err := readWorkspace(cs, "shared.sysml"); err != nil || got != unsaved {
			t.Errorf("%s reads shared.sysml = %q, %v; want its open buffer", name, got, err)
		}
		if got, err := readWorkspace(cs, "scratch.sysml"); err != nil || got != scratch {
			t.Errorf("%s reads scratch.sysml = %q, %v; want its open buffer", name, got, err)
		}
	}
	// Another principal reads the disk.
	if got, err := readWorkspace(bob, "shared.sysml"); err != nil || got != onDisk {
		t.Errorf("bob reads shared.sysml = %q, %v; want the file on disk", got, err)
	}
	if got, err := readWorkspace(bob, "scratch.sysml"); err == nil {
		t.Errorf("bob reads alice's unsaved scratch.sysml: %q", got)
	}
	res, err := bob.CallTool(context.Background(), &sdk.CallToolParams{Name: "workspace_symbols", Arguments: map[string]any{"query": "AliceOnly"}})
	if err != nil {
		t.Fatal(err)
	}
	if text := resultText(res); strings.Contains(text, "AliceOnly") {
		t.Errorf("bob's workspace_symbols finds alice's unsaved declarations: %s", text)
	}
	if raw, _ := res.StructuredContent.(map[string]any); strings.Contains(strings.Join(flatten(raw), " "), "AliceOnly") {
		t.Errorf("bob's workspace_symbols finds alice's unsaved declarations: %v", raw)
	}
	// And holds buffers of its own, which the first does not read.
	const bobs = "package Shared { part def BobOnlyDraft; }\n"
	if res := call(t, bob, "open_document", map[string]any{"path": "shared.sysml", "text": bobs}, nil); res.IsError {
		t.Fatalf("bob open_document: %s", resultText(res))
	}
	if got, err := readWorkspace(bob, "shared.sysml"); err != nil || got != bobs {
		t.Errorf("bob reads shared.sysml = %q, %v; want its open buffer", got, err)
	}
	if got, err := readWorkspace(alice, "shared.sysml"); err != nil || got != unsaved {
		t.Errorf("alice reads shared.sysml = %q, %v after bob opened it; want her open buffer", got, err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

// flatten is the strings anywhere in v.
func flatten(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, e := range v {
			out = append(out, flatten(e)...)
		}
		return out
	case map[string]any:
		var out []string
		for _, e := range v {
			out = append(out, flatten(e)...)
		}
		return out
	}
	return nil
}

// TestResourceReadNeedsTheReadScope checks a principal without sysml:read is refused
// a workspace or library document as a resource, and a holder of the scope served.
func TestResourceReadNeedsTheReadScope(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "shared.sysml"), []byte("package Shared;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	endpoint, stop := serveHTTPWith(t, Config{Root: root, Groups: []Group{GroupLSP}, Version: "test"}, &logs, "127.0.0.1:0", editors)
	carol := connectAs(t, endpoint, "key-carol")
	for _, uri := range []string{"sysml://workspace/shared.sysml", "sysml://library/ScalarValues.kerml"} {
		res, err := carol.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: uri})
		if scope, refused := toolkit.RefusedScope(err); !refused || scope != catalog.ScopeRead {
			t.Errorf("carol reads %s: %v, %v; want an insufficient_scope refusal naming %s", uri, res, err, catalog.ScopeRead)
		}
	}
	if err := carol.Subscribe(context.Background(), &sdk.SubscribeParams{URI: "sysml://workspace/shared.sysml"}); err == nil || !strings.Contains(err.Error(), "insufficient_scope") {
		t.Errorf("carol subscribes: %v; want an insufficient_scope refusal", err)
	}
	if got, err := readWorkspace(connectAs(t, endpoint, "key-bob"), "shared.sysml"); err != nil || got != "package Shared;\n" {
		t.Errorf("bob reads shared.sysml = %q, %v", got, err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

// TestOperatorPrincipalsKeepTheStartupWorkspace checks the local caller and the static
// token's holder are served by the startup server, and the other workspaces are bounded.
func TestOperatorPrincipalsKeepTheStartupWorkspace(t *testing.T) {
	ctx := context.Background()
	srv, err := New(ctx, Config{Root: t.TempDir(), Groups: []Group{GroupLSP}, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close(ctx) })
	for _, p := range []guard.Principal{toolkit.LocalPrincipal(), {Subject: "operator", Source: guard.SourceStaticToken}} {
		if got := srv.serverFor(ctx, p); got != srv.sdk {
			t.Errorf("%s is not served by the startup server", p.ID())
		}
	}
	alice := guard.Principal{Subject: "alice", Source: "api-key"}
	first := srv.serverFor(ctx, alice)
	if first == nil || first == srv.sdk {
		t.Fatalf("alice is served by %p, want a server of her own (startup %p)", first, srv.sdk)
	}
	if again := srv.serverFor(ctx, alice); again != first {
		t.Error("alice's second session is served by another server than her first")
	}
	if other := srv.serverFor(ctx, guard.Principal{Subject: "alice", Source: "oidc"}); other == nil || other == first || other == srv.sdk {
		t.Error("a namesake from another source shares a server")
	}
	srv.mu.Lock()
	for len(srv.tenants) < MaxPrincipalWorkspaces {
		srv.tenants[string(rune('a'+len(srv.tenants)))+":filler"] = &tenant{}
	}
	srv.mu.Unlock()
	if got := srv.serverFor(ctx, guard.Principal{Subject: "late", Source: "api-key"}); got != nil {
		t.Error("a principal past the bound is served")
	}
	if again := srv.serverFor(ctx, alice); again != first {
		t.Error("alice is refused at the bound, though she holds a workspace")
	}
	srv.mu.Lock()
	for id, tn := range srv.tenants {
		if tn.sdk == nil {
			delete(srv.tenants, id)
		}
	}
	srv.mu.Unlock()
}
