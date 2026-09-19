package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// nestedUsageOutputs evaluates every output of the named calc usage in src.
func nestedUsageOutputs(t *testing.T, src, usageName string) []CalcOutputValue {
	t.Helper()

	return nestedUsageOutputsAtDepth(t, src, usageName, DefaultMaxCalcDepth)
}

// nestingProbeDepth is the calc depth budget the cases about that bound run
// under, so such a case states a model the size of the bound rather than of the
// default, which a terminating recursion is sized for.
const nestingProbeDepth = 32

// nestedUsageOutputsAtDepth is nestedUsageOutputs under a stated calc depth
// budget.
func nestedUsageOutputsAtDepth(t *testing.T, src, usageName string, maxCalcDepth int64) []CalcOutputValue {
	t.Helper()

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxCalcDepth = maxCalcDepth
	sym := findSymbolByName(idx.DocumentRoot("<test>"), usageName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc usage %s not found", usageName)
	}
	outputs, err := ctx.CalcUsageOutputs(sym, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("reading the outputs of %s: %v", usageName, err)
	}
	return outputs
}

// outputValue is the value of one named output among those read.
func outputValue(t *testing.T, outputs []CalcOutputValue, name string) Value {
	t.Helper()

	for _, out := range outputs {
		if out.Name == name {
			return out.Value
		}
	}
	t.Fatalf("no output named %s among %d read", name, len(outputs))
	return Value{}
}

// wantReal asserts an output took a real value, within arithmetic tolerance.
func wantReal(t *testing.T, outputs []CalcOutputValue, name string, want float64) {
	t.Helper()

	value := outputValue(t, outputs, name)
	if value.Kind != ValConst || value.Const.Kind != semantics.ValReal {
		t.Fatalf("%s = %s, want a real", name, FormatTraceValue(value))
	}
	if diff := value.Const.Real - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("%s = %v, want %v", name, value.Const.Real, want)
	}
}

// wantInt asserts an output took an integer value.
func wantInt(t *testing.T, outputs []CalcOutputValue, name string, want int64) {
	t.Helper()

	value := outputValue(t, outputs, name)
	if value.Kind != ValConst || value.Const.Int != want {
		t.Errorf("%s = %s, want %d", name, FormatTraceValue(value), want)
	}
}

// A usage declared among a calc's members binds its inputs from the invocation
// reading it, which is what makes a multi-output calc reusable.
func TestNestedCalcUsageBindsFromEnclosingParameters(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import RealFunctions::*;
			calc def Steering {
				in svx : Real;
				in svh : Real;
				out speed = sqrt(svx * svx + svh * svh);
				out ux = 0.0 - svx / speed;
				out uh = 0.0 - svh / speed;
			}
			calc def Rhs {
				in vx : Real;
				in vh : Real;
				in thrust : Real;
				calc st : Steering { in svx = vx; in svh = vh; }
				out dvx = thrust * st.ux;
				out dvh = thrust * st.uh - 1.62;
			}
			calc rhsAt : Rhs { in vx = 150.0; in vh = -20.0; in thrust = 4.0; }
		}
	`
	outputs := nestedUsageOutputs(t, src, "rhsAt")
	wantReal(t, outputs, "dvx", -3.964911602730538)
	wantReal(t, outputs, "dvh", -1.0913451196359283)
}

// The environment a nested usage binds in holds the enclosing computation's
// locals, not only its parameters.
func TestNestedCalcUsageBindsFromEnclosingLocals(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in n : Integer;
				out a = n * 2;
				out b = n * 3;
			}
			calc def Outer {
				in m : Integer;
				attribute doubled : Integer = m + m;
				calc inner : Twice { in n = doubled; }
				out a = inner.a;
				out b = inner.b;
			}
			calc c : Outer { in m = 5; }
		}
	`
	outputs := nestedUsageOutputs(t, src, "c")
	wantInt(t, outputs, "a", 20)
	wantInt(t, outputs, "b", 30)
}

// A usage nested in a calc itself read from a nested usage binds through the
// whole chain: composition is not limited to one level.
func TestNestedCalcUsageChain(t *testing.T) {
	src := `
		package test {
			calc def Inner {
				in n : Integer;
				out a = n + 1;
				out b = n + 2;
			}
			calc def Middle {
				in n : Integer;
				calc deep : Inner { in n = n * 10; }
				out a = deep.a;
				out b = deep.b;
			}
			calc def Outer {
				in n : Integer;
				calc mid : Middle { in n = n + 1; }
				out a = mid.a;
				out b = mid.b;
			}
			calc c : Outer { in n = 2; }
		}
	`
	outputs := nestedUsageOutputs(t, src, "c")
	wantInt(t, outputs, "a", 31)
	wantInt(t, outputs, "b", 32)
}

// An input named as the parameter it binds from reads that parameter rather
// than depending on itself.
func TestNestedCalcUsageShadowsEnclosingName(t *testing.T) {
	src := `
		package test {
			calc def Split {
				in n : Integer;
				out low = n - 1;
				out high = n + 1;
			}
			calc def Span {
				in n : Integer;
				calc s : Split { in n = n; }
				out width = s.high - s.low;
				out top = s.high;
			}
			calc c : Span { in n = 5; }
		}
	`
	outputs := nestedUsageOutputs(t, src, "c")
	wantInt(t, outputs, "width", 2)
	wantInt(t, outputs, "top", 6)
}

// Every input of a nested usage binds in the enclosing environment, so one
// input naming another's name reads the enclosing feature, not the sibling
// binding just written.
func TestNestedCalcUsageInputsDoNotSeeSiblings(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				in m : Integer;
				out d = n * 10 + m;
			}
			calc def Outer {
				in n : Integer;
				in m : Integer;
				calc s : Two { in n = m; in m = n; }
				out d = s.d;
			}
			calc c : Outer { in n = 1; in m = 2; }
		}
	`
	outputs := nestedUsageOutputs(t, src, "c")
	wantInt(t, outputs, "d", 21)
}

// An output the usage itself binds is evaluated in the enclosing environment,
// so two invocations within one run do not share the first one's value.
func TestNestedCalcUsageOwnOutputPerInvocation(t *testing.T) {
	src := `
		package test {
			calc def Base {
				out a = 0;
			}
			calc def Outer {
				in m : Integer;
				calc s : Base { out a = m * 2; }
				out d = s.a;
			}
			calc def Pair {
				calc one : Outer { in m = 1; }
				calc two : Outer { in m = 2; }
				out d1 = one.d;
				out d2 = two.d;
			}
			calc p : Pair;
		}
	`
	outputs := nestedUsageOutputs(t, src, "p")
	wantInt(t, outputs, "d1", 2)
	wantInt(t, outputs, "d2", 4)
}

// One nested usage read from two enclosing invocations whose arguments differ
// computes each invocation's values, not the first's twice, within one run
// where both reach the same memoized runs.
func TestNestedCalcUsageRunsPerInputs(t *testing.T) {
	src := `
		package test {
			calc def Split {
				in n : Integer;
				out low = n - 1;
				out high = n + 1;
			}
			calc def Span {
				in n : Integer;
				calc s : Split { in n = n; }
				out top = s.high;
			}
			calc def Both {
				calc five : Span { in n = 5; }
				calc ten : Span { in n = 10; }
				out ofFive = five.top;
				out ofTen = ten.top;
			}
			calc both : Both;
		}
	`
	outputs := nestedUsageOutputs(t, src, "both")
	wantInt(t, outputs, "ofFive", 6)
	wantInt(t, outputs, "ofTen", 11)
}

// An input a nested usage redeclares without binding a value keeps the default
// its definition declares, which is evaluated in the calc that wrote it — not
// in the enclosing environment the usage's own bindings are written in.
func TestNestedCalcUsageInheritedDefaultOfRedeclaredInput(t *testing.T) {
	src := `
		package test {
			calc def Doubler {
				in a : Integer;
				in b : Integer = a * 2;
				out d = b;
			}
			calc def Outer {
				in a : Integer;
				calc u : Doubler { in a = 1; in :>> b; }
				out d = u.d;
			}
			calc c : Outer { in a = 9; }
		}
	`
	outputs := nestedUsageOutputs(t, src, "c")
	wantInt(t, outputs, "d", 2)
}

// Nesting deeper than the recursion limit is what that limit refuses; an
// evaluation whose answer is an output binding is one level of it, not two.
func TestNestedCalcUsageDepthCountedOnce(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test {\n\tcalc def C0 { out r = 1; }\n")
	for i := 1; i <= nestingProbeDepth-8; i++ {
		fmt.Fprintf(&b, "\tcalc def C%d { out r = C%d() + 1; }\n", i, i-1)
	}
	fmt.Fprintf(&b, "\tcalc def Top { calc deep : C%d; out d = deep.r; }\n", nestingProbeDepth-8)
	b.WriteString("\tcalc top : Top;\n}")

	outputs := nestedUsageOutputsAtDepth(t, b.String(), "top", nestingProbeDepth)
	wantInt(t, outputs, "d", int64(nestingProbeDepth-7))
}

// A chain of outputs of one calc naming each other is evaluated within that one
// evaluation, so its length is not bounded by the nesting limit.
func TestCalcOutputChainIsNotNesting(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test {\n\tcalc def Chain {\n\t\tout o0 = 1;\n")
	for i := 1; i <= nestingProbeDepth+8; i++ {
		fmt.Fprintf(&b, "\t\tout o%d = o%d + 1;\n", i, i-1)
	}
	fmt.Fprintf(&b, "\t}\n\tcalc ch : Chain;\n}")

	// Read the last output alone, so the whole chain is evaluated on demand
	// rather than one output at a time.
	last := fmt.Sprintf("o%d", nestingProbeDepth+8)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, b.String()))
	ctx.maxCalcDepth = nestingProbeDepth
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "ch", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc usage ch not found")
	}
	value, err := ctx.CalcUsageOutput(sym, last, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("reading %s: %v", last, err)
	}
	if want := int64(nestingProbeDepth + 9); value.Kind != ValConst || value.Const.Int != want {
		t.Errorf("%s = %s, want %d", last, FormatTraceValue(value), want)
	}
}

// A nested usage read through an object keeps that object's state in reach: an
// input may name a feature value of the part the enclosing usage is a feature of.
func TestNestedCalcUsageReadsEnclosingObject(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in n : Integer;
				out a = n * 2;
				out b = n * 3;
			}
			calc def Outer {
				calc inner : Twice { in n = mass; }
				out a = inner.a;
				out b = inner.b;
			}
			part def Chassis {
				attribute mass : Integer = 7;
				calc c : Outer;
			}
			part chassis : Chassis;
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	part := findSymbolByName(rootScope, "chassis", ast.DefPart)
	if part == nil {
		t.Fatal("part chassis not found")
	}
	instance, err := ctx.Instantiate(part)
	if err != nil {
		t.Fatalf("instantiating chassis: %v", err)
	}
	matches := idx.LookupQualified("test::Chassis::c")
	if len(matches) != 1 {
		t.Fatalf("test::Chassis::c: %d matching symbols, want 1", len(matches))
	}
	usage := matches[0]
	outputs, err := ctx.CalcUsageOutputs(usage, usage.OwnerScope, instance)
	if err != nil {
		t.Fatalf("reading the outputs of c: %v", err)
	}
	wantInt(t, outputs, "a", 14)
	wantInt(t, outputs, "b", 21)
}
