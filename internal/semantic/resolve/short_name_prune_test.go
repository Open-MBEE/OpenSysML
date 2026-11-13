package resolve

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"strings"
	"testing"
)

func TestAllVisibleAbsentShortNameDoesNotDeriveSiblings(t *testing.T) {
	var text strings.Builder
	text.WriteString("package P { part def Base { ")
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&text, "attribute field%d; ", i)
	}
	text.WriteString("} part unit : Base { ")
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&text, "attribute :>> P::Base::field%d; ", i)
	}
	text.WriteString("} }")
	root := parsedRoot(t, "names.sysml", text.String())
	idx := symbols.NewIndexFromDoc("names.sysml", root)
	r := New(idx)
	scope := scopeOf(t, scopeOf(t, idx.DocumentRoot("names.sysml"), "P"), "unit")
	lookup := func() {
		r.inAllVisible(func() {
			if got := r.shortNamedMembers(scope, "Tag", nil); len(got) != 0 {
				t.Fatal("unexpected short name")
			}
		})
	}
	lookup()
	if allocs := testing.AllocsPerRun(5, lookup); allocs > 1 {
		t.Fatalf("absent short name allocated %.0f objects; want at most 1", allocs)
	}
}

func TestPrunedShortNameFollowsAddedAndRemovedDeclaration(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"lib.sysml":  "package Lib { }",
		"user.sysml": "package User { part base; ref part ::> base; ref part use ::> code; }",
	})
	w.analyzeAll()
	for _, src := range []string{"package Lib { attribute <code> code; }", "package Lib { }"} {
		dropped := w.put("lib.sysml", src)
		found := false
		for _, doc := range dropped {
			found = found || doc == "user.sysml"
		}
		if !found {
			t.Fatalf("short-name edit did not invalidate reader: %v", dropped)
		}
		w.analyzeAll()
	}
}

func TestDerivedShortNameStillResolvesInEveryVisibilityMode(t *testing.T) {
	for _, mode := range []string{"ordinary", "all-visible", "condition"} {
		t.Run(mode, func(t *testing.T) {
			idx := indexOf(t, map[string]string{"names.sysml": "package P { part def Base { attribute <code> field; } part unit : Base { attribute :>> P::Base::field; } }"})
			r := New(idx)
			scope := scopeOf(t, scopeOf(t, idx.DocumentRoot("names.sysml"), "P"), "unit")
			lookup := func() {
				members := r.shortNamedMembers(scope, "code", nil)
				if len(members) != 1 || members[0].OwnerScope != scope {
					t.Fatalf("derived local short name not offered: %v", members)
				}
				if got, ok := r.ResolveName(scope, "code", ident("code")); !ok || got == nil {
					t.Fatalf("derived short name unresolved: %v, %v", got, ok)
				}
			}
			switch mode {
			case "all-visible":
				r.inAllVisible(lookup)
			case "condition":
				r.InCondition(lookup)
			default:
				lookup()
			}
		})
	}
}
