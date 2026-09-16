package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Named takes the document's own declaration, else the one workspace document's;
// two declaring documents are an error naming both, none — or a library only — nil.
func TestRuntimeNamedAcrossDocuments(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("views.sysml", []byte("package Views { part def Local; }\n"), 1)
	ws.Open("machines.sysml", []byte("package Machines { state def Ops; part def Local; }\n"), 1)
	rt, err := ws.NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if sym, err := rt.Named("views.sysml", "Machines::Ops"); err != nil || sym == nil || sym.DocName != "machines.sysml" {
		t.Errorf("Named(views.sysml, Machines::Ops) = %v, %v; want the declaration in machines.sysml", sym, err)
	}
	if sym, err := rt.Named("views.sysml", "Local"); err != nil || sym == nil || sym.DocName != "views.sysml" {
		t.Errorf("Named(views.sysml, Local) = %v, %v; want the document's own", sym, err)
	}
	if sym, err := rt.Named("views.sysml", "Machines::Nope"); err != nil || sym != nil {
		t.Errorf("Named(views.sysml, Machines::Nope) = %v, %v; want nil", sym, err)
	}
	if sym, err := rt.Named("views.sysml", "ScalarValues::Integer"); err != nil || sym != nil {
		t.Errorf("Named(views.sysml, ScalarValues::Integer) = %v, %v; want nil for a library's declaration", sym, err)
	}

	ws.Open("spare.sysml", []byte("package Machines { state def Ops; }\n"), 1)
	if rt, err = ws.NewRuntime(); err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if sym, err := rt.Named("views.sysml", "Machines::Ops"); err == nil || sym != nil || !strings.Contains(err.Error(), "machines.sysml, spare.sysml") {
		t.Errorf("Named(views.sysml, Machines::Ops) = %v, %v; want an error naming both documents", sym, err)
	}
	if sym, err := rt.Named("machines.sysml", "Machines::Ops"); err != nil || sym == nil || sym.DocName != "machines.sysml" {
		t.Errorf("Named(machines.sysml, Machines::Ops) = %v, %v; want the document's own", sym, err)
	}
}

// A runtime over a caller-built index holds the caller's documents too, bare or
// over a frozen base: the workspace's document specializes definitions only the
// caller indexed, and an object of it carries the feature inherited from one.
func TestNewRuntimeKeepsCallerIndexedDocuments(t *testing.T) {
	stdlib, _ := libs.SharedLibrary()
	for _, tc := range []struct {
		name string
		idx  *symbols.Index
	}{
		{"bare", symbols.NewIndex()},
		{"overlay", symbols.NewOverlay(stdlib)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lib := []byte("package Lib {\n    part def Base;\n}\n")
			tc.idx.AddDocumentWithKind("lib.sysml", parser.New(source.New("lib.sysml", lib)).ParseFile(), source.KindSysML)
			tc.idx.MarkLibraryDocument("lib.sysml", symbols.LibraryDocument{Tier: symbols.TierLibrary, Digest: symbols.TextDigest(lib)})
			shared := []byte("package Shared {\n    part def Chassis {\n        attribute mass = 42;\n    }\n}\n")
			tc.idx.AddDocumentWithKind("shared.sysml", parser.New(source.New("shared.sysml", shared)).ParseFile(), source.KindSysML)
			tc.idx.ExpandWildcardImports()
			ws := NewWorkspaceWithIndex(tc.idx)
			ws.Open("main.sysml", []byte("package Main {\n    part def Robot :> Lib::Base, Shared::Chassis;\n}\n"), 1)

			rt, err := ws.NewRuntime()
			if err != nil {
				t.Fatalf("NewRuntime: %v", err)
			}
			if rt.Declared("lib.sysml", "Lib::Base") == nil {
				t.Fatal("runtime does not hold Lib::Base")
			}
			robot := rt.Declared("main.sysml", "Main::Robot")
			if robot == nil {
				t.Fatal("Main::Robot not declared")
			}
			ctx := runtime.NewContext(rt.Model(), 1000)
			obj, err := ctx.Instantiate(robot)
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			mass, err := obj.GetFeatureValue(ctx, "mass")
			if err != nil {
				t.Fatalf("GetFeatureValue(mass): %v", err)
			}
			if got := mass.Value; got.Kind != runtime.ValConst || !strings.Contains(fmt.Sprint(got.Const), "42") {
				t.Errorf("mass = %v, want the value Shared::Chassis states", got)
			}
		})
	}
}
