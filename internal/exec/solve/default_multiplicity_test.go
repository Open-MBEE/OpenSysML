package solve

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// conformanceFixture indexes one runtime conformance case over the standard
// library, as the solve differential reads it.
func conformanceFixture(t *testing.T, name string) (*symbols.Index, func(fqn string) (*Query, error)) {
	t.Helper()
	path := filepath.Join(conformanceDir, name)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ctx, idx := fixture(t, path, string(src))
	return idx, func(fqn string) (*Query, error) {
		sym := symbolNamed(t, idx, fqn)
		return Constraint(ctx, sym, sym.OwnerScope)
	}
}

// A KerML feature that declares and inherits no multiplicity has the default
// `0..*` (KerML 1.1 §7.4.12), whatever count its value happens to have: the
// solver reads it as one value, says so in the query, and asserts no count.
func TestUnstatedKerMLFeatureIsReadAsOneUnderAnAssumption(t *testing.T) {
	for _, name := range []string{
		"solve_default_multiplicity_kerml_none.kerml",
		"solve_default_multiplicity_kerml_one.kerml",
		"solve_default_multiplicity_kerml_two.kerml",
	} {
		t.Run(name, func(t *testing.T) {
			_, translate := conformanceFixture(t, name)
			q, err := translate("test::Tank::nonNegative")
			if err != nil {
				t.Fatalf("translate: %v", err)
			}
			if len(q.AssumedOne) != 1 || q.AssumedOne[0].Name != "test::Tank::level" {
				t.Fatalf("AssumedOne = %+v, want test::Tank::level alone", q.AssumedOne)
			}
			if got := q.AssumedOne[0].Multiplicity.Text(); got != "[0..*]" {
				t.Errorf("assumed multiplicity %s, want the default 0..*", got)
			}
			for _, a := range q.Assertions {
				if a.From.Role == RoleDomain && strings.Contains(a.From.Condition, "multiplicity") {
					t.Errorf("the query asserts a count: %s", a.From.Condition)
				}
			}
			if len(q.Vars) != 1 || q.Vars[0].Name != "test::Tank::level" {
				t.Errorf("vars = %v, want one variable for level", q.Vars)
			}
		})
	}
}

// An attribute usage owned by a definition with no owned subsetting or
// redefinition has the implicit `[1..1]` (SysML v2 §7.6.3): it is held to one,
// so nothing is assumed.
func TestImplicitOneUsageIsReadAsOneWithoutAssumption(t *testing.T) {
	_, translate := conformanceFixture(t, "solve_implicit_multiplicity_usage.sysml")
	q, err := translate("test::Tank::nonNegative")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(q.AssumedOne) != 0 {
		t.Errorf("AssumedOne = %+v, want none for an implicit [1..1] usage", q.AssumedOne)
	}
}

// A redefining attribute declaring no multiplicity inherits the redefined
// declaration's: under `[1]` it is one value and nothing is assumed; under
// `[0..*]` it is a stated collection, which the translation refuses.
func TestRedefinitionInheritsTheStatedBound(t *testing.T) {
	_, translate := conformanceFixture(t, "solve_redefinition_inherits_multiplicity.sysml")
	q, err := translate("test::Derived::levelNonNegative")
	if err != nil {
		t.Fatalf("translate levelNonNegative: %v", err)
	}
	if len(q.AssumedOne) != 0 {
		t.Errorf("AssumedOne = %+v, want none for a redefinition inheriting [1]", q.AssumedOne)
	}
	q, err = translate("test::Derived::readingsNonNegative")
	if err == nil {
		t.Fatalf("translating readingsNonNegative succeeded, want a refusal; script:\n%s", Script(q))
	}
	var refused *NotTranslatableError
	if !errors.As(err, &refused) {
		t.Fatalf("translating readingsNonNegative: %v, want a *NotTranslatableError", err)
	}
	if refused.Construct != "feature `readings`" || refused.Reason != "it may hold more than one value" {
		t.Errorf("refused %q: %q, want feature `readings`: it may hold more than one value", refused.Construct, refused.Reason)
	}
}

// A parameter stating no multiplicity is a reference usage, which §7.6.3 leaves
// at the default `0..*`: the canonical `constraint def { in x; … }` is read under
// the assumption, once per feature however often it is read.
func TestUnstatedParameterIsAssumedOnce(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def C {
				in a : Integer;
				attribute b : Integer;
				assert constraint { a > 0 and a < 9 and b > a }
			}
		}`, "test::C")
	if len(q.AssumedOne) != 1 || q.AssumedOne[0].Name != "test::C::a" {
		t.Fatalf("AssumedOne = %+v, want test::C::a alone (b has the implicit [1..1])", q.AssumedOne)
	}
}

// craftModel holds a part only the default multiplicity governs (`spare`, with
// an owned subsetting), whose type asserts the condition given.
func craftModel(tankCondition string) string {
	return `
		package test {
			private import ScalarValues::Real;
			part def Tank {
				attribute level : Real;
				assert constraint { ` + tankCondition + ` }
			}
			part def Craft {
				attribute pressure : Real;
				assert constraint { pressure > 0.0 }
				part main : Tank;
				part spare : Tank :> main;
			}
		}`
}

// The one-object assumption about a held part is reported only with a group the
// query states: a part whose assertions the translation leaves out, as
// Uncovered says, assumes nothing.
func TestHeldObjectAssumptionFollowsTheTranslatedGroups(t *testing.T) {
	ctx, idx := fixture(t, "<test>", craftModel("level > 0.0"))
	craft := symbolNamed(t, idx, "test::Craft")
	q, err := AssertedOn(ctx, craft, craft.OwnerScope, nil, nil)
	if err != nil {
		t.Fatalf("translate Craft: %v", err)
	}
	if len(q.AssumedOne) != 1 || q.AssumedOne[0].Name != "spare" || !q.AssumedOne[0].Object {
		t.Fatalf("AssumedOne = %+v, want spare as one object", q.AssumedOne)
	}

	ctx, idx = fixture(t, "<test>", craftModel("level ** 2.0 > 0.0"))
	craft = symbolNamed(t, idx, "test::Craft")
	q, err = AssertedOn(ctx, craft, craft.OwnerScope, nil, nil)
	if err != nil {
		t.Fatalf("translate Craft: %v", err)
	}
	if len(q.Uncovered) == 0 {
		t.Fatalf("the power expression under spare was translated; Uncovered = %+v", q.Uncovered)
	}
	if len(q.AssumedOne) != 0 {
		t.Errorf("AssumedOne = %+v, want none: nothing of spare is in the query", q.AssumedOne)
	}
}
