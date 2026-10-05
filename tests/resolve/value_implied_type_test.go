package resolve_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// resolvedWithModel resolves src over the bundled libraries with a semantic
// model attached, as member lookups through a function's result parameter need,
// and returns the resolver's diagnostics sorted.
func resolvedWithModel(t *testing.T, src string) []string {
	t.Helper()
	p := parser.New(source.New("app.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("app.sysml", root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	r.ResolveDocument("app.sysml", root)
	var msgs []string
	for _, d := range r.Diagnostics {
		msgs = append(msgs, d.Message)
	}
	sort.Strings(msgs)
	return msgs
}

// An untyped action usage valued by an invocation is typed by the invoked
// definition, so a chain through it reads the definition's parameters, while a
// member the definition lacks is still unresolved.
func TestUntypedUsageWithInvocationValueReadsInvokedDefinition(t *testing.T) {
	got := resolvedWithModel(t, `package I {
	private import ScalarValues::*;
	action def Base {
		in x : Integer default 3;
		in z : Integer default 7;
		out r : Integer;
		first start;
		then assign r := z;
		then done;
	}
	action caller {
		out attribute result : Integer;
		out attribute other : Integer;
		first start;
		then action run = Base(x = 3, z = 7);
		then action keep { assign result := run.r; assign other := run.nope; }
		then done;
	}
}`)
	want := []string{"unresolved member: nope"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("diagnostics = %q, want %q", got, want)
	}
}

// The result of invoking a function is its result parameter, of invoking an
// action usage the usage, and of `new T()` an instance of T; each gives the
// untyped feature it values the members to read.
func TestUntypedUsageValueResultByExpressionKind(t *testing.T) {
	got := resolvedWithModel(t, `package P {
	private import ScalarValues::*;
	part def Car { attribute wheels : Integer; }
	calc def mk { return : Car; }
	calc mkUsage : mk;
	attribute viaCalcDef = mk();
	attribute w1 = viaCalcDef.wheels;
	attribute viaCalcUsage = mkUsage();
	attribute w2 = viaCalcUsage.wheels;
	attribute w3 = viaCalcUsage.nope;
	action def Base { in x : Integer default 3; out r : Integer; }
	action baseUsage : Base;
	action viaActionUsage = baseUsage();
	attribute w4 = viaActionUsage.r;
	part viaNew = new Car();
	attribute w5 = viaNew.wheels;
	attribute w6 = viaNew.nope;
}`)
	want := []string{"unresolved member: nope", "unresolved member: nope"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("diagnostics = %q, want %q", got, want)
	}
}

// A value types the feature only where KerML checkFeatureValuationSpecialization
// applies: a literal gives no feature to read members from, and a default value,
// a directed feature or one with a declared specialization is not typed by its
// value at all.
func TestUntypedUsageWithOtherValuesGainsNoMembers(t *testing.T) {
	got := resolvedWithModel(t, `package P {
	private import ScalarValues::*;
	part def Car { attribute wheels : Integer; }
	part def Boat { attribute hull : Integer; }
	part typedCar : Car;
	attribute viaLiteral = "abc";
	attribute w1 = viaLiteral.nope;
	part refDefault default typedCar;
	attribute w2 = refDefault.wheels;
	part boat : Boat;
	part refSub :> boat = typedCar;
	attribute w3 = refSub.wheels;
	attribute w4 = refSub.hull;
	action def Base { in x : Integer default 3; out r : Integer; }
	action a {
		in part refIn = typedCar;
		attribute w5 = refIn.wheels;
		action run = Base(x = 3);
		attribute w6 = run.r;
	}
}`)
	want := []string{"unresolved member: nope", "unresolved member: wheels", "unresolved member: wheels", "unresolved member: wheels"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("diagnostics = %q, want %q", got, want)
	}
}
