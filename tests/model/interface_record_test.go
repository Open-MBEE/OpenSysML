package model_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// A document held as its interface record must be indistinguishable, to every
// other document of its workspace, from the same document loaded: the same
// diagnostics in the same order, and every reference into it resolving to the
// same symbol (see docs/internals/interface-records.md).

// interfaceAnswers renders what the workspace says about a document that its
// neighbours' residency may not change: its diagnostics and what each written
// reference resolves to, segment by segment.
func interfaceAnswers(ws *model.Workspace, name string) []string {
	var out []string
	for _, d := range ws.Diagnostics(name) {
		out = append(out, fmt.Sprintf("diag %d+%d %s %s/%s: %s",
			d.Span.Offset, d.Span.Len, d.Severity, d.Source, d.Code, d.Message))
	}
	doc := ws.Document(name)
	if doc == nil {
		return append(out, "no document")
	}
	for _, ref := range resolve.References(doc.AST, doc.Scope) {
		if ref.QN == nil || len(ref.QN.Parts) == 0 {
			continue
		}
		sym, ok := ws.ResolveReferenceInDoc(name, ref)
		line := fmt.Sprintf("ref %d+%d -> %v %s", ref.QN.Span().Offset, ref.QN.Span().Len, ok, symbolID(sym))
		for _, seg := range ws.ResolveReferenceSegmentsInDoc(name, ref) {
			line += " " + symbolID(seg)
		}
		out = append(out, line)
	}
	return out
}

// recordDifferential opens docs loaded, then holds each in turn as its record
// and checks every other document answers as before. It reports how many
// documents were recorded, since one whose interface has no record stays loaded.
func recordDifferential(t *testing.T, docs map[string][]byte) (recorded int) {
	t.Helper()
	names := make([]string, 0, len(docs))
	inputs := make([]model.Input, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		inputs = append(inputs, model.Input{Name: name, Content: docs[name], Version: 1})
	}
	loaded := model.NewWorkspace()
	loaded.OpenAll(inputs)
	want := make(map[string][]string, len(names))
	for _, name := range names {
		want[name] = interfaceAnswers(loaded, name)
	}

	// Every record goes through the cache: what is installed is what a later
	// process would read back, not the writer's in-memory form.
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	digest := libs.SourceDigest(libs.DefaultSource())

	ws := model.NewWorkspace()
	ws.OpenAll(inputs)
	for _, target := range names {
		written, err := loaded.InterfaceRecord(target)
		if errors.Is(err, libs.ErrUnrecordable) {
			t.Logf("%s: %v", target, err)
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		key := cache.InterfaceKey(docs[target], digest, diag.ConformanceDefault)
		if err := cache.StoreInterface(key, written); err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		rec, ok := cache.LoadInterface(key)
		if !ok {
			t.Fatalf("%s: record not read back from the cache", target)
		}
		if err := ws.OpenRecorded(rec); err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		recorded++
		for _, other := range names {
			if other == target {
				continue
			}
			got := interfaceAnswers(ws, other)
			if diff := firstDifference(got, want[other]); diff != "" {
				t.Fatalf("%s with %s recorded differs from loaded:\n%s", other, target, diff)
			}
		}
		// The batch path serves the recorded document its stored diagnostics
		// and analyzes the others as the single-document path did.
		batch := ws.DiagnosticsAll(names)
		for i, name := range names {
			if diff := firstDifference(renderDiagnostics(batch[i]), renderDiagnostics(ws.Diagnostics(name))); diff != "" {
				t.Fatalf("%s with %s recorded: DiagnosticsAll differs from Diagnostics:\n%s", name, target, diff)
			}
		}
		ws.Open(target, docs[target], 2)
	}
	return recorded
}

func renderDiagnostics(diags []diag.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, fmt.Sprintf("diag %d+%d %s %s/%s: %s",
			d.Span.Offset, d.Span.Len, d.Severity, d.Source, d.Code, d.Message))
	}
	return out
}

// The workspace-wide audits fold facts gathered from every document: an
// element's identity, and the relationships the OOSEM and MOSA audits gather
// from a body, are read from a recorded document's record.
func TestInterfaceRecordWorkspaceAudits(t *testing.T) {
	t.Run("identity", func(t *testing.T) {
		docs := map[string][]byte{
			"a.sysml": []byte(`package IdA {
	@IdentityMetadata::ProjectRef { projectId = "proj"; }
	part def A { @IdentityMetadata::ElementId { id = "shared"; } }
}
`),
			"b.sysml": []byte(`package IdB {
	@IdentityMetadata::ProjectRef { projectId = "proj"; }
	part def B { @IdentityMetadata::ElementId { id = "shared"; } }
	part def C { @IdentityMetadata::ElementId { id = "shared_om"; } }
}
`),
		}
		ws := model.NewWorkspace()
		ws.OpenAll([]model.Input{{Name: "a.sysml", Content: docs["a.sysml"], Version: 1}, {Name: "b.sysml", Content: docs["b.sysml"], Version: 1}})
		var collisions int
		for _, d := range ws.Diagnostics("b.sysml") {
			if d.Code == "identity-duplicate-id" {
				collisions++
			}
		}
		if collisions < 2 {
			t.Fatalf("b.sysml reports %d identity collisions with a.sysml loaded, want its id and its membership id shared (the fixture is vacuous otherwise)", collisions)
		}
		if recorded := recordDifferential(t, docs); recorded != 2 {
			t.Fatalf("%d of 2 documents recorded", recorded)
		}
	})
	t.Run("oosem", func(t *testing.T) {
		docs := map[string][]byte{
			"reqs.sysml": []byte(`package Reqs {
	private import OOSEM::*;
	#missionRequirement requirement mission;
	#systemRequirement requirement sys;
}
`),
			"links.sysml": []byte(`package Links {
	private import OOSEM::*;
	#derivation connection { end #original ::> Reqs::mission; end #derive ::> Reqs::sys; }
	part sat { satisfy Reqs::sys; }
}
`),
		}
		alone := model.NewWorkspace()
		alone.Open("reqs.sysml", docs["reqs.sysml"], 1)
		if n := oosemFindings(alone.Diagnostics("reqs.sysml")); n == 0 {
			t.Fatal("reqs.sysml reports no OOSEM finding alone (the fixture is vacuous otherwise)")
		}
		both := model.NewWorkspace()
		both.OpenAll([]model.Input{{Name: "reqs.sysml", Content: docs["reqs.sysml"], Version: 1}, {Name: "links.sysml", Content: docs["links.sysml"], Version: 1}})
		if n := oosemFindings(both.Diagnostics("reqs.sysml")); n != 0 {
			t.Fatalf("reqs.sysml reports %d OOSEM findings with links.sysml loaded, want its requirements derived and satisfied", n)
		}
		if recorded := recordDifferential(t, docs); recorded != 2 {
			t.Fatalf("%d of 2 documents recorded", recorded)
		}
	})
}

func oosemFindings(diags []diag.Diagnostic) int {
	n := 0
	for _, d := range diags {
		if strings.HasPrefix(d.Code, "oosem-") {
			n++
		}
	}
	return n
}

func TestInterfaceRecordDifferential(t *testing.T) {
	for set, files := range fixtureSets(t) {
		for lang, docs := range languageSets(files) {
			if len(docs) < 2 {
				continue
			}
			t.Run(set+"/"+lang, func(t *testing.T) {
				t.Parallel()
				recordDifferential(t, docs)
			})
		}
	}
}

func TestInterfaceRecordDifferentialCorpora(t *testing.T) {
	for _, root := range corpusRoots {
		t.Run(filepath.Base(root.dir), func(t *testing.T) {
			if _, err := os.Stat(root.dir); os.IsNotExist(err) {
				if os.Getenv(root.requireEnv) != "" {
					t.Fatalf("%s is set but %s is missing (run %s)", root.requireEnv, root.dir, root.fetch)
				}
				t.Skipf("%s not downloaded (run %s)", root.dir, root.fetch)
			}
			files := map[string][]byte{}
			for _, f := range modelFiles(t, root.dir) {
				content, err := os.ReadFile(filepath.Join(root.dir, f))
				if err != nil {
					t.Fatal(err)
				}
				files[filepath.ToSlash(f)] = content
			}
			for lang, docs := range languageSets(files) {
				t.Run(lang, func(t *testing.T) {
					t.Parallel()
					recorded := recordDifferential(t, docs)
					t.Logf("%d of %d documents recorded", recorded, len(docs))
				})
			}
		})
	}
}

// A question only a document's syntax tree answers is refused for a recorded
// document with a typed answer, never approximated from its record: the
// runtime in particular is never built over a record.
func TestInterfaceRecordNeedsHydration(t *testing.T) {
	t.Parallel()
	loaded := model.NewWorkspace()
	loaded.OpenAll([]model.Input{
		{Name: "a.sysml", Content: []byte("package A { part def P { attribute x : ScalarValues::Integer = 1; } }"), Version: 1},
		{Name: "b.sysml", Content: []byte("package B { private import A::*; part p : P; }"), Version: 1},
	})
	loaded.DiagnosticsAll([]string{"a.sysml", "b.sysml"})
	rec, err := loaded.InterfaceRecord("a.sysml")
	if err != nil {
		t.Fatal(err)
	}
	ws := model.NewWorkspace()
	ws.OpenAll([]model.Input{{Name: "b.sysml", Content: []byte("package B { private import A::*; part p : P; }"), Version: 1}})
	if err := ws.OpenRecorded(rec); err != nil {
		t.Fatal(err)
	}
	if n := len(ws.Diagnostics("b.sysml")); n != 0 {
		t.Fatalf("b.sysml over the recorded a.sysml: %d diagnostics", n)
	}
	_, err = ws.NewRuntime()
	var needs *symbols.NeedsHydration
	if !errors.Is(err, symbols.ErrNeedsHydration) || !errors.As(err, &needs) || needs.Doc != "a.sysml" {
		t.Fatalf("NewRuntime over a recorded document: got %v, want a NeedsHydration for a.sysml", err)
	}
	if _, err := ws.InterfaceRecord("a.sysml"); !errors.Is(err, model.ErrRecorded) {
		t.Fatalf("InterfaceRecord of a recorded document: got %v, want ErrRecorded", err)
	}
}
