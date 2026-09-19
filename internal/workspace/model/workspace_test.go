package model

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestWorkspaceOpenIndexesDocument(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { namespace N; }"), 1)
	if syms := ws.LookupQualified("P::N"); len(syms) != 1 {
		t.Fatalf("P::N = %d symbols, want 1", len(syms))
	}
}

func TestWorkspaceUpdateReindexesIncrementally(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { namespace Old; }"), 1)
	ws.Update("a.sysml", []byte("package P { namespace New; }"), 2)
	if syms := ws.LookupQualified("P::Old"); len(syms) != 0 {
		t.Fatalf("P::Old = %d, want 0 (stale entry not cleared)", len(syms))
	}
	if syms := ws.LookupQualified("P::New"); len(syms) != 1 {
		t.Fatalf("P::New = %d, want 1", len(syms))
	}
	if syms := ws.LookupQualified("P"); len(syms) != 1 {
		t.Fatalf("P = %d, want 1 (not doubled)", len(syms))
	}
}

func TestWorkspaceCloseKeepsOnDiskContent(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package Disk { namespace D; }"))
	ws.Open("a.sysml", []byte("package Buf { namespace B; }"), 1)
	if syms := ws.LookupQualified("Buf::B"); len(syms) != 1 {
		t.Fatal("open buffer should be authoritative")
	}
	ws.Close("a.sysml")
	if syms := ws.LookupQualified("Disk::D"); len(syms) != 1 {
		t.Fatal("closing should revert to on-disk content")
	}
	if syms := ws.LookupQualified("Buf::B"); len(syms) != 0 {
		t.Fatal("buffer content should be gone after close")
	}
}

func TestWorkspaceDeleteOnDiskKeepsOpenBuffer(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package Disk { namespace D; }"))
	ws.Open("a.sysml", []byte("package Buf { namespace B; }"), 1)
	ws.DeleteOnDisk("a.sysml")
	if syms := ws.LookupQualified("Buf::B"); len(syms) != 1 {
		t.Fatal("open buffer should survive the file's deletion")
	}
	ws.Close("a.sysml")
	if ws.Document("a.sysml") != nil {
		t.Fatal("closing a deleted file should drop the document")
	}
}

func TestWorkspaceDeleteOnDiskRemovesClosedDocument(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package Disk { namespace D; }"))
	ws.DeleteOnDisk("a.sysml")
	if syms := ws.LookupQualified("Disk::D"); len(syms) != 0 {
		t.Fatalf("Disk::D = %d, want 0 after the file was deleted", len(syms))
	}
	if ws.Document("a.sysml") != nil {
		t.Fatal("document should be gone after the file was deleted")
	}
}

func TestWorkspaceTracksOpenNames(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("disk.sysml", []byte("package D;"))
	ws.Open("open.sysml", []byte("package O;"), 1)
	if !ws.IsOpen("open.sysml") || ws.IsOpen("disk.sysml") {
		t.Fatalf("IsOpen: open=%v disk=%v", ws.IsOpen("open.sysml"), ws.IsOpen("disk.sysml"))
	}
	if names := ws.OpenNames(); len(names) != 1 || names[0] != "open.sysml" {
		t.Fatalf("OpenNames = %v, want [open.sysml]", names)
	}
}

func TestWorkspaceAnalyzedContentMatchesDiagnostics(t *testing.T) {
	ws := NewWorkspace()
	src := "package P {\n    part x : Missing;\n}\n"
	ws.Open("a.sysml", []byte(src), 1)

	// The spans returned belong to the content returned with them; a caller
	// converting them against any other revision would misplace the markers.
	content, diags, ok := ws.AnalyzedContent("a.sysml")
	if !ok || string(content) != src {
		t.Fatalf("AnalyzedContent ok=%v content=%q", ok, content)
	}
	if len(diags) == 0 {
		t.Fatal("no diagnostics for an unresolved type")
	}
	for _, d := range diags {
		if d.Span.End() > len(content) {
			t.Errorf("span %v outside the content it was analyzed with (%d bytes)", d.Span, len(content))
		}
	}
	if _, _, ok := ws.AnalyzedContent("gone.sysml"); ok {
		t.Error("AnalyzedContent ok for an unknown document")
	}
}

func TestWorkspaceRemoveDropsFromIndex(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P;"), 1)
	ws.Remove("a.sysml")
	if syms := ws.LookupQualified("P"); len(syms) != 0 {
		t.Fatalf("P = %d, want 0 after remove", len(syms))
	}
	if ws.Document("a.sysml") != nil {
		t.Fatal("document should be gone after remove")
	}
}

func TestWorkspaceEditDropsTheDependentsOnly(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package A { part def X; }"), 1)
	ws.Open("b.sysml", []byte("package B { part def Y :> A::X; }"), 1)
	ws.Open("c.sysml", []byte("package C { part def Z; part z : Z; }"), 1)
	for _, name := range []string{"a.sysml", "b.sysml", "c.sysml"} {
		if diags := ws.Diagnostics(name); len(diags) != 0 {
			t.Fatalf("%s: %v", name, diags)
		}
	}
	ws.Update("a.sysml", []byte("package A { part def X2; }"), 2)
	if _, cached := ws.diagCache["c.sysml"]; !cached {
		t.Fatal("c.sysml, which reads nothing of a.sysml, lost its diagnostics")
	}
	if _, cached := ws.diagCache["b.sysml"]; cached {
		t.Fatal("b.sysml, which specializes A::X, kept its diagnostics")
	}
	if diags := ws.Diagnostics("b.sysml"); len(diags) == 0 {
		t.Fatal("b.sysml still resolves A::X, which is gone")
	}
}

func TestWorkspaceEditsReleaseWhatTheyReplaced(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package A { part def X; part x : X; }"), 1)
	ws.Open("b.sysml", []byte("package B { part def Y :> A::X; part y : Y; }"), 1)
	ws.Diagnostics("a.sysml")
	ws.Diagnostics("b.sysml")
	ownedA, ownedB := ws.resolver.Owned("a.sysml"), ws.resolver.Owned("b.sysml")
	if ownedA == 0 || ownedB == 0 {
		t.Fatalf("owned after the first analysis: a %d, b %d; want both > 0", ownedA, ownedB)
	}
	for i := 2; i < 200; i++ {
		ws.Update("a.sysml", []byte("package A { part def X; part x : X; }"), i)
		ws.Diagnostics("a.sysml")
		ws.Diagnostics("b.sysml")
		if a, b := ws.resolver.Owned("a.sysml"), ws.resolver.Owned("b.sysml"); a != ownedA || b != ownedB {
			t.Fatalf("edit %d: owned a %d, b %d; want %d, %d as after the first analysis", i, a, b, ownedA, ownedB)
		}
	}
}

// TestWorkspaceUnionJudgmentFollowsAnotherDocument: a workspace-wide audit judges
// each document over what every document declares, so a change to one document
// moves another's verdict though the other reads nothing of it.
func TestWorkspaceUnionJudgmentFollowsAnotherDocument(t *testing.T) {
	const notDerived = "oosem-requirement-not-derived"
	sat := []byte("package S { private import OOSEM::*; #systemRequirement requirement sys; }")
	ws := NewWorkspace()
	ws.Open("hub.sysml", []byte("package M { private import OOSEM::*; #missionRequirement requirement mission; }"), 1)
	ws.Open("sat.sysml", sat, 1)
	if got := codesOf(ws.Diagnostics("sat.sysml")); got[notDerived] != 1 {
		t.Fatalf("sat.sysml under a mission requirement: %v, want one %s", got, notDerived)
	}
	ws.Update("hub.sysml", []byte("package M { private import OOSEM::*; #stakeholderNeed requirement need; }"), 2)
	if got := codesOf(ws.Diagnostics("sat.sysml")); got[notDerived] != 0 {
		t.Fatalf("sat.sysml with no mission requirement anywhere: %v, want no %s", got, notDerived)
	}
	fresh := NewWorkspace()
	fresh.Open("hub.sysml", ws.Document("hub.sysml").Content, 1)
	fresh.Open("sat.sysml", sat, 1)
	if got, want := codesOf(ws.Diagnostics("sat.sysml")), codesOf(fresh.Diagnostics("sat.sysml")); !reflect.DeepEqual(got, want) {
		t.Fatalf("incremental %v, fresh %v", got, want)
	}
}

func codesOf(diags []diag.Diagnostic) map[string]int {
	out := map[string]int{}
	for _, d := range diags {
		out[d.Code]++
	}
	return out
}
