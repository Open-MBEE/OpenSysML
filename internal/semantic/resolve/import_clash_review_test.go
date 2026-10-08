// SPDX-License-Identifier: Apache-2.0

package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestRootImportClashPreservesGlobalDeclaration(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml":    `package Left { part def Engine; } package Right { part def Engine; }`,
		"app.sysml":    `private import Left::*; private import Right::*;`,
		"global.sysml": `part def Engine;`,
	})
	r := New(idx)
	want, _ := idx.DocumentRoot("global.sysml").LookupLocal("Engine")
	got, ok := r.LookupName(idx.DocumentRoot("app.sysml"), "Engine")
	if !ok || !symbols.SameElement(got, want) {
		t.Fatalf("root clash fallback = %v, want independently declared Engine", got)
	}
}

func TestHiddenReexportPreservesIndependentImport(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"model.sysml": `package Left { part def Engine; } package Right { part def Engine; }
package Other { part def Engine; }
package Middle { public import Left::*; public import Right::*; }
package Consumer { private import Middle::*; private import Other::*; }`,
	})
	r := New(idx)
	root := idx.DocumentRoot("model.sysml")
	want, _ := scopeOf(t, root, "Other").LookupLocal("Engine")
	got, ok := r.LookupName(scopeOf(t, root, "Consumer"), "Engine")
	if !ok || !symbols.SameElement(got, want) {
		t.Fatalf("import = %v, want Other::Engine", got)
	}
}

func TestGlobalRootImportClash(t *testing.T) {
	for _, other := range []string{"Left", "Right"} {
		t.Run(other, func(t *testing.T) {
			idx := indexOf(t, map[string]string{
				"model.sysml": `package Left { namespace Engine { namespace Member; } }
package Right { namespace Engine { namespace Member; } }
private import Left::*; private import ` + other + `::*;`,
			})
			r := New(idx)
			_, ok := r.ResolveQualified(idx.DocumentRoot("model.sysml"), qn(true, "Engine", "Member"))
			if ok {
				t.Fatal("root imports must not become globally visible")
			}
		})
	}
}
