package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

func TestW7GTwoTypesOnAOneTypeUsageIsAnError(t *testing.T) {
	const src = `package T {
		case def C1;
		case def C2;
		use case def U1;
		use case def U2;
		case cTwo : C1, C2;
		use case uTwo : U1, U2;
	}`
	diags := only(typeDiags(t, src), "one-type")
	if len(diags) != 2 {
		t.Fatalf("expected one diagnostic per over-typed usage, got %v", diags)
	}
	want := map[string]bool{
		"A case must be typed by one case definition.":         true,
		"A use case must be typed by one use case definition.": true,
	}
	for _, d := range diags {
		if !want[d.Message] {
			t.Fatalf("unexpected message %q", d.Message)
		}
		if d.Severity != SeverityError {
			t.Fatalf("the reference reports an error, got %v", d.Severity)
		}
	}
}

// Every usage kind the reference holds to a single definition, each with the
// message that kind's rule words.
func TestW7GEveryOneTypeUsageKindIsReported(t *testing.T) {
	cases := []struct{ keyword, def, want string }{
		{"calc", "calc def", "A calculation must be typed by one calculation definition."},
		{"constraint", "constraint def", "A constraint must be typed by one constraint definition."},
		{"requirement", "requirement def", "A requirement must be typed by one requirement definition."},
		{"case", "case def", "A case must be typed by one case definition."},
		{"analysis case", "analysis case def", "An analysis case must be typed by one analysis case definition."},
		{"verification case", "verification case def", "A verification case must be typed by one verification case definition."},
		{"use case", "use case def", "A use case must be typed by one use case definition."},
		{"enum", "enum def", "An enumeration must be typed by one enumeration definition."},
		{"rendering", "rendering def", "A rendering must be typed by one rendering definition."},
		{"viewpoint", "viewpoint def", "A viewpoint must be typed by one viewpoint definition."},
		{"view", "view def", "A view must be typed by one view definition."},
		{"metadata", "metadata def", "A metadata usage must be typed by one metadata definition."},
	}
	for _, c := range cases {
		t.Run(c.keyword, func(t *testing.T) {
			src := "package T {\n" +
				c.def + " D1;\n" + c.def + " D2;\n" +
				c.keyword + " u : D1, D2;\n}"
			diags := only(typeDiags(t, src), "one-type")
			if len(diags) != 1 || diags[0].Message != c.want {
				t.Fatalf("want %q, got %v", c.want, diags)
			}
		})
	}
}

func TestW7GOneOrNoDeclaredTypeIsSilent(t *testing.T) {
	const src = `package T {
		case def C1;
		case cNone;
		case cOne : C1;
	}`
	if diags := only(typeDiags(t, src), "one-type"); len(diags) != 0 {
		t.Fatalf("a usage with at most one declared type is silent, got %v", diags)
	}
}

func TestW7GTwoTypesOnAPartIsNotAOneTypeError(t *testing.T) {
	const src = `package T {
		part def P1;
		part def P2;
		part p : P1, P2;
	}`
	if diags := only(typeDiags(t, src), "one-type"); len(diags) != 0 {
		t.Fatalf("a part may be typed by several definitions, got %v", diags)
	}
}

func TestW7GAnAttributeTypedByAnEnumerationTakesNoOtherType(t *testing.T) {
	const src = `package T {
		enum def E { enum a; }
		attribute def A;
		attribute withEnum : E, A;
		attribute plain : A, A;
		ref refEnum : E, A;
		bareEnum : E, A;
	}`
	diags := only(typeDiags(t, src), "one-type")
	if len(diags) != 1 || diags[0].Message != msgEnumerationAttributeTypes {
		t.Fatalf("expected only the enumeration attribute to be reported, got %v", diags)
	}
}

// TestW7GAnEnumeratedValueIsTypedByItsEnumerationAlone: a declared type or an
// implicitly typing value outside the enumeration's generals makes two types.
func TestW7GAnEnumeratedValueIsTypedByItsEnumerationAlone(t *testing.T) {
	const src = `package T {
		enum def Level { low; high; }
		enum def Wrong {
			a : ScalarValues::Real;
			b = 1;
			c = 2.5;
			d = "s";
			e = true;
			f = Level::low;
			g : Level = Level::high;
			h = wrongOrLevel;
		}
		ref wrongOrLevel : Wrong, Level;
		ref rightOrReal : Right, ScalarValues::Real;
		enum def Right :> ScalarValues::Real {
			ok1 = 4.0;
			ok2 : ScalarValues::Real;
			ok3 : Right;
			ok4 :> ok1 = 1.0;
			ok5 default = 3.0;
			ok6 := 2.0;
			ok7;
			= 5.0;
			ok8 = rightOrReal;
		}
		enum def Nested {
			enum n1 : Nested;
			n2 = Nested::n1;
		}
	}`
	var got []string
	for _, d := range only(libraryTypeDiags(t, src), "one-type") {
		if d.Message != oneTypeUsageMessages[ast.UsageEnumeration] {
			t.Fatalf("unexpected message %q", d.Message)
		}
		got = append(got, strings.Fields(src[d.Span.Offset:d.Span.End()])[0])
	}
	want := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("enumerated values typed outside their enumeration: got %v, want %v", got, want)
	}
}

// TestW7GAComputedEnumeratedValueIsTypedByItsFunctionResult: an operator
// expression types the value as the result of the library function it names —
// Boolean for comparisons and logic, DataValue for arithmetic — a constructor as
// its definition, `#` as an element of its sequence, an expression body as an
// Evaluation (pilot 2026-07 agrees).
func TestW7GAComputedEnumeratedValueIsTypedByItsFunctionResult(t *testing.T) {
	const src = `package T {
		private import ScalarValues::*;
		attribute def A;
		attribute a0 : A;
		attribute r0 : Real;
		attribute xs : Integer[0..*];
		enum def Wrong {
			a = true and false;
			b = 1 == 2;
			c = a0 == a0;
			d = new A();
			e = xs#(1);
			f = 1 < 2;
			g = 1 istype Integer;
			h = { 1 + 2 };
			i = { true };
		}
		enum def Right {
			ok1 = 1 + 2;
			ok2 = -r0;
			ok3 = "x" + "y";
			ok4 = (1, 2);
			ok5 = if true ? 1 else 2;
			ok6 = r0 + 1;
			ok7 = not true;
			ok8 = 1 ?? 2;
			ok9 = 3 % 2;
		}
		enum def Flag :> Boolean {
			ok10 = 1 == 2;
			ok11 = true and false;
		}
	}`
	var got []string
	for _, d := range only(libraryTypeDiags(t, src), "one-type") {
		if d.Message != oneTypeUsageMessages[ast.UsageEnumeration] {
			t.Fatalf("unexpected message %q", d.Message)
		}
		got = append(got, strings.Fields(src[d.Span.Offset:d.Span.End()])[0])
	}
	want := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("computed enumerated values typed outside their enumeration: got %v, want %v", got, want)
	}
}

// TestW7GACastAndBodyEnumeratedValuesAreTyped: `x as T` types the value as T
// (through an alias too) and an expression body `{ … }` as an Evaluation, so
// neither escapes the check as Anything (pilot 2026-07 agrees line for line).
func TestW7GACastAndBodyEnumeratedValuesAreTyped(t *testing.T) {
	const src = `package T {
		private import ScalarValues::*;
		attribute def A;
		attribute a0 : A;
		attribute r0 : Real;
		alias Rl for Real;
		enum def Wrong {
			a = {in r : Real; r > 1.0};
			b = r0 as Real;
			c = a0 as A;
			d = r0 as Integer;
			e = r0 as Rl;
		}
		enum def Num :> Real {
			ok1 = r0 as Real;
			ok2 = r0 as Rl;
			f = r0 as Integer;
			g = {in r : Real; r > 1.0};
		}
	}`
	var got []string
	for _, d := range only(libraryTypeDiags(t, src), "one-type") {
		if d.Message != oneTypeUsageMessages[ast.UsageEnumeration] {
			t.Fatalf("unexpected message %q", d.Message)
		}
		got = append(got, strings.Fields(src[d.Span.Offset:d.Span.End()])[0])
	}
	want := []string{"a", "b", "c", "d", "e", "f", "g"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("cast/body enumerated values typed outside their enumeration: got %v, want %v", got, want)
	}
}

// TestW7GASelectedEnumeratedValueKeepsItsOperandType: `xs.?{…}` keeps the
// elements of xs, so it types the value as xs is typed — through a chain, an
// alias, a feature typed only by its own value or subsetting/redefining one, a
// nested selection or an indexed element — and `xs.{…}` is typed as its body's
// result is, an untyped parameter taking the elements' type, Anything where the
// collection itself is untyped, while `,` yields Anything (pilot 2026-07 agrees
// on every case but the collect bodies, which it leaves Anything).
func TestW7GASelectedEnumeratedValueKeepsItsOperandType(t *testing.T) {
	const src = `package T {
		private import ScalarValues::*;
		part def Other;
		part def W { attribute q : Real[0..*]; attribute qq = q; }
		part def W2 :> W { attribute :>> qq; }
		attribute xs : Real[0..*];
		attribute ns : Natural[0..*];
		alias ys for xs;
		attribute zs = xs;
		attribute zz :> zs;
		attribute z1 = 1.5;
		attribute anys;
		part others : Other[0..*];
		part w : W;
		part w2 : W2;
		enum def Wrong {
			a = xs.?{in r : Real; r > 1.0};
			b = others.?{in o : Other; true};
			c = ns.?{in n : Natural; n > 1};
			d = xs.?{in r : Real; r > 1.0}.?{in r : Real; r > 2.0};
			e = w.q.?{in r : Real; r > 1.0};
			f = ys.?{in r : Real; r > 1.0};
			g = zs.?{in r : Real; r > 1.0};
			h = zs;
			i = w.qq.?{in r : Real; r > 1.0};
			j = z1.?{in r : Real; r > 1.0};
			k = zs#(1);
			l = zz.?{in r : Real; r > 1.0};
			m = zz;
			n = w2.qq.?{in r : Real; r > 1.0};
			q = xs.{in r : Real; r};
			r = others.{in o : Other; o};
			t = xs.{in r; r};
			u = others.?{in o; true};
		}
		enum def WrongNum :> Real {
			o = ns.?{in n : Natural; n > 1};
			p = others.?{in o : Other; true};
			s = ns.{in n : Natural; n};
		}
		enum def Right {
			ok1 = anys.{in r; r};
			ok2 = Right::ok1.?{in l : Right; true};
		}
		enum def RightNum :> Real {
			ok3 = xs.?{in r : Real; r > 1.0};
			ok4 = xs.?{in r : Real; r > 1.0}.?{in r : Real; r > 2.0};
			ok5 = (xs, xs).?{in r : Real; r > 1.0};
			ok6 = xs#(1).?{in r : Real; r > 1.0};
			ok7 = w.q.?{in r : Real; r > 1.0};
			ok8 = ys.?{in r : Real; r > 1.0};
			ok9 = zs.?{in r : Real; r > 1.0};
			ok10 = zs;
			ok11 = w.qq.?{in r : Real; r > 1.0};
			ok12 = z1.?{in r : Real; r > 1.0};
			ok13 = zs#(1);
			ok14 = zz.?{in r : Real; r > 1.0};
			ok15 = zz;
			ok16 = w2.qq.?{in r : Real; r > 1.0};
			ok17 = xs.{in r : Real; r};
		}
	}`
	var got []string
	for _, d := range only(libraryTypeDiags(t, src), "one-type") {
		if d.Message != oneTypeUsageMessages[ast.UsageEnumeration] {
			t.Fatalf("unexpected message %q", d.Message)
		}
		got = append(got, strings.Fields(src[d.Span.Offset:d.Span.End()])[0])
	}
	want := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "q", "r", "t", "u", "o", "p", "s"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("selected enumerated values typed outside their enumeration: got %v, want %v", got, want)
	}
}
