package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// unimportedLibraryCallSrc calls Kernel Function Library functions without
// importing their packages; importedLibraryCallSrc is the same model with the
// imports, abs binding to IntegerFunctions so that i holds an Integer.
const unimportedLibraryCallSrc = `package P {
	private import ScalarValues::*;
	attribute r : Real = sqrt(4.0);
	attribute i : Integer = abs(-2);
}
`

const importedLibraryCallSrc = `package P {
	private import ScalarValues::*;
	private import RealFunctions::*;
	private import IntegerFunctions::*;
	attribute r : Real = sqrt(4.0);
	attribute i : Integer = abs(-2);
}
`

const unimportedExtensionCallSrc = `package P {
	private import ScalarValues::*;
	attribute e : Real = exp(1.0);
}
`

// overloadedLibraryCallSrc imports several function packages that each declare
// the same names; every call binds to the declaration its argument fits.
const overloadedLibraryCallSrc = `package P {
	private import ScalarValues::*;
	private import IntegerFunctions::*;
	private import RealFunctions::*;
	private import RationalFunctions::*;
	private import ComplexFunctions::*;
	attribute a : Natural = abs(-2);
	attribute b : Real = abs(-2.5);
	attribute c : Real = abs(rect(3.0, 4.0));
	attribute z : Boolean = isZero(rect(0.0, 0.0));
}
`

func publishedDiagnostics(t *testing.T, name, src string) []string {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	ws.Open(name, []byte(src), 1)
	s.publishDiagnostics(context.Background(), name)
	if len(fc.published) != 1 {
		t.Fatalf("published count = %d, want 1", len(fc.published))
	}
	var out []string
	for _, d := range fc.published[0].Diagnostics {
		out = append(out, d.Message)
	}
	return out
}

// evalInModel loads src into a REPL session and evaluates each expression,
// which is what the editor's diagnostics are held to agree with.
func evalInModel(t *testing.T, src string, exprs ...string) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := repl.NewSession()
	if _, err := sess.LoadFile(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	var out []string
	for _, expr := range exprs {
		lines, err := sess.EvalExpr(expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		out = append(out, strings.Join(lines, "\n"))
	}
	return out
}

// The editor and the runtime agree on a bare library call: unimported, both
// report it unresolved; under the import, it checks clean and evaluates.
func TestPublishDiagnosticsAgreesWithRuntimeOnLibraryCall(t *testing.T) {
	diags := publishedDiagnostics(t, "lib.sysml", unimportedLibraryCallSrc)
	if len(diags) != 2 || !strings.Contains(diags[0], "unresolved reference: sqrt") || !strings.Contains(diags[1], "unresolved reference: abs") {
		t.Fatalf("diagnostics = %v, want both calls unresolved", diags)
	}
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(unimportedLibraryCallSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := repl.NewSession()
	if _, err := sess.LoadFile(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if lines, err := sess.EvalExpr("P::r"); err == nil {
		t.Fatalf("P::r evaluated to %v, want the import to be required", lines)
	} else if !strings.Contains(err.Error(), "unresolved reference: sqrt") {
		t.Fatalf("error %q does not report the unresolved call", err)
	}

	if diags := publishedDiagnostics(t, "lib_imported.sysml", importedLibraryCallSrc); len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	got := evalInModel(t, importedLibraryCallSrc, "P::r", "P::i")
	for i, want := range []string{"2.0", "2"} {
		if !strings.Contains(got[i], want) {
			t.Errorf("value %d = %q, want %q", i, got[i], want)
		}
	}
}

// An OpenSysML extension function stays import-gated: the editor reports the
// call the runtime refuses.
func TestPublishDiagnosticsKeepsExtensionFunctionImportGated(t *testing.T) {
	diags := publishedDiagnostics(t, "ext.sysml", unimportedExtensionCallSrc)
	if len(diags) != 1 || !strings.Contains(diags[0], "unresolved reference: exp") {
		t.Fatalf("diagnostics = %v, want the unresolved call", diags)
	}
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(unimportedExtensionCallSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := repl.NewSession()
	if _, err := sess.LoadFile(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if lines, err := sess.EvalExpr("P::e"); err == nil {
		t.Fatalf("P::e evaluated to %v, want the import to be required", lines)
	} else if !strings.Contains(err.Error(), "OpenSysMLMathFunctions") {
		t.Fatalf("error %q does not name the import", err)
	}
}

// Overloaded library calls check clean and evaluate through the declaration
// the checker selected.
func TestPublishDiagnosticsSelectsLibraryOverloadByArgumentType(t *testing.T) {
	if diags := publishedDiagnostics(t, "overload.sysml", overloadedLibraryCallSrc); len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	got := evalInModel(t, overloadedLibraryCallSrc, "P::a", "P::b", "P::c", "P::z")
	for i, want := range []string{"2", "2.5", "5.0", "true"} {
		if !strings.Contains(got[i], want) {
			t.Errorf("value %d = %q, want %q", i, got[i], want)
		}
	}
}
