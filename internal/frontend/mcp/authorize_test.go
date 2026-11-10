package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/all" // links %save, which the matrix refuses
)

// representative is one tool of each scope a principal can be granted, with
// arguments that succeed over an empty workspace.
var representative = map[catalog.Scope]struct {
	tool string
	args map[string]any
}{
	catalog.ScopeRead:    {"tool_catalog", map[string]any{}},
	catalog.ScopeWrite:   {"open_document", map[string]any{"path": "probe.sysml", "text": "package P;"}},
	catalog.ScopeExecute: {"session_eval", map[string]any{"expression": "1 + 1"}},
	catalog.ScopeSession: {"session_list", map[string]any{}},
}

// principalOnly is a principal of the given source and subject holding exactly
// the scopes given.
func principalOnly(source, subject string, scopes ...catalog.Scope) guard.Principal {
	p := guard.Principal{Subject: subject, Name: subject, Source: source}
	for _, s := range scopes {
		p.Scopes = append(p.Scopes, string(s))
	}
	return p
}

// serveAuthenticated serves cfg over HTTP to the holders of the given bearer
// tokens, each verified to its principal, and answers the endpoint and the log.
func serveAuthenticated(t *testing.T, cfg Config, principals map[string]guard.Principal) (string, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	cfg.NewSession = func() (*repl.Session, error) { return repl.NewSession(), nil }
	cfg.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { srv.Close(context.Background()) })
	verify := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		p, ok := principals[token]
		if !ok {
			return nil, auth.ErrInvalidToken
		}
		return TokenInfoFor(p), nil
	}
	require := auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})
	hs := httptest.NewServer(require(srv.protocolHandler(HTTPConfig{})))
	t.Cleanup(hs.Close)
	return hs.URL + HTTPPath, &logs
}

// bearer sends every request with the token.
type bearer string

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

// connectAs connects a client holding token to the endpoint.
func connectAs(t *testing.T, url, token string) *sdk.ClientSession {
	t.Helper()
	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	transport := &sdk.StreamableClientTransport{Endpoint: url, HTTPClient: &http.Client{Transport: bearer(token)}}
	cs, err := c.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("Connect as %s: %v", token, err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// resultText is the text a tool answered or failed with.
func resultText(res *sdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*sdk.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

// sessionList is session_list's answer.
type sessionList struct {
	Sessions []string `json:"sessions"`
	All      []struct{ Owner, Session string }
}

// listSessions is what cs sees of the sessions.
func listSessions(t *testing.T, cs *sdk.ClientSession) sessionList {
	t.Helper()
	var list sessionList
	call(t, cs, "session_list", map[string]any{}, &list)
	return list
}

// refusal is the insufficient_scope error of a call, or nil for any other outcome.
func refusal(err error) *jsonrpc.Error {
	var jerr *jsonrpc.Error
	if errors.As(err, &jerr) && jerr.Code == CodeInsufficientScope {
		return jerr
	}
	return nil
}

// Scope matrix: a principal holding one scope calls one tool of each scope; only
// its own answers, the rest are refused naming their scope; tools/list is never narrowed.
func TestEachScopeAdmitsExactlyItsTools(t *testing.T) {
	principals := map[string]guard.Principal{}
	for _, s := range catalog.Scopes() {
		principals[string(s)] = principalOnly("test", "holder-of-"+string(s), s)
	}
	url, _ := serveAuthenticated(t, Config{Root: t.TempDir(), Groups: []Group{GroupLSP, GroupSession}, Version: "test"}, principals)
	for _, held := range catalog.Scopes() {
		cs := connectAs(t, url, string(held))
		if got := toolNames(t, cs); !slices.Equal(got, catalog.Names(GroupLSP, GroupSession)) {
			t.Errorf("%s: tools/list answers %v, want both groups whole", held, got)
		}
		for _, scope := range slices.Sorted(func(yield func(catalog.Scope) bool) {
			for s := range representative {
				if !yield(s) {
					return
				}
			}
		}) {
			rep := representative[scope]
			entry, ok := catalog.Tool(rep.tool)
			if !ok || entry.Scope != scope {
				t.Fatalf("%s is catalogued with scope %s, the matrix expects %s", rep.tool, entry.Scope, scope)
			}
			res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: rep.tool, Arguments: rep.args})
			switch jerr := refusal(err); {
			case scope == held && err != nil:
				t.Errorf("%s holder calling %s: %v, want an answer", held, rep.tool, err)
			case scope == held && res.IsError:
				t.Errorf("%s holder calling %s: tool error %q, want an answer", held, rep.tool, resultText(res))
			case scope != held && jerr == nil:
				t.Errorf("%s holder calling %s: err %v, want the insufficient_scope refusal", held, rep.tool, err)
			case scope != held && (!strings.Contains(jerr.Message, string(scope)) || !strings.Contains(jerr.Message, rep.tool)):
				t.Errorf("%s holder calling %s: the refusal %q names neither the scope %s nor the tool", held, rep.tool, jerr.Message, scope)
			case scope != held:
				var data struct{ Tool, Scope string }
				if err := json.Unmarshal(jerr.Data, &data); err != nil || data.Tool != rep.tool || data.Scope != string(scope) {
					t.Errorf("%s holder calling %s: refusal data %s, want the tool and the scope", held, rep.tool, jerr.Data)
				}
			}
		}
	}
}

// A principal holding every scope, as the static token does, is refused nothing.
func TestEveryScopeAdmitsEveryTool(t *testing.T) {
	all := guard.Principal{Subject: guard.StaticTokenSubject, Scopes: catalog.ScopeNames(), Source: guard.SourceStaticToken}
	url, _ := serveAuthenticated(t, Config{Root: t.TempDir(), Groups: []Group{GroupLSP, GroupSession}, Version: "test"}, map[string]guard.Principal{"all": all})
	cs := connectAs(t, url, "all")
	for _, rep := range representative {
		res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: rep.tool, Arguments: rep.args})
		if err != nil || res.IsError {
			t.Errorf("%s: err %v, result %q", rep.tool, err, resultText(res))
		}
	}
}

// Session isolation: two principals' sessions of one name are two sessions,
// each lists only its own, and neither closes the other's.
func TestSessionsBelongToThePrincipalThatOpenedThem(t *testing.T) {
	principals := map[string]guard.Principal{
		"ann": principalOnly("oidc", "ann", catalog.ScopeSession, catalog.ScopeExecute),
		"bob": principalOnly("oidc", "bob", catalog.ScopeSession, catalog.ScopeExecute),
	}
	url, _ := serveAuthenticated(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, principals)
	ann, bob := connectAs(t, url, "ann"), connectAs(t, url, "bob")

	// The same names, opened by both; each one's default session is its own too.
	for _, cs := range []*sdk.ClientSession{ann, bob} {
		if res := call(t, cs, "session_open", map[string]any{"session": "work"}, nil); res.IsError {
			t.Fatalf("session_open: %v", res.Content)
		}
	}
	call(t, ann, "session_submit", map[string]any{"text": "attribute x = 1;"}, nil)
	call(t, bob, "session_submit", map[string]any{"text": "attribute x = 2;"}, nil)
	var value struct {
		Text string `json:"text"`
	}
	call(t, ann, "session_eval", map[string]any{"expression": "x"}, &value)
	if !strings.Contains(value.Text, "1") || strings.Contains(value.Text, "2") {
		t.Errorf("ann's default session evaluates x to %q, want her 1", value.Text)
	}
	call(t, bob, "session_eval", map[string]any{"expression": "x"}, &value)
	if !strings.Contains(value.Text, "2") || strings.Contains(value.Text, "1") {
		t.Errorf("bob's default session evaluates x to %q, want his 2", value.Text)
	}

	list := listSessions(t, ann)
	if !slices.Equal(list.Sessions, []string{"default", "work"}) || list.All != nil {
		t.Errorf("ann lists %+v, want her default and work and no one else's", list)
	}

	// Closing bob's session by name closes only bob's; ann cannot reach it by owner.
	if res := call(t, bob, "session_close", map[string]any{"session": "work"}, nil); res.IsError {
		t.Fatalf("bob closing work: %v", res.Content)
	}
	if list := listSessions(t, ann); !slices.Equal(list.Sessions, []string{"default", "work"}) {
		t.Errorf("after bob closed his work, ann lists %v, want hers untouched", list.Sessions)
	}
	_, err := bob.CallTool(context.Background(), &sdk.CallToolParams{Name: "session_close", Arguments: map[string]any{"session": "work", "owner": "oidc:ann"}})
	if refused, ok := toolkit.RefusedScope(err); !ok || refused != catalog.ScopeAdmin {
		t.Errorf("bob closing ann's work: %v, want the insufficient_scope refusal naming %s", err, catalog.ScopeAdmin)
	}
	if list := listSessions(t, ann); !slices.Equal(list.Sessions, []string{"default", "work"}) {
		t.Errorf("after bob was refused, ann lists %v, want hers untouched", list.Sessions)
	}
}

// The admin path: sysml:admin sees every principal's sessions with their
// owners and closes another's by owner, and its own sessions stay its own.
func TestAdminSeesAndClosesEveryonesSessions(t *testing.T) {
	principals := map[string]guard.Principal{
		"ann":   principalOnly("oidc", "ann", catalog.ScopeSession),
		"root":  principalOnly("api-key", "root", catalog.ScopeSession, catalog.ScopeAdmin),
		"noone": principalOnly("api-key", "noone", catalog.ScopeAdmin),
	}
	url, _ := serveAuthenticated(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, principals)
	ann, root := connectAs(t, url, "ann"), connectAs(t, url, "root")
	call(t, ann, "session_open", map[string]any{"session": "work"}, nil)
	call(t, root, "session_open", map[string]any{"session": "audit"}, nil)

	list := listSessions(t, root)
	if !slices.Equal(list.Sessions, []string{"audit"}) {
		t.Errorf("root's own sessions are %v, want [audit]", list.Sessions)
	}
	want := []struct{ Owner, Session string }{{"api-key:root", "audit"}, {"oidc:ann", "work"}}
	if !slices.Equal(list.All, want) {
		t.Errorf("root sees %v, want %v", list.All, want)
	}

	if res := call(t, root, "session_close", map[string]any{"session": "work", "owner": "oidc:ann"}, nil); res.IsError {
		t.Fatalf("root closing ann's work: %v", res.Content)
	}
	if list := listSessions(t, ann); len(list.Sessions) != 0 || list.All != nil {
		t.Errorf("after root closed her work, ann lists %+v, want nothing", list)
	}
	if list := listSessions(t, root); !slices.Equal(list.All, want[:1]) {
		t.Errorf("after closing ann's work, root sees %v, want only its own", list.All)
	}

	// sysml:admin widens session_list and session_close; it is no tool's scope.
	noone := connectAs(t, url, "noone")
	_, err := noone.CallTool(context.Background(), &sdk.CallToolParams{Name: "session_list", Arguments: map[string]any{}})
	if refusal(err) == nil {
		t.Errorf("sysml:admin alone calling session_list: %v, want the insufficient_scope refusal", err)
	}
}

// Audit: every tools/call is one info line with principal, source, remote
// address, tool, scope, outcome and duration, and never the credential or the arguments.
func TestEveryToolCallIsAudited(t *testing.T) {
	secret := "s3cret-token-value"
	principals := map[string]guard.Principal{
		secret: principalOnly("oidc", "ann", catalog.ScopeSession, catalog.ScopeExecute),
	}
	url, logs := serveAuthenticated(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, principals)
	ann := connectAs(t, url, secret)
	call(t, ann, "session_eval", map[string]any{"expression": "40 + 2"}, nil)
	if res := call(t, ann, "session_eval", map[string]any{"expression": ""}, nil); !res.IsError {
		t.Fatal("an empty expression evaluated")
	}
	if _, err := ann.CallTool(context.Background(), &sdk.CallToolParams{Name: "session_save", Arguments: map[string]any{"path": "never.sysml"}}); refusal(err) == nil {
		t.Fatalf("session_save without sysml:write: %v, want the refusal", err)
	}

	var lines []string
	for _, line := range strings.Split(logs.String(), "\n") {
		if strings.Contains(line, `msg=tools/call`) {
			lines = append(lines, line)
		}
	}
	if len(lines) != 3 {
		t.Fatalf("%d audit lines for 3 calls:\n%s", len(lines), logs.String())
	}
	for i, want := range []string{
		`tool=session_eval scope=sysml:execute outcome=ok`,
		`tool=session_eval scope=sysml:execute outcome=error`,
		`tool=session_save scope=sysml:write outcome=refused`,
	} {
		line := lines[i]
		for _, field := range []string{"level=INFO", "principal=ann", "source=oidc", "remote=127.0.0.1:", want, "duration="} {
			if !strings.Contains(line, field) {
				t.Errorf("audit line %d lacks %q:\n%s", i, field, line)
			}
		}
		for _, leak := range []string{secret, "40 + 2", "never.sysml", "expression"} {
			if strings.Contains(line, leak) {
				t.Errorf("audit line %d carries %q:\n%s", i, leak, line)
			}
		}
	}
}

// Over stdio nothing is authenticated: the one caller is the local principal
// with every scope, its sessions are the server's, and the audit line says so.
func TestStdioHasOneLocalPrincipalWithEveryScope(t *testing.T) {
	var logs bytes.Buffer
	cfg := Config{Root: t.TempDir(), Groups: []Group{GroupLSP, GroupSession}, Version: "test", Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))}
	cs := connect(t, cfg)
	for _, rep := range representative {
		res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: rep.tool, Arguments: rep.args})
		if err != nil || res.IsError {
			t.Errorf("%s over stdio: err %v, result %q", rep.tool, err, resultText(res))
		}
	}
	list := listSessions(t, cs)
	if !slices.Equal(list.Sessions, []string{"default"}) {
		t.Errorf("stdio lists %v, want [default]", list.Sessions)
	}
	want := []struct{ Owner, Session string }{{toolkit.LocalPrincipal().ID(), "default"}}
	if !slices.Equal(list.All, want) {
		t.Errorf("the local principal holds every scope, so it sees %v, want %v", list.All, want)
	}
	if log := logs.String(); !strings.Contains(log, "principal=local source=local remote=stdio tool=session_eval scope=sysml:execute outcome=ok") {
		t.Errorf("stdio audit log:\n%s", log)
	}
}

// The principal of a request is the one the transport verified, read whole
// from the SDK's token record, or the local one when nothing was verified.
func TestPrincipalOfReadsTheTransportsVerdict(t *testing.T) {
	ann := principalOnly("oidc", "ann", catalog.ScopeRead)
	info := TokenInfoFor(ann)
	if info.UserID != "oidc:ann" || !slices.Equal(info.Scopes, []string{"sysml:read"}) {
		t.Errorf("TokenInfoFor(ann) = %+v", info)
	}
	cases := []struct {
		name  string
		extra *sdk.RequestExtra
		want  guard.Principal
	}{
		{"no http request", nil, toolkit.LocalPrincipal()},
		{"http without a token", &sdk.RequestExtra{Header: http.Header{}}, toolkit.LocalPrincipal()},
		{"a verified principal", &sdk.RequestExtra{TokenInfo: info}, ann},
		{"a token verified by other code", &sdk.RequestExtra{TokenInfo: &auth.TokenInfo{UserID: "u1", Scopes: []string{"sysml:read"}}}, guard.Principal{Subject: "u1", Scopes: []string{"sysml:read"}, Source: "bearer"}},
	}
	for _, c := range cases {
		req := &sdk.ServerRequest[*sdk.CallToolParamsRaw]{Params: &sdk.CallToolParamsRaw{Name: "tool_catalog"}, Extra: c.extra}
		got := principalOf(req)
		if got.Subject != c.want.Subject || got.Source != c.want.Source || !slices.Equal(got.Scopes, c.want.Scopes) {
			t.Errorf("%s: principalOf = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// The static token's principal holds every scope and owns the sessions the
// token's clients open, and the audit line names it by subject and source.
func TestTheStaticTokenIsAPrincipalWithEveryScope(t *testing.T) {
	var logs bytes.Buffer
	tok, _ := NewBearerToken("s3cret-token")
	url, stop := serveHTTPWith(t, Config{Root: t.TempDir(), Groups: []Group{GroupLSP, GroupSession}, Version: "test"}, &logs, "127.0.0.1:0", tok)
	cs := connectAs(t, url, "s3cret-token")
	for _, rep := range representative {
		res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: rep.tool, Arguments: rep.args})
		if err != nil || res.IsError {
			t.Errorf("%s with the token: err %v, result %q", rep.tool, err, resultText(res))
		}
	}
	owner := guard.Principal{Subject: guard.StaticTokenSubject, Source: guard.SourceStaticToken}.ID()
	want := []struct{ Owner, Session string }{{owner, "default"}}
	if list := listSessions(t, cs); !slices.Equal(list.All, want) {
		t.Errorf("the token's client sees %v, want %v", list.All, want)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	log := logs.String()
	if !strings.Contains(log, "principal=token source=static-token remote=127.0.0.1:") || !strings.Contains(log, "tool=session_eval scope=sysml:execute outcome=ok") {
		t.Errorf("audit log:\n%s", log)
	}
	if strings.Contains(log, "s3cret") {
		t.Errorf("the token was logged:\n%s", log)
	}
}

// repl_command's gate is the read scope; each command then takes the scope of
// the typed tool doing the same, refused before the session is touched.
func TestReplCommandTakesTheScopeOfItsCommand(t *testing.T) {
	principals := map[string]guard.Principal{
		"reader":   principalOnly("oidc", "reader", catalog.ScopeRead),
		"executor": principalOnly("oidc", "executor", catalog.ScopeRead, catalog.ScopeExecute, catalog.ScopeSession),
	}
	url, logs := serveAuthenticated(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, principals)
	reader, executor := connectAs(t, url, "reader"), connectAs(t, url, "executor")
	run := func(cs *sdk.ClientSession, line string) (*sdk.CallToolResult, error) {
		return cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "repl_command", Arguments: map[string]any{"line": line}})
	}
	if res, err := run(reader, "%list"); err != nil || res.IsError {
		t.Errorf("reader %%list: err %v, result %q", err, resultText(res))
	}
	for line, scope := range map[string]catalog.Scope{"%eval 1 + 1": catalog.ScopeExecute, "%action A": catalog.ScopeExecute, "%solve X": catalog.ScopeExecute, "%load a.sysml": catalog.ScopeSession, "%save out.sysml": catalog.ScopeWrite} {
		_, err := run(reader, line)
		jerr := refusal(err)
		if jerr == nil {
			t.Errorf("reader %s: %v, want the insufficient_scope refusal", line, err)
			continue
		}
		if refused, _ := toolkit.RefusedScope(jerr); refused != scope || !strings.Contains(jerr.Message, string(scope)) {
			t.Errorf("reader %s: refused for %s (%s), want %s", line, refused, jerr.Message, scope)
		}
	}
	if res, err := run(executor, "%eval 1 + 1"); err != nil || res.IsError || !strings.Contains(resultText(res), "2") {
		t.Errorf("executor %%eval: err %v, result %q", err, resultText(res))
	}
	if _, err := run(executor, "%save out.sysml"); refusal(err) == nil {
		t.Errorf("executor %%save: %v, want the insufficient_scope refusal", err)
	}
	// A command this build does not know has no effect and is reported, not refused.
	if res, err := run(reader, "%nothing"); err != nil || !res.IsError || !strings.Contains(resultText(res), "no command") {
		t.Errorf("reader %%nothing: err %v, result %q", err, resultText(res))
	}
	// The reader's refusals opened no session for it.
	if list := listSessions(t, executor); len(list.Sessions) != 1 {
		t.Errorf("executor lists %v, want only the default its %%eval opened", list.Sessions)
	}
	if log := logs.String(); !strings.Contains(log, "principal=reader source=oidc remote=127.0.0.1:") || !strings.Contains(log, "tool=repl_command scope=sysml:execute outcome=refused") || !strings.Contains(log, "tool=repl_command scope=sysml:write outcome=refused") {
		t.Errorf("audit log:\n%s", log)
	}
}

// A tool this server does not serve is answered as unknown by the SDK, not
// refused for a scope the caller lacks, so no credential is sought for it.
func TestAnUnservedToolIsUnknownNotRefused(t *testing.T) {
	principals := map[string]guard.Principal{"reader": principalOnly("oidc", "reader", catalog.ScopeSession)}
	url, logs := serveAuthenticated(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, principals)
	cs := connectAs(t, url, "reader")
	_, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "hover", Arguments: map[string]any{}})
	var jerr *jsonrpc.Error
	if !errors.As(err, &jerr) || jerr.Code == CodeInsufficientScope || !strings.Contains(jerr.Message, "unknown tool") {
		t.Errorf("hover over the session group: %v, want the SDK's unknown tool error", err)
	}
	if log := logs.String(); !strings.Contains(log, `tool=hover scope="" outcome=error`) {
		t.Errorf("audit log:\n%s", log)
	}
}

// %check-witness names where checks write files, so it takes sysml:write, and
// a directory outside the workspace root is refused by the session's guard.
func TestCheckWitnessTakesWriteAndStaysUnderTheRoot(t *testing.T) {
	principals := map[string]guard.Principal{
		"runner": principalOnly("oidc", "runner", catalog.ScopeRead, catalog.ScopeSession, catalog.ScopeExecute),
		"writer": principalOnly("oidc", "writer", catalog.ScopeRead, catalog.ScopeSession, catalog.ScopeExecute, catalog.ScopeWrite),
	}
	root := t.TempDir()
	url, _ := serveAuthenticated(t, Config{Root: root, Groups: []Group{GroupSession}, Version: "test"}, principals)
	run := func(cs *sdk.ClientSession, line string) (*sdk.CallToolResult, error) {
		return cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "repl_command", Arguments: map[string]any{"line": line}})
	}
	inside := filepath.Join(root, "witnesses")
	_, err := run(connectAs(t, url, "runner"), "%check-witness "+inside)
	if refused, ok := toolkit.RefusedScope(err); !ok || refused != catalog.ScopeWrite {
		t.Errorf("runner %%check-witness: %v, want the refusal naming %s", err, catalog.ScopeWrite)
	}
	writer := connectAs(t, url, "writer")
	if res, err := run(writer, "%check-witness "+inside); err != nil || res.IsError {
		t.Errorf("writer %%check-witness under the root: err %v, result %q", err, resultText(res))
	}
	outside := t.TempDir()
	if res, err := run(writer, "%check-witness "+outside); err != nil || !res.IsError || !strings.Contains(resultText(res), "root") {
		t.Errorf("writer %%check-witness outside the root: err %v, result %q, want a refusal naming the root", err, resultText(res))
	}
	if res, err := run(writer, "%check-witness"); err != nil || res.IsError || !strings.Contains(resultText(res), inside) {
		t.Errorf("after the refusal, %%check-witness reports %q, want %s kept", resultText(res), inside)
	}
}
