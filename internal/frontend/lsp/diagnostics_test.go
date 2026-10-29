package lsp

import (
	"context"
	"strings"
	"sync"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// baseClient stubs all 12 protocol.Client methods; only PublishDiagnostics is overridden by fakeClient.
// baseClient answers every client request as unimplemented; fakes embed it and
// record what they need.
type baseClient struct{ protocol.UnimplementedClient }

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
	// The code description opens the catalogue at the code's section.
	if cd := got.Diagnostics[0].CodeDescription; cd == nil || string(cd.Href) != "https://runtime.opensysml.org/reference/diagnostics/#syntax" {
		t.Errorf("codeDescription = %v, want the catalogue section of syntax", cd)
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
	if d.CodeDescription == nil || !strings.HasSuffix(string(d.CodeDescription.Href), "/reference/diagnostics/#import-visibility") {
		t.Errorf("codeDescription = %v, want the catalogue section of import-visibility", d.CodeDescription)
	}
	want := protocol.Range{
		Start: protocol.Position{Line: 2, Character: 4},
		End:   protocol.Position{Line: 2, Character: 10},
	}
	if d.Range != want {
		t.Errorf("range = %v, want %v (the `import` keyword)", d.Range, want)
	}
}

// The removed `defer <event>;` state member reaches the editor as one error
// carrying the parser's stable code, in either conformance mode, and the rest
// of the state body is still analysed.
func TestPublishDiagnosticsReportsRemovedDeferNotation(t *testing.T) {
	for _, mode := range []diag.ConformanceMode{diag.ConformanceDefault, diag.ConformanceStrict} {
		ws := model.NewWorkspace(model.WithConformanceMode(mode))
		s := NewServer(ws)
		fc := &fakeClient{}
		s.client = fc

		name := "legacy.sysml"
		ws.Open(name, []byte("package P {\n    attribute def Alarm;\n    state def S {\n        state a {\n            defer Alarm;\n        }\n        state b;\n    }\n}\n"), 1)
		s.publishDiagnostics(context.Background(), name)

		published := fc.all()
		if len(published) != 1 || len(published[0].Diagnostics) != 1 {
			t.Fatalf("%v: published = %v, want one diagnostic", mode, published)
		}
		d := published[0].Diagnostics[0]
		if d.Severity != protocol.DiagnosticSeverityError {
			t.Errorf("%v: severity = %v, want %v", mode, d.Severity, protocol.DiagnosticSeverityError)
		}
		if d.Code != "defer-notation-removed" {
			t.Errorf("%v: code = %v, want %q", mode, d.Code, "defer-notation-removed")
		}
		if d.Range.Start != (protocol.Position{Line: 4, Character: 12}) {
			t.Errorf("%v: range = %v, want the `defer` keyword at 4:12", mode, d.Range)
		}
		for _, want := range []string{"extension was removed", "ordered", "exit action", "sysml-v1-migration.md"} {
			if !strings.Contains(d.Message, want) {
				t.Errorf("%v: message %q lacks %q", mode, d.Message, want)
			}
		}
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
