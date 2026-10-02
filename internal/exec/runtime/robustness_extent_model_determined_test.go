package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRuntimeRobustnessExtentModelDetermined covers the failure and determinism modes of
// the namespace-owned usages an extent reaches: a binding connector between objects must
// refuse ends whose values differ or that resolve through each other, an abstract
// collection must refuse the subsetters that leave it short of its lower bound, the
// objects `all T` enumerates must not depend on the order usages were read, a binding
// must carry into an adopted context, and a probe must unwind one on rollback.
func TestRuntimeRobustnessExtentModelDetermined(t *testing.T) {
	t.Run("a namespace binding refusing conflicting ends leaves nothing behind", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car = new Car();
			part b : Car = new Car();
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "a")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("a = %v, want binding conflict", err)
		}
		for _, name := range []string{"test::a", "test::b"} {
			sym := lookupOne(t, idx, name)
			if ids := ctx.occurrences[sym]; len(ids) != 0 {
				t.Fatalf("occurrences[%s] = %v, want none: the refused binding records no occurrences", name, ids)
			}
		}
		if len(ctx.instances) != 2 {
			t.Fatalf("%d objects materialized, want the two stated values only", len(ctx.instances))
		}
	})

	t.Run("a namespace binding cycle is refused", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car = b;
			part b : Car = a;
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "a")
		if !errors.Is(err, ErrCyclicFeatureValue) {
			t.Fatalf("a = %v, want cyclic feature value dependency", err)
		}
	})

	t.Run("a class of two differing valued members leaves nothing bound", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car = new Car();
			part b : Car = new Car();
			part c : Car;
			bind a = b;
			bind b = c;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "c")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("c = %v, want binding conflict", err)
		}
		sym := lookupOne(t, idx, "test::c")
		if ids := ctx.occurrences[sym]; len(ids) != 0 {
			t.Fatalf("occurrences[test::c] = %v, want none: the refused class records no occurrences", ids)
		}
		if _, ok := ctx.namespaceBindings[sym]; ok {
			t.Fatal("c is bound, want nothing bound after the refused class")
		}
	})

	t.Run("the class's value is the same whichever member is read first", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car;
			bind a = b;
			part e : Car;
			bind b = e;
		}`
		extents := make([][]int64, 2)
		for i, first := range []string{"e", "a"} {
			ctx, idx := contextForSource(t, src)
			pkg := lookupOne(t, idx, "test")
			if _, err := evalIn(t, ctx, pkg.Scope, first); err != nil {
				t.Fatalf("%s: %v", first, err)
			}
			for _, expr := range []string{"a === e", "a === b"} {
				same, err := evalIn(t, ctx, pkg.Scope, expr)
				if err != nil || FormatValue(same) != "true" {
					t.Fatalf("%s after %s = %v (%v), want true", expr, first, FormatValue(same), err)
				}
			}
			all, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
			if err != nil {
				t.Fatalf("all test::Car: %v", err)
			}
			extents[i] = heldObjects(all)
		}
		if !slices.Equal(extents[0], extents[1]) {
			t.Fatalf("extent read from e first = %v, from a first = %v", extents[0], extents[1])
		}
		if len(extents[0]) != 1 {
			t.Fatalf("extent = %v, want the class's one object", extents[0])
		}
	})

	t.Run("an optional subsetter contributes its object once", func(t *testing.T) {
		src := `package test {
			part def Car;
			part vs : Car[1];
			part opt : Car[0..1] :> vs;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		allCold, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car: %v", err)
		}
		if ids := heldObjects(allCold); len(ids) != 1 {
			t.Fatalf("cold all test::Car = %v, want the one object the optional subsetter fills", ids)
		}
		var firstIDs []int64
		for i := 0; i < 2; i++ {
			got, err := evalIn(t, ctx, pkg.Scope, "vs")
			if err != nil {
				t.Fatalf("vs read %d: %v", i, err)
			}
			ids := heldObjects(got)
			if len(ids) != 1 {
				t.Fatalf("vs read %d = %v, want the one member the optional subsetter contributes", i, ids)
			}
			if i == 0 {
				firstIDs = ids
			} else if !slices.Equal(ids, firstIDs) {
				t.Fatalf("vs read warm = %v, cold = %v, want the same object", ids, firstIDs)
			}
		}
		allWarm, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car warm: %v", err)
		}
		if !slices.Equal(heldObjects(allWarm), heldObjects(allCold)) {
			t.Fatalf("warm extent = %v, cold = %v", heldObjects(allWarm), heldObjects(allCold))
		}
	})

	t.Run("a class's materialized members honor every member's multiplicity", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car[2];
			part b : Car[2];
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		aVal, err := evalIn(t, ctx, pkg.Scope, "a")
		if err != nil {
			t.Fatalf("a: %v", err)
		}
		if ids := heldObjects(aVal); len(ids) != 2 {
			t.Fatalf("a = %v, want its two lower-bound occurrences", ids)
		}
		bVal, err := evalIn(t, ctx, pkg.Scope, "b")
		if err != nil {
			t.Fatalf("b: %v", err)
		}
		if !slices.Equal(heldObjects(bVal), heldObjects(aVal)) {
			t.Fatalf("b = %v, a = %v, want the same identities in the same order", heldObjects(bVal), heldObjects(aVal))
		}
		all, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car: %v", err)
		}
		if ids := heldObjects(all); len(ids) != 2 {
			t.Fatalf("all test::Car = %v, want the class's two occurrences", ids)
		}
	})

	t.Run("a class whose members' multiplicities disagree is refused and leaves nothing", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car[2];
			part b : Car[3];
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "a")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("a = %v, want binding conflict: two occurrences cannot be b's [3]", err)
		}
		for _, name := range []string{"test::a", "test::b"} {
			sym := lookupOne(t, idx, name)
			if ids := ctx.occurrences[sym]; len(ids) != 0 {
				t.Fatalf("occurrences[%s] = %v, want none: the refused class records no occurrences", name, ids)
			}
			if _, ok := ctx.namespaceBindings[sym]; ok {
				t.Fatalf("%s is bound, want nothing bound after the refused class", name)
			}
		}
		if len(ctx.instances) != 0 {
			t.Fatalf("%d objects materialized, want none after the refused class", len(ctx.instances))
		}
	})

	t.Run("a member's existing occurrence joins its class's sources", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car = new Car();
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")
		aSym := lookupOne(t, idx, "test::a")

		inst, err := ctx.materialize(aSym, 0, nil, "")
		if err != nil {
			t.Fatalf("materialize a: %v", err)
		}
		ctx.occurrences[aSym] = []int64{inst.ID}
		if _, err := evalIn(t, ctx, pkg.Scope, "a"); !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("a = %v, want binding conflict: a's recorded occurrence differs from b's value", err)
		}
	})

	t.Run("a member's existing occurrence matching its class's value is no conflict", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car = new Car();
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")
		aSym := lookupOne(t, idx, "test::a")
		bSym := lookupOne(t, idx, "test::b")

		ec := NewEvalContext(ctx, bSym.OwnerScope)
		bVal, err := ec.declaredValue(bSym, bSym.Decl.(*ast.Usage).Value)
		if err != nil {
			t.Fatalf("b's declared value: %v", err)
		}
		ctx.occurrences[aSym] = heldObjects(bVal)
		got, err := evalIn(t, ctx, pkg.Scope, "a")
		if err != nil {
			t.Fatalf("a = %v: a's recorded occurrence is b's value, so there is no conflict", err)
		}
		if !slices.Equal(heldObjects(got), heldObjects(bVal)) {
			t.Fatalf("a = %v, want b's occurrence %v", heldObjects(got), heldObjects(bVal))
		}
		same, err := evalIn(t, ctx, pkg.Scope, "a === b")
		if err != nil || FormatValue(same) != "true" {
			t.Fatalf("a === b = %v (%v), want true: one recorded object is the member's scalar value", FormatValue(same), err)
		}
	})

	t.Run("a member's recorded occurrences match a multi-valued source", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car[2];
			part b : Car[2] = (new Car(), new Car());
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")
		aSym := lookupOne(t, idx, "test::a")
		bSym := lookupOne(t, idx, "test::b")

		ec := NewEvalContext(ctx, bSym.OwnerScope)
		bVal, err := ec.declaredValue(bSym, bSym.Decl.(*ast.Usage).Value)
		if err != nil {
			t.Fatalf("b's declared value: %v", err)
		}
		if ids := heldObjects(bVal); len(ids) != 2 {
			t.Fatalf("b's declared value = %v, want two occurrences", ids)
		}
		ctx.occurrences[aSym] = heldObjects(bVal)
		got, err := evalIn(t, ctx, pkg.Scope, "a")
		if err != nil {
			t.Fatalf("a = %v: a's recorded occurrences are b's value, so there is no conflict", err)
		}
		if !slices.Equal(heldObjects(got), heldObjects(bVal)) {
			t.Fatalf("a = %v, want b's occurrences %v", heldObjects(got), heldObjects(bVal))
		}
	})

	t.Run("a class materializes the largest member lower bound", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car[0..1];
			part b : Car;
			bind a = b;
		}`
		extents := make([][]int64, 2)
		for i, first := range []string{"a", "b"} {
			ctx, idx := contextForSource(t, src)
			pkg := lookupOne(t, idx, "test")
			got, err := evalIn(t, ctx, pkg.Scope, first)
			if err != nil {
				t.Fatalf("%s first: %v", first, err)
			}
			if ids := heldObjects(got); len(ids) != 1 {
				t.Fatalf("%s first = %v, want the one occurrence the class materializes", first, ids)
			}
			all, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
			if err != nil {
				t.Fatalf("all test::Car after %s first: %v", first, err)
			}
			extents[i] = heldObjects(all)
		}
		if !slices.Equal(extents[0], extents[1]) || len(extents[0]) != 1 {
			t.Fatalf("extent read from a first = %v, from b first = %v, want the same one object", extents[0], extents[1])
		}
	})

	t.Run("a class whose largest lower bound a member cannot hold is refused", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car[0..1];
			part b : Car[2];
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "b")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("b = %v, want binding conflict: two occurrences cannot be a's [0..1]", err)
		}
		for _, name := range []string{"test::a", "test::b"} {
			sym := lookupOne(t, idx, name)
			if ids := ctx.occurrences[sym]; len(ids) != 0 {
				t.Fatalf("occurrences[%s] = %v, want none: the refused class records no occurrences", name, ids)
			}
			if _, ok := ctx.namespaceBindings[sym]; ok {
				t.Fatalf("%s is bound, want nothing bound after the refused class", name)
			}
		}
		if len(ctx.instances) != 0 {
			t.Fatalf("%d objects materialized, want none after the refused class", len(ctx.instances))
		}
	})

	t.Run("a member's classifier behavior reads the class's one object", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			part def Car {
				attribute probe : Car;
				exhibit state tally {
					entry; then on;
					state on { entry action peek { assign probe := a; } }
				}
			}
			part a : Car;
			part b : Car;
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		if _, err := evalIn(t, ctx, pkg.Scope, "a"); err != nil {
			t.Fatalf("a: %v", err)
		}
		all, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car: %v", err)
		}
		if ids := heldObjects(all); len(ids) != 1 {
			t.Fatalf("all test::Car = %v, want the class's one object: a behavior reading a member must not materialize another", ids)
		}
	})

	t.Run("a scope registered after the index builds still resolves its bindings", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car;
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")
		if _, err := evalIn(t, ctx, pkg.Scope, "a"); err != nil {
			t.Fatalf("a: %v", err)
		}

		file := parser.New(source.New("<test2>", []byte(`package other {
			part def Car;
			part c : Car;
			part d : Car;
			bind c = d;
		}`))).ParseFile()
		idx.AddDocument("<test2>", file)
		scope := idx.DocumentRoot("<test2>")
		ctx.Model().RegisterScope(scope)

		same, err := evalIn(t, ctx, scope, "other::c === other::d")
		if err != nil {
			t.Fatalf("other::c === other::d: %v", err)
		}
		if FormatValue(same) != "true" {
			t.Fatalf("other::c === other::d = %s, want true: the registered tree's binding must resolve", FormatValue(same))
		}
	})

	t.Run("an abstract collection under-count is refused", func(t *testing.T) {
		src := `package test {
			part def Car;
			abstract part vs : Car[2];
			part c1 : Car[1] :> vs;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "vs")
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Fatalf("vs = %v, want multiplicity violation", err)
		}
		if !strings.Contains(err.Error(), "vs") {
			t.Fatalf("vs = %v, want the violation naming vs", err)
		}
	})

	t.Run("the extent is the same cold and after the members were read", func(t *testing.T) {
		src := `package test {
			part def Car;
			abstract part vs : Car[1..*];
			part c1 : Car[1] :> vs;
			part c2 : Car[1] :> vs;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		cold, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car: %v", err)
		}
		coldIDs := heldObjects(cold)
		for _, expr := range []string{"c1", "c2", "vs"} {
			if _, err := evalIn(t, ctx, pkg.Scope, expr); err != nil {
				t.Fatalf("%s: %v", expr, err)
			}
		}
		warm, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car: %v", err)
		}
		warmIDs := heldObjects(warm)
		if !slices.Equal(warmIDs, coldIDs) {
			t.Fatalf("extent after the reads = %v, cold extent = %v", warmIDs, coldIDs)
		}
	})

	t.Run("adoption keeps a namespace binding", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car;
			bind a = b;
		}`
		prev, _ := contextForSource(t, src)
		prevPkg := lookupOne(t, prev.model.resolver.Index(), "test")
		got, err := evalIn(t, prev, prevPkg.Scope, "a")
		if err != nil {
			t.Fatalf("a: %v", err)
		}
		obj := prev.instances[heldObjects(got)[0]]

		ctx := contextOver(t, src)
		pkg := lookupOne(t, ctx.model.resolver.Index(), "test")
		if _, err := ctx.Adopt(prev, prev.ShapesOf(obj), obj); err != nil {
			t.Fatalf("Adopt: %v", err)
		}
		carried, err := evalIn(t, ctx, pkg.Scope, "a")
		if err != nil {
			t.Fatalf("a after adoption: %v", err)
		}
		if heldObjects(carried)[0] != obj.ID {
			t.Fatalf("a after adoption = object %d, want carried object %d", heldObjects(carried)[0], obj.ID)
		}
		same, err := evalIn(t, ctx, pkg.Scope, "a === b")
		if err != nil {
			t.Fatalf("a === b: %v", err)
		}
		if FormatValue(same) != "true" {
			t.Fatalf("a === b after adoption = %s, want true", FormatValue(same))
		}
	})

	t.Run("a probe undo forgets a namespace binding", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car;
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		end := ctx.beginProbe()
		if _, err := evalIn(t, ctx, pkg.Scope, "a"); err != nil {
			t.Fatalf("a: %v", err)
		}
		end()
		if len(ctx.namespaceBindings) != 0 {
			t.Fatalf("namespace bindings after undo = %v, want none", ctx.namespaceBindings)
		}
		same, err := evalIn(t, ctx, pkg.Scope, "a === b")
		if err != nil {
			t.Fatalf("a === b: %v", err)
		}
		if FormatValue(same) != "true" {
			t.Fatalf("a === b after undo = %s, want true", FormatValue(same))
		}
	})
}
