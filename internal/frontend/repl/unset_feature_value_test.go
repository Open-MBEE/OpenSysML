package repl

import (
	"strings"
	"testing"
)

// unsetFeatureValueModel declares valueless features of value types — a library one, an
// attribute definition with no features, and a collection — beside a valued
// attribute and objects of classes.
const unsetFeatureValueModel = `package P {
    private import ScalarValues::*;
    attribute def Empty;
    attribute def Point { attribute x : Real = 1.0; }
    part def Engine;
    part def Q {
        attribute d : Real;
        attribute ds : Real[2];
        attribute empty : Empty;
        attribute origin : Point;
        attribute k : Real = 2.0;
        part engine : Engine;
    }
}
`

// A valueless feature of a value type holds no value, so the feature value listing says
// so rather than naming the object materialization holds for it. What does hold
// a value — a valued attribute, a value type with features, an object of a class
// — still reads as what it holds.
func TestFeatureValueListingReportsAValuelessValueTypedFeatureAsUnset(t *testing.T) {
	s := loadSource(t, unsetFeatureValueModel)
	wants(t, run(t, s, "%instantiate P::Q"), "Created instance")

	fvs := run(t, s, "%features P::Q")
	wants(t, fvs,
		"d = <unset>",
		"ds = [<unset>, <unset>]",
		"empty = <unset>",
		"k = 2.0",
		"origin = Instance(ID: ",
		"x = 1.0",
		"engine = Instance(ID: ",
	)
	// The object materialization holds for such a feature is not named, and not
	// expanded — only the class-typed part is reported as an object, with what it
	// inherits from the library.
	rejects(t, fvs, "d = Instance(", "empty = Instance(", "(no features)")
	if n := strings.Count(fvs, "    isSolid = true"); n != 1 {
		t.Errorf("%d nested objects expanded, want only the class-typed part:\n%s", n, fvs)
	}
}

// An evaluation of the same feature reports the same thing the feature value listing
// does: the two surfaces read one runtime value.
func TestEvaluationReportsAValuelessValueTypedFeatureAsUnset(t *testing.T) {
	s := loadSource(t, unsetFeatureValueModel)
	wants(t, run(t, s, "%instantiate P::Q"), "Created instance")

	wants(t, run(t, s, "%eval P::Q::d"), "= <unset>")
	wants(t, run(t, s, "%eval P::Q::k"), "= 2.0")
}

// A debugger session outlives a submission that does not rewrite what it runs,
// while the session rebuilds its own runtime context, so the run's results are
// read against the context that produced them.
func TestActionResultsReadAgainstTheContextThatProducedThem(t *testing.T) {
	s := loadSource(t, `private import ScalarValues::*;
part def Holder {
    attribute d : Real;
}
part holder : Holder;
action tally {
    attribute got = 0;
    action read {
        assign got := holder.d;
    }
    first start;
    then read;
    then done;
}
`)
	wants(t, run(t, s, "%action tally"), "Started action executor")

	if res := s.Submit("package Other { part def Unrelated; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated submission has diagnostics: %v", res.Diagnostics)
	}
	if s.actionExec == nil {
		t.Fatal("the debugger session ended on an unrelated submission")
	}
	if s.rtCtx == s.actionExec.contextOf() {
		t.Fatal("the session still holds the run's own context, so this no longer tests anything")
	}

	out := run(t, s, "%continue")
	wants(t, out, "got = <unset>")
	rejects(t, out, "got = Instance(")
}
