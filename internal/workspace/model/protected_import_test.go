package model

import "testing"

// openAll opens each document in one workspace and returns the name-resolution
// findings of the document called uri.
func openAll(t *testing.T, uri string, docs map[string]string) []string {
	t.Helper()
	ws := NewWorkspace()
	for name, src := range docs {
		ws.Open(name, []byte(src), 1)
		defer ws.Close(name)
	}
	return unresolvedMessages(ws, uri)
}

// A protected import is visible in the importing body and in what specializes
// it (SysML v2 7.5.3), across documents: Lib, the importer and the
// specializations are separate files, so only a real workspace shows the reach.
func TestProtectedImportReachesSpecializationsAcrossDocuments(t *testing.T) {
	lib := `package Lib { part def Pub; }`
	base := `package App {
		part def Base { protected import Lib::*; }
		part def Sub :> Base { part p : Pub; }
	}`
	sub := `package Deep { part def Deeper :> App::Base { part q : Pub; } }`

	docs := map[string]string{
		"lib.sysml":  lib,
		"app.sysml":  base,
		"deep.sysml": sub,
	}
	for _, uri := range []string{"app.sysml", "deep.sysml"} {
		if found := openAll(t, uri, docs); len(found) != 0 {
			t.Errorf("%s: expected no findings, got %v", uri, found)
		}
	}
}

// The reach stops at specialization: an unrelated namespace sees nothing of what
// another namespace protectedly imported.
func TestProtectedImportDoesNotReachAnUnrelatedDocument(t *testing.T) {
	docs := map[string]string{
		"lib.sysml":   `package Lib { part def Pub; }`,
		"app.sysml":   `package App { part def Base { protected import Lib::*; } }`,
		"other.sysml": `package Other { part def Thing { part p : Pub; } }`,
	}
	if found := openAll(t, "other.sysml", docs); len(found) != 1 {
		t.Fatalf("expected one unresolved finding for Pub in an unrelated namespace, got %d: %v",
			len(found), found)
	}
}

// An expose is a protected import (SysML v2 8.3.26.2), so a view specializing
// the exposing one sees the exposed members too.
func TestExposeReachesASpecializingViewAcrossDocuments(t *testing.T) {
	docs := map[string]string{
		"lib.sysml":  `package Lib { part def Pub; }`,
		"view.sysml": `package App { view def V { expose Lib::**; } view def W :> V { part p : Pub; } }`,
	}
	if found := openAll(t, "view.sysml", docs); len(found) != 0 {
		t.Fatalf("expected no findings, got %v", found)
	}
}
