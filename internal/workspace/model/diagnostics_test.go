package model

import "testing"

func TestWorkspaceDiagnosticsClean(t *testing.T) {
	ws := NewWorkspace()
	// A nested `package`, not a `namespace`: the latter is KerML notation and is
	// warned about in a .sysml file.
	ws.Open("a.sysml", []byte("package P { package N; alias A for P::N; }"), 1)
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(d), d)
	}
}

func TestWorkspaceDiagnosticsReportsUnresolved(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { alias A for P::Missing; }"), 1)
	d := ws.Diagnostics("a.sysml")
	if len(d) == 0 {
		t.Fatal("expected an unresolved diagnostic")
	}
	if d[0].Source != "name-resolution" {
		t.Fatalf("Source = %q, want name-resolution", d[0].Source)
	}
}

func TestWorkspaceDiagnosticsRecomputeAfterEdit(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { alias A for P::Missing; }"), 1)
	if len(ws.Diagnostics("a.sysml")) == 0 {
		t.Fatal("expected diagnostics before fix")
	}
	ws.Update("a.sysml", []byte("package P { package Missing; alias A for P::Missing; }"), 2)
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("diagnostics after fix = %d, want 0: %+v", len(d), d)
	}
}

func TestWorkspaceDiagnosticsUnknownDocumentNil(t *testing.T) {
	ws := NewWorkspace()
	if d := ws.Diagnostics("nope.sysml"); d != nil {
		t.Fatalf("unknown doc diagnostics = %+v, want nil", d)
	}
}
