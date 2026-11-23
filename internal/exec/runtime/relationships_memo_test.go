package runtime

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const relationshipsMemoModel = `package test {
	part def Rig {
		part panels[*];
		part solar[*] :> panels;
		part front :> solar, panels;
	}
	part rig : Rig;
}`

// relationshipQueryAllocs is what n repeated queries for one feature's `:>`
// relationships allocate after the first answered.
func relationshipQueryAllocs(t *testing.T, n int) float64 {
	t.Helper()
	ctx, idx := libraryShapeContext(t, relationshipsMemoModel)
	front := lookupOne(t, idx, "test::Rig::front")
	if rels := ctx.relationshipsOfKind(front, ast.RelSubsets); len(rels) != 2 {
		t.Fatalf("relationshipsOfKind(front, subsets) = %d relationships, want 2", len(rels))
	}
	return testing.AllocsPerRun(20, func() {
		for i := 0; i < n; i++ {
			ctx.relationshipsOfKind(front, ast.RelSubsets)
		}
	})
}

// A feature's relationships of a kind are read from its declaration once per
// model: twice the queries allocate no more than half of them.
func TestRelationshipsOfKindAreCollectedOncePerFeature(t *testing.T) {
	small, large := relationshipQueryAllocs(t, 50), relationshipQueryAllocs(t, 100)
	t.Logf("50 queries allocate %.0f objects, 100 queries %.0f", small, large)
	if large > small {
		t.Errorf("100 repeated relationship queries allocate %.0f objects, 50 allocate %.0f: the declaration is rescanned per query", large, small)
	}
}

// subsettingWalkAllocs is what n repeated subsetting walks over one type allocate
// after the first answered: the features front subsets, whether it reaches panels,
// and the features of rig subsetting panels.
func subsettingWalkAllocs(t *testing.T, n int) (related, transitive, subsetting float64) {
	t.Helper()
	ctx, idx := libraryShapeContext(t, relationshipsMemoModel)
	rig := instantiateQualified(t, ctx, idx, "test::rig")
	typ, front, panels := lookupOne(t, idx, "test::Rig"), lookupOne(t, idx, "test::Rig::front"), lookupOne(t, idx, "test::Rig::panels")
	if got := ctx.relatedFeatures(front, typ, ast.RelSubsets); len(got) != 2 {
		t.Fatalf("relatedFeatures(front, Rig, subsets) = %v, want solar and panels", got)
	}
	if !ctx.subsetsTransitively(front, typ, panels) {
		t.Fatal("subsetsTransitively(front, Rig, panels) = false, want true")
	}
	if got := ctx.SubsettingFeatures(rig, typ, "panels"); len(got) != 2 {
		t.Fatalf("SubsettingFeatures(rig, Rig, panels) = %v, want solar and front", featureNames(got))
	}
	related = testing.AllocsPerRun(20, func() {
		for i := 0; i < n; i++ {
			ctx.relatedFeatures(front, typ, ast.RelSubsets)
		}
	})
	transitive = testing.AllocsPerRun(20, func() {
		for i := 0; i < n; i++ {
			ctx.subsetsTransitively(front, typ, panels)
		}
	})
	subsetting = testing.AllocsPerRun(20, func() {
		for i := 0; i < n; i++ {
			ctx.SubsettingFeatures(rig, typ, "panels")
		}
	})
	return related, transitive, subsetting
}

// The subsetting walks a read repeats are answered once per model: twice the
// walks allocate no more than half of them.
func TestSubsettingWalksAreAnsweredOncePerModel(t *testing.T) {
	r1, t1, s1 := subsettingWalkAllocs(t, 50)
	r2, t2, s2 := subsettingWalkAllocs(t, 100)
	t.Logf("50 walks allocate related %.0f, transitive %.0f, subsetting %.0f; 100 walks %.0f, %.0f, %.0f", r1, t1, s1, r2, t2, s2)
	if r2 > r1 {
		t.Errorf("100 relatedFeatures walks allocate %.0f objects, 50 allocate %.0f: the relationships are re-resolved per walk", r2, r1)
	}
	if t2 > t1 {
		t.Errorf("100 subsetsTransitively walks allocate %.0f objects, 50 allocate %.0f: the chain is re-walked per question", t2, t1)
	}
	if s2 > s1 {
		t.Errorf("100 SubsettingFeatures reads allocate %.0f objects, 50 allocate %.0f: the held subsetters are re-filtered per read", s2, s1)
	}
}

// A symbol declaring no relationship of a kind takes no memo entry, and every
// answer is still what the declaration states.
func TestRelationshipsOfKindMemoizesOnlyDeclarers(t *testing.T) {
	ctx, idx := libraryShapeContext(t, relationshipsMemoModel)
	kinds := []ast.RelationshipKind{ast.RelSubsets, ast.RelRedefines, ast.RelSpecializes}
	for _, name := range []string{"front", "solar", "panels"} {
		sym := lookupOne(t, idx, "test::Rig::"+name)
		for _, kind := range kinds {
			want := declaredRelationshipsOfKind(sym, kind)
			for read := 0; read < 2; read++ {
				if got := ctx.relationshipsOfKind(sym, kind); !slices.Equal(got, want) {
					t.Errorf("%s, kind %v, read %d: %d relationships, want %d", name, kind, read, len(got), len(want))
				}
			}
			if _, memoized := ctx.model.relationships[relationshipsKey{sym, kind}]; memoized != (len(want) > 0) {
				t.Errorf("%s, kind %v: memoized = %v with %d relationships declared", name, kind, memoized, len(want))
			}
		}
	}
}
