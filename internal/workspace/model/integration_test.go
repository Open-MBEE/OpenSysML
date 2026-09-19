package model

import "testing"

// hasSym reports whether the workspace index resolves fqn to exactly one symbol.
func hasSym(t *testing.T, ws *Workspace, fqn string) bool {
	t.Helper()
	return len(ws.LookupQualified(fqn)) == 1
}

func TestWorkspaceConvergesAcrossChangeSequence(t *testing.T) {
	ws := NewWorkspace()

	// 1. Open a buffer. The members are packages: `namespace` is KerML notation,
	// warned about in a .sysml file.
	ws.Open("a.sysml", []byte("package P { package First; }"), 1)
	if !hasSym(t, ws, "P::First") {
		t.Fatal("after open: P::First missing")
	}
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("after open: %d diagnostics, want 0", len(d))
	}

	// 2. Edit the buffer (incremental reindex must drop the stale entry).
	ws.Update("a.sysml", []byte("package P { package Second; }"), 2)
	if hasSym(t, ws, "P::First") {
		t.Fatal("after edit: stale P::First still indexed")
	}
	if !hasSym(t, ws, "P::Second") {
		t.Fatal("after edit: P::Second missing")
	}

	// 3. External on-disk write while OPEN must NOT change the model (buffer wins).
	ws.SetOnDisk("a.sysml", []byte("package P { package Disk; }"))
	if hasSym(t, ws, "P::Disk") {
		t.Fatal("while open: on-disk content leaked into index")
	}
	if !hasSym(t, ws, "P::Second") {
		t.Fatal("while open: buffer content lost")
	}

	// 4. Close: on-disk content now takes effect.
	ws.Close("a.sysml")
	if hasSym(t, ws, "P::Second") {
		t.Fatal("after close: buffer content still indexed")
	}
	if !hasSym(t, ws, "P::Disk") {
		t.Fatal("after close: on-disk content not indexed")
	}
}

func TestWorkspaceCrossFileConverges(t *testing.T) {
	ws := NewWorkspace()
	// Two files; b imports a. Diagnostics must be clean once both present.
	ws.Open("a.sysml", []byte("package Lib { public namespace Widgets; }"), 1)
	ws.Open("b.sysml", []byte("package App { private import Lib::*; alias W for Lib::Widgets; }"), 1)
	if d := ws.Diagnostics("b.sysml"); len(d) != 0 {
		t.Fatalf("cross-file clean: %d diagnostics, want 0: %v", len(d), d)
	}

	// Remove the provider; b's alias target must now be unresolved.
	ws.Remove("a.sysml")
	d := ws.Diagnostics("b.sysml")
	if len(d) == 0 {
		t.Fatal("after removing provider: expected unresolved diagnostic on b")
	}
}
