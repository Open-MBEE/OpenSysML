package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// errorsIn returns the error diagnostics of one document, joined.
func errorsIn(t *testing.T, ws *Workspace, uri string) string {
	t.Helper()
	var errs []string
	for _, d := range ws.Diagnostics(uri) {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
		}
	}
	return strings.Join(errs, "; ")
}

// A usage's `that` names the object featuring its values, so a chain from it
// reads the members of that object's type ([KerML, 8.4.2]) — `that` itself is
// declared as Anything and owns none.
func TestThatChainsThroughTheFeaturingType(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "a member of the featuring type resolves",
			src: `package T { private import ScalarValues::*;
				part def P { attribute a : Real = 1.0; }
				part p : P { attribute b : Real = that.a; } }`,
		},
		{
			name: "a member the usage itself declares resolves",
			src: `package T { private import ScalarValues::*;
				part def P;
				part p : P { attribute q : Real = 2.0; attribute b : Real = that.q; } }`,
		},
		{
			name: "a member the featuring type does not own is reported",
			src: `package T { private import ScalarValues::*;
				part def P { attribute a : Real = 1.0; }
				part p : P { attribute b : Real = that.zzz; } }`,
			want: "unresolved member: zzz",
		},
		{
			name: "a chain of two members resolves through the featuring type",
			src: `package T { private import ScalarValues::*;
				part def Inner { attribute q : Real = 1.0; }
				part def P { part i : Inner; }
				part p : P { attribute b : Real = that.i.q; } }`,
		},
		{
			name: "the last member of a chain of two is checked",
			src: `package T { private import ScalarValues::*;
				part def Inner { attribute q : Real = 1.0; }
				part def P { part i : Inner; }
				part p : P { attribute b : Real = that.i.zzz; } }`,
			want: "unresolved member: zzz",
		},
		{
			name: "the base usage contributes `that` to usages only",
			src: `package T { private import ScalarValues::*;
				attribute c : Real = that.a; }`,
			want: "unresolved reference: that",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := NewWorkspace()
			ws.Open("t.sysml", []byte(tc.src), 1)
			got := errorsIn(t, ws, "t.sysml")
			if tc.want == "" {
				if got != "" {
					t.Fatalf("unexpected error(s): %s", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("diagnostics %q do not contain %q", got, tc.want)
			}
		})
	}
}

// A root-level wildcard import surfaces its names in the importing document's
// own root namespace, so no visibility on it reaches another document (KerML
// 8.2.3.3): each document imports the library it names, or qualifies the name.
func TestRootImportServesItsOwnDocumentOnly(t *testing.T) {
	for _, importer := range []string{
		"public import ScalarValues::*;",
		"protected import ScalarValues::*;",
		"private import ScalarValues::*;",
		"",
	} {
		name := importer
		if name == "" {
			name = "no import"
		}
		t.Run(name, func(t *testing.T) {
			ws := NewWorkspace()
			ws.Open("a.sysml", []byte(importer+"\npackage A { attribute here : Real; }\n"), 1)
			ws.Open("b.sysml", []byte("package B { attribute there : Real; }\n"), 1)

			// The importing document resolves the name; the other one does not.
			// A non-private root import is itself invalid (KerML
			// validateImportTopLevelVisibility), which is not a resolution error.
			if importer != "" {
				got := strings.ReplaceAll(errorsIn(t, ws, "a.sysml"),
					"Top level import must be private", "")
				if strings.Trim(got, "; ") != "" {
					t.Errorf("importing document reports %q", got)
				}
			}
			if got := errorsIn(t, ws, "b.sysml"); !strings.Contains(got, "unresolved reference: Real") {
				t.Errorf("other document reports %q, want Real unresolved", got)
			}
		})
	}
}
