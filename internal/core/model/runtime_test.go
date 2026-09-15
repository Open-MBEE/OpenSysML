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
