package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRuntimeRobustnessBindingConnectors covers binding connectors: conflicts, cycles, rings,
// valueless ends and the element budget spent by propagation.
func TestRuntimeRobustnessBindingConnectors(t *testing.T) {
	t.Run("binding_conflict", testBindingConflict)
	t.Run("binding_collection_conflicts_do_not_use_hashes", testBindingCollectionConflictsDoNotUseHashes)
	t.Run("binding_multiple_scalar_contributors", testBindingMultipleScalarContributors)
	t.Run("binding_multiple_collection_contributors", testBindingMultipleCollectionContributors)
	t.Run("binding_propagation_spends_element_budget", testBindingPropagationSpendsElementBudget)
	t.Run("binding_distinct_materialized_objects_conflict", testBindingDistinctMaterializedObjectsConflict)
	t.Run("binding_bare_ends_bind", testBindingBareEndsBind)
	t.Run("binding_incomplete_end_does_not_poison_read", testBindingIncompleteEndDoesNotPoisonRead)
	t.Run("binding_single_valueless", testBindingSingleValueless)
	t.Run("binding_cycle", testBindingCycle)
	t.Run("binding_three_binding_ring", testBindingThreeBindingRing)
	t.Run("binding_cycle_with_value", testBindingCycleWithValue)
	t.Run("binding_unrelated_expression_does_not_poison_read", testBindingUnrelatedExpressionDoesNotPoisonRead)
	t.Run("binding_result_tracks_later_mutation", testBindingResultTracksLaterMutation)
	t.Run("binding_nested_container_is_not_a_cycle", testBindingNestedContainerIsNotACycle)
	t.Run("binding_end_of_a_destroyed_object", testBindingEndOfADestroyedObject)
}

func testBindingConflict(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-conflict>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 1;
			attribute b = 2;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "b")
	if !errors.Is(err, errBindingConflict) {
		t.Fatalf("GetFeatureValue(b) = %v, want errBindingConflict", err)
	}
	if got, want := err.Error(), "binding conflict: b = 2, a = 1"; got != want {
		t.Errorf("conflict error = %q, want %q", got, want)
	}
}

func testBindingCollectionConflictsDoNotUseHashes(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-string-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a : ScalarValues::String[*] = ("a");
				attribute b : ScalarValues::String[*] = ("b");
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "b")
		if !errors.Is(err, errBindingConflict) {
			t.Fatalf("GetFeatureValue(b) = %v, want errBindingConflict", err)
		}
		if got, want := err.Error(),
			`binding conflict: b = ["b"], a = ["a"]`; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})

	t.Run("integers", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-integer-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a : Integer[*] = (1);
				attribute b : Integer[*] = (65537);
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "b")
		if !errors.Is(err, errBindingConflict) {
			t.Fatalf("GetFeatureValue(b) = %v, want errBindingConflict", err)
		}
		if got, want := err.Error(),
			"binding conflict: b = [65537], a = [1]"; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})
}

func testBindingMultipleScalarContributors(t *testing.T) {
	t.Run("unequal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-scalar-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b = 1;
				attribute c = 2;
				bind a = b;
				bind a = c;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "a")
		if !errors.Is(err, errBindingConflict) {
			t.Fatalf("GetFeatureValue(a) = %v, want errBindingConflict", err)
		}
		if got, want := err.Error(), "binding conflict at Sys.a: b = 1, c = 2"; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})

	t.Run("equal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-scalar-equal>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b = 1;
				attribute c = 1;
				bind a = b;
				bind a = c;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "a")
		if err != nil {
			t.Fatalf("GetFeatureValue(a): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 1 {
			t.Errorf("a = %d, want 1", got)
		}
	})
}

func testBindingMultipleCollectionContributors(t *testing.T) {
	t.Run("partial", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-collection-contributors>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute leftEdge : Integer[0..1] = (1);
				attribute rightEdge : Integer[0..1] = (2);
				binding [1] bind [0..1] edges = [0..1] leftEdge;
				binding [1] bind [0..1] edges = [0..1] rightEdge;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "edges")
		if !errors.Is(err, errBindingEnd) {
			t.Fatalf("GetFeatureValue(edges) = %v, want errBindingEnd", err)
		}
		if got, want := err.Error(), "binding end cannot be resolved: Sys.edges is bound by `binding [1] bind [0..1] edges = [0..1] leftEdge`, "+
			"which makes some value of edges a value of leftEdge without saying which value of either; the model does not state what edges holds"; got != want {
			t.Errorf("error = %q, want %q", got, want)
		}
		fv := inst.FeatureValues["edges"]
		if fv.Materialized || fv.Written || fv.BindingDerived || fv.HeldValue().Kind != ValInvalid {
			t.Errorf("unsupported binding left an assignment behind: %+v", *fv)
		}
	})

	// An end admitting one value links a feature holding one value whole,
	// however wide the feature is declared; one holding more stays partial.
	t.Run("partial_by_values_held", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-partial-by-values-held>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute pick : Integer[1];
				binding [1] bind [0..1] edges = [0..1] pick;
			}
			part one : Sys { :>> edges = (7); }
			part two : Sys { :>> edges = (7, 8); }
		}`))
		one, err := ctx.Instantiate(oneSymbol(t, idx, "P::one"))
		if err != nil {
			t.Fatalf("instantiate one: %v", err)
		}
		fv, err := one.GetFeatureValue(ctx, "pick")
		if err != nil {
			t.Fatalf("one.pick: %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 7 {
			t.Errorf("one.pick = %d (%s), want 7, the one value edges holds", got, FormatValue(fv.HeldValue()))
		}
		two, err := ctx.Instantiate(oneSymbol(t, idx, "P::two"))
		if err != nil {
			t.Fatalf("instantiate two: %v", err)
		}
		if _, err := two.GetFeatureValue(ctx, "pick"); !errors.Is(err, errBindingEnd) {
			t.Fatalf("two.pick = %v, want errBindingEnd", err)
		}
		// An end valued on its own — written, as one valued by a default — keeps that value.
		if err := two.SetFeatureValue(ctx, "pick", integerValue(8)); err != nil {
			t.Fatalf("write two.pick: %v", err)
		}
		fv, err = two.GetFeatureValue(ctx, "pick")
		if err != nil {
			t.Fatalf("two.pick after the write: %v", err)
		}
		if fv.HeldValue().Const.Int != 8 {
			t.Errorf("two.pick = %s after writing 8, want 8", FormatValue(fv.HeldValue()))
		}
	})

	// A binding of the whole feature determines it whatever partial bindings it has
	// besides, and in whatever order they are declared.
	t.Run("whole_beside_partial", func(t *testing.T) {
		for name, bindings := range map[string]string{
			"partial_first": "binding [1] bind [0..1] edges = [0..1] pick; bind edges = every;",
			"whole_first":   "bind edges = every; binding [1] bind [0..1] edges = [0..1] pick;",
		} {
			t.Run(name, func(t *testing.T) {
				idx, _, ctx := buildRuntime(t, "<binding-whole-beside-partial>", parseAndBuild(t, `package P {
					part def Sys {
						attribute edges : Integer[*];
						attribute every : Integer[*] = (1, 2, 3);
						attribute pick : Integer[0..1] = (2);
						`+bindings+`
					}
				}`))
				inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
				if err != nil {
					t.Fatalf("instantiate: %v", err)
				}
				fv, err := inst.GetFeatureValue(ctx, "edges")
				if err != nil {
					t.Fatalf("edges: %v", err)
				}
				if got := FormatValue(fv.HeldValue()); got != "[1, 2, 3]" {
					t.Errorf("edges = %s, want [1, 2, 3], what the whole binding determines", got)
				}
			})
		}
	})

	// An end of multiplicity [0] links no value: each feature reads what it holds on its own.
	t.Run("zero_width_end", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-zero-width-end>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*] = (7, 8);
				attribute pick : Integer[0..1];
				binding [1] bind [0] edges = [0..1] pick;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "edges")
		if err != nil {
			t.Fatalf("edges: %v", err)
		}
		if got := len(elementsOf(fv.HeldValue())); got != 2 {
			t.Errorf("edges holds %d values (%s), want the two it is valued with", got, FormatValue(fv.HeldValue()))
		}
		fv, err = inst.GetFeatureValue(ctx, "pick")
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		if got := elementsOf(fv.HeldValue()); len(got) != 0 {
			t.Errorf("pick = %s, want nothing: the binding links no value to it", FormatValue(fv.HeldValue()))
		}
	})

	// An end stating how many values it links is not met by a feature holding fewer: the
	// binding is a multiplicity violation, not a whole binding of what there is — whichever
	// end is read, whether the features admit more than the end links or exactly as many.
	t.Run("under_lower_bound", func(t *testing.T) {
		for name, c := range map[string]struct{ ends, same string }{
			"exact":  {"[2]", "[0..2]"},
			"ranged": {"[2..3]", "[0..3]"},
		} {
			ends := c.ends
			for shape, declared := range map[string]string{"wider": "[*]", "same": c.same} {
				t.Run(name+"_"+shape, func(t *testing.T) {
					idx, _, ctx := buildRuntime(t, "<binding-under-lower-bound>", parseAndBuild(t, `package P {
						part def Sys {
							attribute edges : Integer`+declared+` = (7);
							attribute pair : Integer`+declared+`;
							binding [1] bind `+ends+` edges = `+ends+` pair;
						}
					}`))
					want := "multiplicity violation: `binding [1] bind " + ends + " edges = " + ends + " pair` links " +
						ends + " of edges, which holds 1 value(s)"
					for _, order := range [][]string{{"pair", "edges", "pair"}, {"edges", "pair", "edges"}} {
						inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
						if err != nil {
							t.Fatalf("instantiate: %v", err)
						}
						for _, feature := range order {
							_, err := inst.GetFeatureValue(ctx, feature)
							if !errors.Is(err, ErrMultiplicityViolation) {
								t.Fatalf("%v: %s = %v, want ErrMultiplicityViolation", order, feature, err)
							}
							if got := err.Error(); got != want {
								t.Errorf("%v: %s error = %q, want %q", order, feature, got, want)
							}
						}
						fv := inst.FeatureValues["pair"]
						if fv.Materialized || fv.Written || fv.BindingDerived || fv.HeldValue().Kind != ValInvalid {
							t.Errorf("%v: the refused binding left an assignment behind: %+v", order, *fv)
						}
					}
				})
			}
		}
	})

	// An end that makes the binding partial does not excuse the other end from its
	// lower bound, whichever end is read.
	t.Run("under_lower_bound_beyond_partial_end", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-under-lower-bound-beyond-partial>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute pair : Integer[0..3] = (5);
				binding [1] bind [2] edges = [2] pair;
			}
		}`))
		want := "multiplicity violation: `binding [1] bind [2] edges = [2] pair` links [2] of pair, which holds 1 value(s)"
		for _, order := range [][]string{{"edges", "pair"}, {"pair", "edges"}} {
			inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
			if err != nil {
				t.Fatalf("instantiate: %v", err)
			}
			for _, feature := range order {
				_, err := inst.GetFeatureValue(ctx, feature)
				if !errors.Is(err, ErrMultiplicityViolation) {
					t.Fatalf("%v: %s = %v, want ErrMultiplicityViolation", order, feature, err)
				}
				if got := err.Error(); got != want {
					t.Errorf("%v: %s error = %q, want %q", order, feature, got, want)
				}
			}
			fv := inst.FeatureValues["edges"]
			if fv.Written || fv.BindingDerived || fv.HeldValue().Kind != ValInvalid {
				t.Errorf("%v: the refused binding left an assignment behind: %+v", order, *fv)
			}
		}
	})

	// Ends requiring a value are not met by two optional features holding none: the
	// binding links nothing, a multiplicity violation rather than an unknown value or a
	// cycle. Once anything values the features — a default, another binding — it is whole.
	t.Run("empty_required_ends", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-empty-required-ends>", parseAndBuild(t, `package P {
			part def Empty {
				attribute a : Integer[0..1];
				attribute b : Integer[0..1];
				binding [1] bind [1] a = [1] b;
			}
			part def Defaulted {
				attribute a : Integer[0..1] = 5;
				attribute b : Integer[0..1];
				binding [1] bind [1] a = [1] b;
			}
			part def Joined {
				attribute a : Integer[0..1];
				attribute b : Integer[0..1];
				attribute c : Integer[0..1] = 5;
				binding [1] bind [1] a = [1] b;
				binding [1] bind [1] a = [1] c;
			}
		}`))
		want := "multiplicity violation: `binding [1] bind [1] a = [1] b` links [1] of a, which holds 0 value(s)"
		for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
			inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Empty"))
			if err != nil {
				t.Fatalf("instantiate: %v", err)
			}
			for _, feature := range order {
				_, err := inst.GetFeatureValue(ctx, feature)
				if !errors.Is(err, ErrMultiplicityViolation) {
					t.Fatalf("Empty %v: %s = %v, want ErrMultiplicityViolation", order, feature, err)
				}
				if got := err.Error(); got != want {
					t.Errorf("Empty %v: %s error = %q, want %q", order, feature, got, want)
				}
			}
		}
		for def, orders := range map[string][][]string{
			"P::Defaulted": {{"a", "b"}, {"b", "a"}},
			"P::Joined":    {{"a", "b", "c"}, {"b", "a", "c"}, {"c", "b", "a"}},
		} {
			for _, order := range orders {
				inst, err := ctx.Instantiate(oneSymbol(t, idx, def))
				if err != nil {
					t.Fatalf("instantiate: %v", err)
				}
				for _, feature := range order {
					fv, err := inst.GetFeatureValue(ctx, feature)
					if err != nil {
						t.Fatalf("%s %v: %s: %v", def, order, feature, err)
					}
					if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Int != 5 {
						t.Errorf("%s %v: %s = %s, want 5", def, order, feature, FormatValue(got))
					}
				}
			}
		}
	})

	t.Run("whole_unequal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-collection-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute left : Integer[*] = (1, 2);
				attribute right : Integer[*] = (2, 1);
				bind edges = left;
				bind edges = right;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "edges")
		if !errors.Is(err, errBindingConflict) {
			t.Fatalf("GetFeatureValue(edges) = %v, want errBindingConflict", err)
		}
		if got, want := err.Error(), "binding conflict at Sys.edges: left = [1, 2], right = [2, 1]"; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})

	t.Run("whole_equal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-collection-equal>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute left : Integer[*] = (1, 2);
				attribute right : Integer[*] = (1, 2);
				bind edges = left;
				bind edges = right;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "edges")
		if err != nil {
			t.Fatalf("GetFeatureValue(edges): %v", err)
		}
		if got := FormatValue(fv.HeldValue()); got != "[1, 2]" {
			t.Errorf("edges = %s, want [1, 2]", got)
		}
	})
}

func testBindingPropagationSpendsElementBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-element-budget>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a : Integer[*] = (1, 2, 3);
			attribute b : Integer[*];
			bind b = a;
		}
	}`))
	ctx.maxElements = 2
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "b")
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("GetFeatureValue(b) = %v, want ErrElementLimitExceeded", err)
	}
}

func testBindingDistinctMaterializedObjectsConflict(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-distinct-objects>", parseAndBuild(t, `package P {
		part def A {
			attribute q = 1;
		}
		part def Sys {
			part p1 : A;
			part p2 : A;
			binding bind p1 = p2;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if _, err := inst.materializeFeatureValueIntrinsic(ctx, "p1"); err != nil {
		t.Fatalf("materialize p1: %v", err)
	}
	if _, err := inst.materializeFeatureValueIntrinsic(ctx, "p2"); err != nil {
		t.Fatalf("materialize p2: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "p1")
	if !errors.Is(err, errBindingConflict) {
		t.Fatalf("GetFeatureValue(p1) = %v, want errBindingConflict", err)
	}
}

// `binding bnd = a` states two ends (KerML.xtext BindingConnectorDeclaration);
// `bnd` is the first end, not the binding's name.
func testBindingBareEndsBind(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-bare-ends>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 5;
			attribute bnd;
			binding bnd = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "bnd")
	if err != nil {
		t.Fatalf("GetFeatureValue(bnd) = %v, want 5", err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Int != 5 {
		t.Errorf("bnd = %#v, want integer 5", got)
	}
}

func testBindingCycle(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-cycle>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a;
			attribute b;
			binding bind a = b;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "a")
	if !errors.Is(err, errBindingCycle) {
		t.Fatalf("GetFeatureValue(a) = %v, want errBindingCycle", err)
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Errorf("cycle error %q does not name both ends", err)
	}
}

func testBindingSingleValueless(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-single-valueless>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a;
			attribute b;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if got := fv.HeldValue(); got.Kind != ValInvalid {
			t.Errorf("%s = %#v, want an unknown value", name, got)
		}
	}
	sym := oneSymbol(t, idx, "P::Sys")
	expr := parser.New(source.New("<binding-single-valueless-eval>", []byte("b"))).ParseExpression()
	if _, err := ctx.EvalWithScopeOn(expr, sym.Scope, inst); !errors.Is(err, errUninitializedFeatureValue) {
		t.Fatalf("evaluating b = %v, want errUninitializedFeatureValue", err)
	}
}

func testBindingThreeBindingRing(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-three-ring>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a;
			attribute b;
			attribute c;
			binding bind a = b;
			binding bind b = c;
			binding bind c = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "a")
	if !errors.Is(err, errBindingCycle) {
		t.Fatalf("GetFeatureValue(a) = %v, want errBindingCycle", err)
	}
	for _, name := range []string{"a", "c"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("cycle error %q does not name %s", err, name)
		}
	}
}

func testBindingCycleWithValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-cycle-value>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 4;
			attribute b;
			binding bind a = b;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if fv.HeldValue().Kind != ValConst || fv.HeldValue().Const.Int != 4 {
			t.Errorf("%s = %#v, want integer 4", name, fv.HeldValue())
		}
	}
}

func testBindingUnrelatedExpressionDoesNotPoisonRead(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-unrelated-expression>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 5;
			attribute b;
			attribute sibling = 8;
			attribute b2 = a + 1;
			binding bind b = b2;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "sibling")
	if err != nil {
		t.Fatalf("GetFeatureValue(sibling): %v", err)
	}
	if got := fv.HeldValue().Const.Int; got != 8 {
		t.Errorf("sibling = %d, want 8", got)
	}
	fv, err = inst.GetFeatureValue(ctx, "b")
	if err != nil {
		t.Fatalf("GetFeatureValue(b): %v", err)
	}
	if got := fv.HeldValue().Const.Int; got != 6 {
		t.Errorf("b = %d, want 6", got)
	}
}

func testBindingResultTracksLaterMutation(t *testing.T) {
	t.Run("initially_unresolved", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-late-value>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b;
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if fv, err := inst.GetFeatureValue(ctx, "b"); err != nil {
			t.Fatalf("initial GetFeatureValue(b): %v", err)
		} else if got := fv.HeldValue(); got.Kind != ValInvalid {
			t.Fatalf("initial b = %#v, want an unknown value", got)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(9)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "b")
		if err != nil {
			t.Fatalf("later GetFeatureValue(b): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 9 {
			t.Errorf("later b = %d, want 9", got)
		}
	})

	t.Run("cached_value", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-late-change>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a = 3;
				attribute b;
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if fv, err := inst.GetFeatureValue(ctx, "b"); err != nil {
			t.Fatalf("initial GetFeatureValue(b): %v", err)
		} else if got := fv.HeldValue().Const.Int; got != 3 {
			t.Fatalf("initial b = %d, want 3", got)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(9)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "b")
		if err != nil {
			t.Fatalf("later GetFeatureValue(b): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 9 {
			t.Errorf("later b = %d, want 9", got)
		}
	})

	t.Run("derived_binding_chain", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-late-chain>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a = 3;
				attribute b;
				attribute c;
				binding bind b = a;
				binding bind c = b;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if fv, err := inst.GetFeatureValue(ctx, "c"); err != nil {
			t.Fatalf("initial GetFeatureValue(c): %v", err)
		} else if got := fv.HeldValue().Const.Int; got != 3 {
			t.Fatalf("initial c = %d, want 3", got)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(9)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "c")
		if err != nil {
			t.Fatalf("later GetFeatureValue(c): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 9 {
			t.Errorf("later c = %d, want 9", got)
		}
	})

	t.Run("written_both_ends_conflict", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-written-both-ends>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b;
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(1)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		if err := inst.SetFeatureValue(ctx, "b", constInt(2)); err != nil {
			t.Fatalf("assign b: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "b")
		if !errors.Is(err, errBindingConflict) {
			t.Fatalf("GetFeatureValue(b) = %v, want errBindingConflict", err)
		}
	})
}

func testBindingIncompleteEndDoesNotPoisonRead(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-incomplete-end>", parseAndBuild(t, `package P {
		part def Sys {
			attribute x = 4;
			bind x;
			binding bb of x;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "x")
	if err != nil {
		t.Fatalf("GetFeatureValue(x) = %v, want 4", err)
	}
	if got := fv.HeldValue().Const.Int; got != 4 {
		t.Errorf("x = %d, want 4", got)
	}
}

func testBindingNestedContainerIsNotACycle(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-nested-container>", parseAndBuild(t, `package P {
		part def Child {
			attribute b;
		}
		part def Sys {
			attribute x = 9;
			part child : Child;
			binding bind child.b = x;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	childValue, err := inst.GetFeatureValue(ctx, "child")
	if err != nil {
		t.Fatalf("GetFeatureValue(child) = %v, want no binding cycle", err)
	}
	id, isObject := childValue.HeldValue().Object()
	if !isObject {
		t.Fatalf("child holds %s, want an object", childValue.HeldValue().Kind)
	}
	child, ok := ctx.Instance(id)
	if !ok {
		t.Fatalf("child instance %d is not materialized", id)
	}
	b, err := child.GetFeatureValue(ctx, "b")
	if err != nil {
		t.Fatalf("GetFeatureValue(child.b) = %v, want 9", err)
	}
	if got := b.HeldValue().Const.Int; got != 9 {
		t.Fatalf("child.b = %d, want 9", got)
	}
}

// testBindingEndOfADestroyedObject: a binding end naming a destroyed object's
// feature is errOccurrenceDestroyed, never a stale read of it or a write into it.
func testBindingEndOfADestroyedObject(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			part def Widget { attribute n : Integer = 1; attribute m : Integer; }
			part def Rig {
				part w : Widget;
				attribute shown : Integer;
				bind shown = w.n;
				attribute knob : Integer = 9;
				bind w.m = knob;
			}
			calc def DestroyWidget { in w : Widget; return : Widget = destroy(w); }
		}
	`)
	rig := instantiate("Rig")
	fv, err := rig.GetFeatureValue(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := fv.HeldValue().Object()
	w, _ := ctx.getInstance(id)
	if _, err := invoke("DestroyWidget", objectValue(w)); err != nil {
		t.Fatalf("destroy(w): %v", err)
	}
	for _, name := range []string{"shown", "knob"} {
		_, err := rig.GetFeatureValue(ctx, name)
		if !errors.Is(err, errOccurrenceDestroyed) || !errors.Is(err, errBindingEnd) {
			t.Errorf("%s bound to a destroyed end: %v; want %v naming the binding end", name, err, errOccurrenceDestroyed)
		}
	}
	if m := w.FeatureValues["m"]; m.Materialized {
		t.Errorf("destroyed w.m = %s; want it left unwritten", FormatValue(m.HeldValue()))
	}
}
