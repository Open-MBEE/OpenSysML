package model_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// A root package named like a standard library package is resolved by the
// workspace's own declaration from every document, as it is within one file:
// an import of it written in a package of another document reaches the user's
// root, not the library's.
func TestPackageImportAcrossDocumentsShadowsLibraryRoot(t *testing.T) {
	for _, imp := range []string{"private import Base::*;", "public import Base::*;"} {
		for _, baseFirst := range []bool{true, false} {
			name := imp + " baseFirst=" + map[bool]string{true: "true", false: "false"}[baseFirst]
			t.Run(name, func(t *testing.T) {
				docs := map[string][]byte{
					"base.sysml":    []byte("package Base { part def Heater; part def Timer; }"),
					"chapter.sysml": []byte("package Chapter { " + imp + " part def Toaster { part heater : Heater; part timer : Timer; } }"),
				}
				order := []string{"base.sysml", "chapter.sysml"}
				if !baseFirst {
					order = []string{"chapter.sysml", "base.sysml"}
				}
				ws := model.NewWorkspace()
				var inputs []model.Input
				for _, n := range order {
					inputs = append(inputs, model.Input{Name: n, Content: docs[n], Version: 1})
				}
				ws.OpenAll(inputs)

				for _, n := range order {
					var errs []string
					for _, d := range ws.Diagnostics(n) {
						if d.Severity == diag.SeverityError {
							errs = append(errs, d.Message)
						}
					}
					if len(errs) > 0 {
						t.Errorf("%s: unexpected error diagnostics: %v", n, errs)
					}
				}

				doc := ws.Document("chapter.sysml")
				if doc == nil {
					t.Fatal("no document chapter.sysml")
				}
				var heaterFound, timerFound bool
				for _, ref := range resolve.References(doc.AST, doc.Scope) {
					if ref.QN == nil {
						continue
					}
					sym, ok := ws.ResolveReferenceInDoc("chapter.sysml", ref)
					switch ref.QN.Text() {
					case "Heater":
						heaterFound = true
						if !ok || sym.DocName != "base.sysml" {
							t.Errorf("Heater resolved to %v (ok=%v), want a symbol of base.sysml", symbolID(sym), ok)
						}
					case "Timer":
						timerFound = true
						if !ok || sym.DocName != "base.sysml" {
							t.Errorf("Timer resolved to %v (ok=%v), want a symbol of base.sysml", symbolID(sym), ok)
						}
					}
				}
				if !heaterFound || !timerFound {
					t.Errorf("references not found: Heater=%v Timer=%v", heaterFound, timerFound)
				}
			})
		}
	}
}

// Control: a root package whose name no library document claims is reached
// across documents too.
func TestPackageImportAcrossDocumentsNonLibraryRoot(t *testing.T) {
	ws := model.NewWorkspace()
	ws.OpenAll([]model.Input{
		{Name: "basis.sysml", Content: []byte("package Basis { part def Wheel; }"), Version: 1},
		{Name: "car.sysml", Content: []byte("package Car { private import Basis::*; part def Sedan { part wheel : Wheel; } }"), Version: 1},
	})
	for _, n := range []string{"basis.sysml", "car.sysml"} {
		for _, d := range ws.Diagnostics(n) {
			if d.Severity == diag.SeverityError {
				t.Errorf("%s: unexpected error diagnostic: %s", n, d.Message)
			}
		}
	}
}
