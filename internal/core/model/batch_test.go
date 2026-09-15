package model

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// The model roots a batch is checked against: the fixtures, the shipped examples
// and the four OMG corpora, the latter skipped when absent unless CI requires them.
var batchRoots = []struct {
	dir     string
	require string
	fetch   string
}{
	{dir: "../../../testdata"},
	{dir: "../../../examples"},
	{dir: trainingGate.roots[0].dir, require: trainingGate.requireEnv, fetch: trainingGate.fetch},
	{dir: pilotCorporaGate.roots[0].dir, require: pilotCorporaGate.requireEnv, fetch: pilotCorporaGate.fetch},
	{dir: pilotCorporaGate.roots[1].dir, require: pilotCorporaGate.requireEnv, fetch: pilotCorporaGate.fetch},
	{dir: pilotCorporaGate.roots[2].dir, require: pilotCorporaGate.requireEnv, fetch: pilotCorporaGate.fetch},
}

// Every directory of models, opened as one batch, reports the same diagnostics in
// the same order on one worker as on many, and as opening its files one by one.
func TestParallelBatchValidationMatchesSerial(t *testing.T) {
	seen := map[string]bool{}
	for _, root := range batchRoots {
		if _, err := os.Stat(root.dir); os.IsNotExist(err) {
			if os.Getenv(root.require) != "" {
				t.Fatalf("%s is missing and %s is set; fetch it with %s", root.dir, root.require, root.fetch)
			}
			t.Logf("%s is absent (fetch it with %s), so this run proves nothing about it", root.dir, root.fetch)
			continue
		}
		for _, files := range modelDirectories(t, root.dir) {
			dir := filepath.Dir(files[0])
			if seen[dir] {
				continue
			}
			seen[dir] = true
			t.Run(filepath.ToSlash(dir), func(t *testing.T) {
				inputs := readInputs(t, files)
				serial := serialDiagnostics(inputs)
				for _, workers := range []int{1, 2, 8} {
					if got := batchDiagnostics(t, inputs, workers); got != serial {
						t.Errorf("a batch on %d workers reported:\n%s\nwant, as opening the files one by one does:\n%s", workers, got, serial)
					}
				}
			})
		}
	}
}

// A batch analyzes only what is not cached, answers every name asked for in the
// order asked, repeats included, and nil for a name the workspace does not hold.
func TestDiagnosticsAllAnswersInTheOrderAsked(t *testing.T) {
	ws := NewWorkspace()
	ws.OpenAll([]Input{
		{Name: "a.sysml", Content: []byte("package A { part def X; }"), Version: 1},
		{Name: "b.sysml", Content: []byte("package B { part x : A::X; part y : Missing; }"), Version: 1},
	})
	if diags := ws.Diagnostics("a.sysml"); len(diags) != 0 {
		t.Fatalf("a.sysml should analyze cleanly, got %+v", diags)
	}
	got := ws.DiagnosticsAll([]string{"b.sysml", "none.sysml", "a.sysml", "b.sysml"})
	if len(got) != 4 || got[1] != nil || len(got[2]) != 0 {
		t.Fatalf("DiagnosticsAll = %v, want b, nil, none, b", got)
	}
	if len(got[0]) != 1 || got[0][0].Message != "unresolved reference: Missing" || len(got[3]) != 1 {
		t.Fatalf("b.sysml reported %v, want one unresolved reference to Missing", got[0])
	}
	if &got[0][0] != &got[3][0] {
		t.Errorf("the two answers for b.sysml should be the one cached slice")
	}
}

// A batch opened over documents already there replaces them as Open does, so
// the index holds each name once.
func TestOpenAllReplacesEarlierDocuments(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package A { part def Old; }"), 1)
	ws.OpenAll([]Input{{Name: "a.sysml", Content: []byte("package A { part def New; }"), Version: 2}})
	if syms := ws.LookupQualified("A::Old"); len(syms) != 0 {
		t.Errorf("A::Old should be gone, found %d", len(syms))
	}
	if syms := ws.LookupQualified("A::New"); len(syms) != 1 {
		t.Errorf("A::New should be indexed once, found %d", len(syms))
	}
	if doc := ws.Document("a.sysml"); doc == nil || doc.Version != 2 || !ws.IsOpen("a.sysml") {
		t.Errorf("a.sysml should be the open version 2 document, got %+v", doc)
	}
}

func TestWorkersSetting(t *testing.T) {
	ws := NewWorkspace()
	if ws.Workers() != DefaultWorkers() || DefaultWorkers() < 1 {
		t.Fatalf("Workers() = %d, want the default %d", ws.Workers(), DefaultWorkers())
	}
	if err := ws.SetWorkers(0); err == nil {
		t.Error("SetWorkers(0) should be an error")
	}
	if err := ws.SetWorkers(3); err != nil || ws.Workers() != 3 {
		t.Errorf("SetWorkers(3) = %v, Workers() = %d", err, ws.Workers())
	}
	env := map[string]string{}
	lookup := func(key string) string { return env[key] }
	if n, err := workersFromLookup(lookup); err != nil || n != DefaultWorkers() {
		t.Errorf("unset: %d, %v; want the default", n, err)
	}
	env[WorkersEnvVar] = " 4 "
	if n, err := workersFromLookup(lookup); err != nil || n != 4 {
		t.Errorf("4: %d, %v", n, err)
	}
	for _, bad := range []string{"0", "-2", "many", "1.5"} {
		env[WorkersEnvVar] = bad
		_, err := workersFromLookup(lookup)
		var we *WorkersError
		if err == nil || !strings.Contains(err.Error(), WorkersEnvVar) || !errors.As(err, &we) {
			t.Errorf("%q: err = %v, want a WorkersError naming %s", bad, err, WorkersEnvVar)
		}
	}
}

// modelDirectories walks root and returns the model files of every directory
// holding one or more, each directory's files sorted.
func modelDirectories(t *testing.T, root string) [][]string {
	t.Helper()
	byDir := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || source.KindOf(path) == source.KindUnknown {
			return nil
		}
		byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	out := make([][]string, 0, len(dirs))
	for _, dir := range dirs {
		files := byDir[dir]
		sort.Strings(files)
		out = append(out, files)
	}
	return out
}

func readInputs(t *testing.T, paths []string) []Input {
	t.Helper()
	inputs := make([]Input, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, Input{Name: path, Content: content, Version: 1})
	}
	return inputs
}

// serialDiagnostics opens the inputs one by one and diagnoses each in turn, the
// path the editor takes, and renders the result for comparison.
func serialDiagnostics(inputs []Input) string {
	ws := NewWorkspace()
	for _, in := range inputs {
		ws.Open(in.Name, in.Content, in.Version)
	}
	var b strings.Builder
	for _, in := range inputs {
		renderDiagnostics(&b, in.Name, ws.Diagnostics(in.Name))
	}
	return b.String()
}

// batchDiagnostics opens the inputs as one batch on the given workers and
// diagnoses them as one batch, rendering the result as serialDiagnostics does.
func batchDiagnostics(t *testing.T, inputs []Input, workers int) string {
	t.Helper()
	ws := NewWorkspace()
	if err := ws.SetWorkers(workers); err != nil {
		t.Fatal(err)
	}
	ws.OpenAll(inputs)
	names := make([]string, len(inputs))
	for i, in := range inputs {
		names[i] = in.Name
	}
	var b strings.Builder
	for i, diags := range ws.DiagnosticsAll(names) {
		renderDiagnostics(&b, names[i], diags)
	}
	return b.String()
}

// renderDiagnostics writes every field of each diagnostic, in the order given,
// so a comparison sees content and order alike.
func renderDiagnostics(b *strings.Builder, name string, diags []passes.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintf(b, "%s: %+v\n", filepath.Base(name), d)
	}
}
