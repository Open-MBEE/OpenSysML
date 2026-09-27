package model

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

var (
	hydrateBase = []byte("package Hull { part def Bus; part def Panel; }")
	hydrateUser = []byte("package Craft { private import Hull::*; part def Sat :> Bus { part p : Panel; } part def Odd :> Missing; }")
)

// recordCache analyzes the documents in a workspace over a fresh cache, writing
// their records to it, and returns the cache with the diagnostics observed loaded.
func recordCache(t *testing.T) (*libs.Cache, [][]string) {
	t.Helper()
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(hydrateInputs())
	diags := ws.DiagnosticsAll([]string{"base.sysml", "user.sysml"})
	for _, name := range []string{"base.sysml", "user.sysml"} {
		if ws.Recorded(name) {
			t.Fatalf("%s is recorded on a cold cache", name)
		}
	}
	return cache, [][]string{messagesOf(diags[0]), messagesOf(diags[1])}
}

func hydrateInputs() []Input {
	return []Input{
		{Name: "base.sysml", Content: hydrateBase, Version: 1},
		{Name: "user.sysml", Content: hydrateUser, Version: 1},
	}
}

func messagesOf(diags []diag.Diagnostic) []string {
	out := make([]string, len(diags))
	for i, d := range diags {
		out[i] = fmt.Sprintf("%d+%d %s", d.Span.Offset, d.Span.Len, d.Message)
	}
	return out
}

// A warm cache opens the same files as records, reporting the stored diagnostics;
// no cache, or the switch off, holds them loaded as before.
func TestOpenAllReadsRecordsFromAWarmCache(t *testing.T) {
	cache, want := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(hydrateInputs())
	for _, name := range []string{"base.sysml", "user.sysml"} {
		if !ws.Recorded(name) {
			t.Fatalf("%s is not recorded on a warm cache", name)
		}
		if ws.Document(name).AST != nil || ws.Document(name).Version != 1 {
			t.Fatalf("%s holds a tree or lost its version", name)
		}
	}
	got := ws.DiagnosticsAll([]string{"base.sysml", "user.sysml"})
	if !reflect.DeepEqual([][]string{messagesOf(got[0]), messagesOf(got[1])}, want) {
		t.Fatalf("recorded diagnostics %v, loaded %v", got, want)
	}
	if want[1] == nil || len(want[1]) == 0 {
		t.Fatal("user.sysml reports nothing: the fixture is vacuous")
	}

	plain := NewWorkspace()
	plain.OpenAll(hydrateInputs())
	if plain.Recorded("base.sysml") || plain.Recorded("user.sysml") {
		t.Fatal("a workspace with no cache holds a record")
	}
}

// A file whose bytes changed is a cache miss and is parsed; a record whose
// analysis read the changed file does not hold either, and that file is parsed too.
func TestOpenAllParsesMissesAndTheirDependents(t *testing.T) {
	cache, _ := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	changed := []byte("package Hull { part def Bus; part def Panel; part def Missing; }")
	ws.OpenAll([]Input{
		{Name: "base.sysml", Content: changed, Version: 2},
		{Name: "user.sysml", Content: hydrateUser, Version: 1},
	})
	if ws.Recorded("base.sysml") {
		t.Fatal("changed base.sysml is held as a record of other bytes")
	}
	if ws.Recorded("user.sysml") {
		t.Fatal("user.sysml is held as a record whose analysis read the old base.sysml")
	}
	if got := messagesOf(ws.Diagnostics("user.sysml")); len(got) != 0 {
		t.Fatalf("user.sysml beside a Hull::Missing still reports %v", got)
	}
}

// Hydrating a recorded document gives its symbols trees and invalidates the
// documents that read it, through the same relation an edit uses; a recorded
// reader stays recorded, since the unchanged document still answers its reads.
func TestHydrateIsAnInvalidation(t *testing.T) {
	cache, want := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(hydrateInputs())
	ws.DiagnosticsAll([]string{"base.sysml", "user.sysml"})
	gen := ws.Generation()
	if err := ws.Hydrate("base.sysml"); err != nil {
		t.Fatal(err)
	}
	if ws.Recorded("base.sysml") || !ws.Recorded("user.sysml") {
		t.Fatal("Hydrate did not hydrate base.sysml alone")
	}
	if syms := ws.index.LookupQualified("Hull::Bus"); len(syms) != 1 || syms[0].Decl == nil {
		t.Fatalf("hydrated Hull::Bus resolves to %d symbols without a tree", len(syms))
	}
	if ws.Generation() == gen {
		t.Fatal("hydration left the generation alone")
	}
	if _, cached := ws.diagCache["user.sysml"]; cached {
		t.Fatal("user.sysml's diagnostics survived the hydration of a document it reads")
	}
	if got := messagesOf(ws.Diagnostics("user.sysml")); !reflect.DeepEqual(got, want[1]) {
		t.Fatalf("user.sysml after hydration reports %v, want %v", got, want[1])
	}
	if err := ws.Hydrate("base.sysml"); err != nil {
		t.Fatalf("hydrating a loaded document: %v", err)
	}
}

// Closing a document clean on disk demotes it to its record; closing one whose
// buffer differs from the disk holds the disk bytes, as their record too.
func TestCloseDemotesACleanDocument(t *testing.T) {
	cache, want := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.SetOnDisk("base.sysml", hydrateBase)
	ws.SetOnDisk("user.sysml", hydrateUser)
	ws.Open("base.sysml", hydrateBase, 1)
	ws.Open("user.sysml", hydrateUser, 1)
	ws.Diagnostics("user.sysml")
	ws.Close("user.sysml")
	if !ws.Recorded("user.sysml") {
		t.Fatal("a clean user.sysml was not demoted on close")
	}
	if got := messagesOf(ws.Diagnostics("user.sysml")); !reflect.DeepEqual(got, want[1]) {
		t.Fatalf("demoted user.sysml reports %v, want %v", got, want[1])
	}
	ws.Open("user.sysml", hydrateUser, 2)
	if ws.Recorded("user.sysml") {
		t.Fatal("an open buffer is held as a record")
	}
	ws.Update("user.sysml", []byte("package Craft { part def Edited; }"), 3)
	if len(ws.index.LookupQualified("Craft::Sat")) != 0 {
		t.Fatal("the edited buffer still declares Craft::Sat")
	}
	ws.Close("user.sysml")
	if !ws.Recorded("user.sysml") {
		t.Fatal("a buffer closed dirty is not held as the record of the disk bytes")
	}
	if syms := ws.index.LookupQualified("Craft::Sat"); len(syms) != 1 || !syms[0].Recorded() {
		t.Fatalf("closed user.sysml resolves Craft::Sat to %d symbols", len(syms))
	}
	if got := messagesOf(ws.Diagnostics("user.sysml")); !reflect.DeepEqual(got, want[1]) {
		t.Fatalf("user.sysml closed dirty reports %v, want %v", got, want[1])
	}
}

// A demoted document beside no cache, or with no record for its bytes, stays loaded.
func TestCloseWithoutARecordStaysLoaded(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("base.sysml", hydrateBase)
	ws.Open("base.sysml", hydrateBase, 1)
	ws.Close("base.sysml")
	if ws.Recorded("base.sysml") || ws.Document("base.sysml") == nil {
		t.Fatal("a workspace with no cache demoted or dropped base.sysml")
	}
}

// Editing a document a record's analysis read hydrates the recorded dependent,
// as an edit invalidates a loaded one.
func TestSiblingEditHydratesRecordedDependents(t *testing.T) {
	cache, _ := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(hydrateInputs())
	ws.Update("base.sysml", []byte("package Hull { part def Bus; part def Panel; part def Missing; }"), 2)
	if ws.Recorded("user.sysml") {
		t.Fatal("user.sysml stays recorded after the document it reads changed")
	}
	if got := messagesOf(ws.Diagnostics("user.sysml")); len(got) != 0 {
		t.Fatalf("user.sysml beside a Hull::Missing still reports %v", got)
	}
}

// The references written in a workspace and a runtime over it read every
// document's tree: both hydrate what is recorded rather than answer from records.
func TestReferencesAndRuntimeHydrate(t *testing.T) {
	cache, _ := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(hydrateInputs())
	bus := ws.index.LookupQualified("Hull::Bus")
	if len(bus) != 1 {
		t.Fatalf("Hull::Bus resolves to %d symbols", len(bus))
	}
	refs, err := ws.ReferencesTo(bus[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Doc != "user.sysml" {
		t.Fatalf("references to Hull::Bus: %+v", refs)
	}
	if ws.Recorded("base.sysml") || ws.Recorded("user.sysml") {
		t.Fatal("a reverse-reference query left a document recorded")
	}

	ws = NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(hydrateInputs())
	if _, err := ws.NewRuntime(); err != nil {
		t.Fatal(err)
	}
	if ws.Recorded("base.sysml") || ws.Recorded("user.sysml") {
		t.Fatal("a runtime was built over a recorded document")
	}
}

// A new sibling that takes part in answering what a recorded document read
// hydrates it too: the record's diagnostics are those of an analysis that never
// saw the sibling, and the hydrated document reports what a loaded one does.
func TestNewSiblingHydratesRecordedDependents(t *testing.T) {
	cache, _ := recordCache(t)
	third := []byte("package Hull { part def Missing; }")
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(hydrateInputs())
	if !ws.Recorded("user.sysml") {
		t.Fatal("user.sysml is not recorded from a warm cache")
	}
	ws.SetOnDisk("third.sysml", third)
	if ws.Recorded("user.sysml") {
		t.Fatal("user.sysml stays recorded after a new document answered what it read")
	}
	loaded := NewWorkspace()
	loaded.OpenAll(append(hydrateInputs(), Input{Name: "third.sysml", Content: third, Version: 1}))
	want := messagesOf(loaded.Diagnostics("user.sysml"))
	if got := messagesOf(ws.Diagnostics("user.sysml")); !reflect.DeepEqual(got, want) {
		t.Fatalf("hydrated user.sysml reports %v, loaded beside the same documents %v", got, want)
	}
}

// A closed file set from disk is held as its record once the documents its
// analysis read are held — itself among them, since a record's reads are
// answered by its own document; a record installed alone before its siblings
// does not hold and is parsed.
func TestSetOnDiskTakesTheRecordAmongHeldSiblings(t *testing.T) {
	cache, want := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.SetOnDisk("base.sysml", hydrateBase)
	ws.SetOnDisk("user.sysml", hydrateUser)
	if !ws.Recorded("user.sysml") {
		t.Fatal("user.sysml set from disk beside its held sibling is not held as its record")
	}
	if got := messagesOf(ws.Diagnostics("user.sysml")); !reflect.DeepEqual(got, want[1]) {
		t.Fatalf("recorded diagnostics %v, loaded %v", got, want[1])
	}
	if ws.Document("base.sysml").AST == nil {
		t.Fatal("base.sysml, set from disk before the sibling its analysis read, holds no tree")
	}
}
