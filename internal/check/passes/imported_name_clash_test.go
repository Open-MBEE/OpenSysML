package passes

import "testing"

// KerML 7.2.5.4 hides conflicting imported memberships, retaining repeated
// imports of the same element and ordinary owned/outer-scope lookup.
func TestImportedNameClashVisibility(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		clean        bool
	}{
		{"two-imports", "package Left { part def Engine; }\npackage Right { part def Engine; }\npackage Consumer {\nprivate import Left::*;\nprivate import Right::*;\npart engine : Engine;\n}", false},
		{"two-imports-reversed", "package Left { part def Engine; }\npackage Right { part def Engine; }\npackage Consumer {\nprivate import Right::*;\nprivate import Left::*;\npart engine : Engine;\n}", false},
		{"recursive", "package Models { package A { part def Engine; } package B { part def Engine; } }\npackage Consumer { private import Models::**; part engine : Engine; }", false},
		{"recursive-reversed", "package Models { package B { part def Engine; } package A { part def Engine; } }\npackage Consumer { private import Models::**; part engine : Engine; }", false},
		{"qualified-control", "package Left { part def Engine; }\npackage Right { part def Engine; }\npackage Consumer {\nprivate import Left::*;\nprivate import Right::*;\npart engine : Left::Engine;\n}", true},
		{"owned-control", "package Left { part def Engine; }\npackage Right { part def Engine; }\npackage Consumer {\nprivate import Left::*;\nprivate import Right::*;\npart def Engine;\npart engine : Engine;\n}", true},
		{"outer-control", "package Left { part def Engine; }\npackage Right { part def Engine; }\npart def Engine;\npackage Consumer {\nprivate import Left::*;\nprivate import Right::*;\npart engine : Engine;\n}", true},
		{"same-element-twice", "package Left { part def Engine; }\npackage Right { part def Engine; }\npackage Consumer { private import Left::*; private import Left::*; part engine : Engine; }", true},
		{"single-import-shadows-outer", "package Left { part def Engine; }\npackage Right { part def Engine; }\npart def Engine; package Consumer { private import Left::*; part engine : Engine; }", true},
		{"reexport-conflict", "package Left { part def Engine; }\npackage Right { part def Engine; }\npackage Middle { public import Left::*; public import Right::*; } package Consumer { private import Middle::*; part engine : Engine; }", false},
		{"qualified-conflict", "package Left { part def Engine; }\npackage Right { part def Engine; }\npackage Middle { public import Left::*; public import Right::*; } package Consumer { part engine : Middle::Engine; }", false},
		{"same-target-reexport", "package Left { part def Engine; } package Middle { public import Left::*; } package Consumer { private import Left::*; private import Middle::*; part engine : Engine; }", true},
		{"aliased-same-element", "package Left { part def Engine; } package Right { public alias Engine for Left::Engine; } package Consumer { private import Left::*; private import Right::*; part engine : Engine; }", true},
		{"short-name-clash", "package Left { part def <E> Engine; } package Right { part def E; } package Consumer { private import Left::*; private import Right::*; part engine : E; }", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.clean {
				wantLibraryClean(t, tc.source)
				return
			}
			diags := libraryDiags(t, tc.source)
			if !hasCode(diags, "unresolved") {
				t.Fatalf("hidden imported name must not bind: %v", diags)
			}
		})
	}
}
