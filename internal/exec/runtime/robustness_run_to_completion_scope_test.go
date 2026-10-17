package runtime

import (
	"errors"
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

func TestRuntimeRobustnessRunToCompletionScope(t *testing.T) {
	t.Run("scope_not_ancestor", func(t *testing.T) {
		err := stateExecutorError(t, readRunToCompletionFixture(t, "state_run_to_completion_scope_not_ancestor.sysml"), "Machine")
		var refusal *lower.RunToCompletionRedefinition
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want run-to-completion redefinition", err)
		}
		if !errors.Is(err, lower.ErrUnsupportedStateContent) {
			t.Fatalf("error = %v, want unsupported state content", err)
		}
		if !refusal.NotAncestor {
			t.Fatalf("refusal = %+v, want non-ancestor scope", refusal)
		}
	})

	t.Run("scope_no_occurrence", func(t *testing.T) {
		err := stateExecutorError(t, readRunToCompletionFixture(t, "state_run_to_completion_scope_no_occurrence.sysml"), "Machine")
		var refusal *lower.RunToCompletionRedefinition
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want run-to-completion redefinition", err)
		}
		if !errors.Is(err, lower.ErrUnsupportedStateContent) {
			t.Fatalf("error = %v, want unsupported state content", err)
		}
		if refusal.NotAncestor {
			t.Fatalf("refusal = %+v, want unresolved scope", refusal)
		}
	})

	t.Run("value_not_boolean", func(t *testing.T) {
		err := stateExecutorError(t, `
			package test {
				state def Machine {
					entry; then idle;
					state idle parallel {
						attribute :>> isRunToCompletion = 1;
						state inner {
							entry; then leaf;
							state leaf;
						}
					}
				}
			}
		`, "Machine")
		var valueErr *runToCompletionValueError
		if !errors.As(err, &valueErr) {
			t.Fatalf("error = %v, want run-to-completion value error", err)
		}
	})
	t.Run("run_to_completion_redefined_false", testRunToCompletionRedefinedFalse)
	t.Run("run_to_completion_scope_narrowed", testRunToCompletionScopeNarrowed)
	t.Run("run_to_completion_redefined_by_specialized_def", testRunToCompletionRedefinedBySpecializedDef)
	t.Run("run_to_completion_redefined_in_orthogonal_region", testRunToCompletionRedefinedInOrthogonalRegion)
	t.Run("run_to_completion_redefined_undecidably", testRunToCompletionRedefinedUndecidably)
	t.Run("run_to_completion_redefined_through_alias", testRunToCompletionRedefinedThroughAlias)
	t.Run("run_to_completion_redefined_through_redefining_feature", testRunToCompletionRedefinedThroughRedefiningFeature)
	t.Run("run_to_completion_model_feature_under_library_name", testRunToCompletionModelFeatureUnderLibraryName)
	t.Run("run_to_completion_defaults_restated", testRunToCompletionDefaultsRestated)
	t.Run("run_to_completion_default_restored_by_specialization", testRunToCompletionDefaultRestoredBySpecialization)
	t.Run("run_to_completion_default_masked_by_specialization", testRunToCompletionDefaultMaskedBySpecialization)
}

func readRunToCompletionFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/robustness/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// testRunToCompletionRedefinedFalse: a machine redefining isRunToCompletion to
// false executes under the declared non-run-to-completion behavior.
func testRunToCompletionRedefinedFalse(t *testing.T) {
	if err := stateExecutorError(t, `
		package test {
			state def Machine {
				attribute :>> isRunToCompletion = false;
				entry; then idle;
				state idle;
			}
		}
	`, "Machine"); err != nil {
		t.Fatalf("false redefinition failed: %v", err)
	}
}

// testRunToCompletionScopeNarrowed: a substate redefining runToCompletionScope to
// itself executes with the declared state-local scope.
func testRunToCompletionScopeNarrowed(t *testing.T) {
	if err := stateExecutorError(t, `
		package test {
			state def Machine {
				entry; then idle;
				state idle {
					ref :>> runToCompletionScope = self;
				}
			}
		}
	`, "Machine"); err != nil {
		t.Fatalf("scope narrowing failed: %v", err)
	}
}

// testRunToCompletionRedefinedBySpecializedDef: the effective inherited
// redefinition on a specialized state definition executes on the machine.
func testRunToCompletionRedefinedBySpecializedDef(t *testing.T) {
	if err := stateExecutorError(t, `
		package test {
			state def Base {
				attribute :>> isRunToCompletion = false;
				entry; then idle;
				state idle;
			}
			state def Machine :> Base;
		}
	`, "Machine"); err != nil {
		t.Fatalf("inherited redefinition failed: %v", err)
	}
}

// testRunToCompletionRedefinedInOrthogonalRegion: a substate of an orthogonal
// region carrying the redefinition executes through the lowered state path.
func testRunToCompletionRedefinedInOrthogonalRegion(t *testing.T) {
	if err := stateExecutorError(t, `
		package test {
			state def Machine parallel {
				state left {
					entry; then l1;
					state l1 {
						attribute :>> isRunToCompletion = false;
					}
				}
				state right {
					entry; then r1;
					state r1;
				}
			}
		}
	`, "Machine"); err != nil {
		t.Fatalf("orthogonal redefinition failed: %v", err)
	}
}

// testRunToCompletionRedefinedUndecidably: a value lowering that cannot verify
// the library default is refused rather than assumed to restate it.
func testRunToCompletionRedefinedUndecidably(t *testing.T) {
	if err := stateExecutorError(t, `
		package test {
			state def Machine {
				attribute strict : Boolean = true;
				attribute :>> isRunToCompletion = strict or true;
				entry; then idle;
				state idle;
			}
		}
	`, "Machine"); err != nil {
		t.Fatalf("expression redefinition failed: %v", err)
	}
}

// testRunToCompletionDefaultsRestated: redefinitions restating the library
// defaults execute with the default scheduling behavior.
func testRunToCompletionDefaultsRestated(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state def Machine {
				attribute :>> isRunToCompletion = true;
				ref :>> runToCompletionScope = self;
				entry; then idle;
				state idle {
					attribute :>> isRunToCompletion = true;
				}
			}
		}
	`, "Machine")
	if err != nil {
		t.Fatalf("restating the defaults was refused: %v", err)
	}
}

// testRunToCompletionDefaultRestoredBySpecialization: a redefinition restating
// the default masks the inherited definition on the machine and substate.
func testRunToCompletionDefaultRestoredBySpecialization(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state def Base {
				attribute :>> isRunToCompletion = false;
				entry; then idle;
				state idle : Leaf {
					attribute :>> isRunToCompletion = true;
				}
			}
			state def Leaf {
				attribute :>> isRunToCompletion = false;
			}
			state def Machine :> Base {
				attribute :>> isRunToCompletion = true;
			}
		}
	`, "Machine")
	if err != nil {
		t.Fatalf("restoring the default over an inherited redefinition was refused: %v", err)
	}
}

// testRunToCompletionDefaultMaskedBySpecialization: the machine's effective
// redefinition is judged over and executes instead of the inherited one it masks.
func testRunToCompletionDefaultMaskedBySpecialization(t *testing.T) {
	if err := stateExecutorError(t, `
		package test {
			state def Base {
				attribute :>> isRunToCompletion = true;
				entry; then idle;
				state idle;
			}
			state def Machine :> Base {
				attribute :>> isRunToCompletion = false;
			}
		}
	`, "Machine"); err != nil {
		t.Fatalf("specialized redefinition failed: %v", err)
	}
}

// testRunToCompletionRedefinedThroughAlias: a redefinition naming the library
// feature through an alias resolves to it and executes by its target.
func testRunToCompletionRedefinedThroughAlias(t *testing.T) {
	err := libraryStateExecutorError(t, `
		package test {
			alias Rtc for Occurrences::Occurrence::isRunToCompletion;
			state def Machine {
				attribute :>> Rtc = false;
				entry; then idle;
				state idle;
			}
		}
	`, "Machine")
	if err != nil {
		t.Fatalf("alias redefinition failed: %v", err)
	}
}

// testRunToCompletionRedefinedThroughRedefiningFeature: a redefinition of a
// feature that itself redefines the library one reaches and executes it.
func testRunToCompletionRedefinedThroughRedefiningFeature(t *testing.T) {
	err := libraryStateExecutorError(t, `
		package test {
			state def Base {
				attribute strict :>> isRunToCompletion;
				entry; then idle;
				state idle;
			}
			state def Machine :> Base {
				attribute :>> strict = false;
			}
		}
	`, "Machine")
	if err != nil {
		t.Fatalf("redefining-feature route failed: %v", err)
	}
}

// testRunToCompletionModelFeatureUnderLibraryName: a model's own feature
// declared under the library's qualified name is an ordinary attribute, so
// redefining it to false executes without changing library RTC semantics.
func testRunToCompletionModelFeatureUnderLibraryName(t *testing.T) {
	const src = `
		package Occurrences {
			state def Occurrence {
				attribute isRunToCompletion = true;
				entry; then idle;
				state idle;
			}
		}
		package test {
			state def Machine :> Occurrences::Occurrence {
				attribute :>> isRunToCompletion = false;
			}
		}
	`
	for name, build := range map[string]func(*testing.T, string, string) error{
		"without library": stateExecutorError,
		"with library":    libraryStateExecutorError,
	} {
		if err := build(t, src, "Machine"); err != nil {
			t.Fatalf("%s: a model's own isRunToCompletion was taken for the library's: %v", name, err)
		}
	}
}
