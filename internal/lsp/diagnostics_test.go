package lsp

import (
	"context"
	"strings"
	"sync"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// baseClient stubs all 12 protocol.Client methods; only PublishDiagnostics is overridden by fakeClient.
type baseClient struct{}

func (baseClient) Progress(ctx context.Context, params *protocol.ProgressParams) error {
	return nil
}
func (baseClient) WorkDoneProgressCreate(ctx context.Context, params *protocol.WorkDoneProgressCreateParams) error {
	return nil
}
func (baseClient) LogMessage(ctx context.Context, params *protocol.LogMessageParams) error {
	return nil
}
func (baseClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	return nil
}
func (baseClient) ShowMessage(ctx context.Context, params *protocol.ShowMessageParams) error {
	return nil
}
func (baseClient) ShowMessageRequest(ctx context.Context, params *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, nil
}
func (baseClient) Telemetry(ctx context.Context, params interface{}) error {
	return nil
}
func (baseClient) RegisterCapability(ctx context.Context, params *protocol.RegistrationParams) error {
	return nil
}
func (baseClient) UnregisterCapability(ctx context.Context, params *protocol.UnregistrationParams) error {
	return nil
}
func (baseClient) ApplyEdit(ctx context.Context, params *protocol.ApplyWorkspaceEditParams) (bool, error) {
	return false, nil
}
func (baseClient) Configuration(ctx context.Context, params *protocol.ConfigurationParams) ([]interface{}, error) {
	return nil, nil
}
func (baseClient) WorkspaceFolders(ctx context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, nil
}

// fakeClient is locked because the debounced cross-document refresh publishes
// from a timer goroutine.
type fakeClient struct {
	baseClient
	mu        sync.Mutex
	published []*protocol.PublishDiagnosticsParams
}

func (c *fakeClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.published = append(c.published, params)
	return nil
}

// all returns a snapshot of what has been published so far.
func (c *fakeClient) all() []*protocol.PublishDiagnosticsParams {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*protocol.PublishDiagnosticsParams(nil), c.published...)
}

func TestPublishDiagnosticsReportsSyntaxError(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc

	// Missing closing brace -> syntax diagnostic.
	name := "bad.sysml"
	ws.Open(name, []byte("package P {"), 1)
	s.publishDiagnostics(context.Background(), name)

	if len(fc.all()) != 1 {
		t.Fatalf("published count = %d, want 1", len(fc.all()))
	}
	got := fc.all()[0]
	if got.URI != uri.File(name) {
		t.Errorf("URI = %q, want %q", got.URI, uri.File(name))
	}
	if len(got.Diagnostics) == 0 {
		t.Fatalf("expected at least one diagnostic")
	}
	// diag.SeverityError (0) -> LSP severity 1.
	if got.Diagnostics[0].Severity != protocol.DiagnosticSeverityError {
		t.Errorf("severity = %v, want %v", got.Diagnostics[0].Severity, protocol.DiagnosticSeverityError)
	}
	// Source/Code are copied verbatim from the syntax pass.
	if got.Diagnostics[0].Source != "syntax" {
		t.Errorf("source = %q, want %q", got.Diagnostics[0].Source, "syntax")
	}
	if got.Diagnostics[0].Code != "syntax" {
		t.Errorf("code = %v, want %q", got.Diagnostics[0].Code, "syntax")
	}
}

// A bare import reaches the editor as an error on the `import` keyword: the
// visibility indicator is mandatory in ImportPrefix (D2).
func TestPublishDiagnosticsReportsBareImportAsError(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc

	name := "bare.sysml"
	ws.Open(name, []byte("package Q { part def W; }\npackage P {\n    import Q::*;\n}\n"), 1)
	s.publishDiagnostics(context.Background(), name)

	published := fc.all()
	if len(published) != 1 || len(published[0].Diagnostics) != 1 {
		t.Fatalf("published = %v, want one diagnostic", published)
	}
	d := published[0].Diagnostics[0]
	if d.Severity != protocol.DiagnosticSeverityError {
		t.Errorf("severity = %v, want %v", d.Severity, protocol.DiagnosticSeverityError)
	}
	if d.Code != "import-visibility" {
		t.Errorf("code = %v, want %q", d.Code, "import-visibility")
	}
	want := protocol.Range{
		Start: protocol.Position{Line: 2, Character: 4},
		End:   protocol.Position{Line: 2, Character: 10},
	}
	if d.Range != want {
		t.Errorf("range = %v, want %v (the `import` keyword)", d.Range, want)
	}
}

func TestPublishDiagnosticsClearsWhenClean(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc

	// A valid document produces no diagnostics but must still publish an empty
	// list so the editor clears any stale squiggles (LSP push-clear semantics).
	ws.Open("ok.sysml", []byte("package P;"), 1)
	s.publishDiagnostics(context.Background(), "ok.sysml")

	if len(fc.all()) != 1 {
		t.Fatalf("published count = %d, want 1", len(fc.all()))
	}
	if len(fc.all()[0].Diagnostics) != 0 {
		t.Fatalf("diagnostics = %d, want 0 on clean doc", len(fc.all()[0].Diagnostics))
	}
}

func TestPublishDiagnosticsNilClientNoPanic(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	ws.Open("ok.sysml", []byte("package P;"), 1)
	// s.client is nil; must not panic.
	s.publishDiagnostics(context.Background(), "ok.sysml")
}

func TestPublishDiagnosticsKeepsQueryDependencyErrorsInTheirDocument(t *testing.T) {
	const dependency = `package Shared {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	calc def Leaf :> Query {
		in source : Element;
		OwnedElements(source = source)
	}
	calc def Broken :> Query {
		in source : Element;
		Leaf(source)
	}
}`
	const caller = `package Caller {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import Shared::*;
	calc def Entry :> Query {
		in source : Element;
		Broken(source = source)
	}
}`
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	ws.Open("dependency.sysml", []byte(dependency), 1)
	ws.Open("caller.sysml", []byte(caller), 1)

	s.publishDiagnostics(context.Background(), "caller.sysml")
	s.publishDiagnostics(context.Background(), "dependency.sysml")

	published := fc.all()
	if len(published) != 2 {
		t.Fatalf("published = %d document(s), want caller and dependency", len(published))
	}
	for _, diagnostic := range published[0].Diagnostics {
		if diagnostic.Source == "document-query" {
			t.Fatalf("caller received dependency diagnostic: %v", published[0].Diagnostics)
		}
	}
	for _, diagnostic := range published[1].Diagnostics {
		if diagnostic.Code != "document-query-positional-query-arguments" {
			continue
		}
		want := offsetToPosition([]byte(dependency), strings.Index(dependency, "Leaf(source)"))
		if diagnostic.Range.Start != want {
			t.Fatalf("dependency diagnostic starts at %v, want %v", diagnostic.Range.Start, want)
		}
		return
	}
	t.Fatalf("dependency received no planner diagnostic: %v", published[1].Diagnostics)
}
