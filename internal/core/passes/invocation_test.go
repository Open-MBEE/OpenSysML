package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// libraryDiags analyzes src against the bundled standard library and returns
// the name-resolution and type diagnostics, which is what a call reports.
func libraryDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	return libraryDiagsOf(parser.New(source.New("<t>", []byte(src))).ParseFile())
}

func libraryDiagsOf(root *ast.RootNamespace) []Diagnostic {
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()
	var out []Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == "type" || d.Source == "name-resolution" {
			out = append(out, d)
		}
	}
	return out
}

func wantLibraryClean(t *testing.T, src string) {
	t.Helper()
	if diags := libraryDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func wantLibraryDiag(t *testing.T, src, code, want string) {
	t.Helper()
	wantLibrarySeverity(t, src, SeverityError, code, want)
}

// wantLibraryWarning is wantLibraryDiag for an advisory.
func wantLibraryWarning(t *testing.T, src, code, want string) {
	t.Helper()
	wantLibrarySeverity(t, src, SeverityWarning, code, want)
}

func wantLibrarySeverity(t *testing.T, src string, severity Severity, code, want string) {
	t.Helper()
	diags := libraryDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one diagnostic, got %v", diags)
	}
	if diags[0].Severity != severity || diags[0].Code != code {
		t.Fatalf("expected %s %q, got %s %q (%s)", severity, code, diags[0].Severity, diags[0].Code, diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, want) {
		t.Fatalf("expected message containing %q, got %q", want, diags[0].Message)
	}
}

const numericImports = `
	private import ScalarValues::*;
	private import IntegerFunctions::*;
	private import RealFunctions::*;
	private import RationalFunctions::*;
	private import ComplexFunctions::*;
`

// A local name several imported function packages declare binds to the declaration
// whose parameter types the arguments conform to, so every acceptance probe checks clean.
func TestInvocationOverloadSelectsByArgumentType(t *testing.T) {
	wantLibraryClean(t, `package P {`+numericImports+`
		attribute s = ToInteger("7");
		attribute r = ToInteger(7.9);
		attribute i = abs(-2);
		attribute q = abs(-2.5);
		attribute c = abs(rect(3.0, 4.0));
		attribute z = isZero(rect(0.0, 0.0));
		attribute m = max(3, 2.5);
	}`)
}

// Only imported packages contribute candidates: without StringFunctions a String fits
// no visible ToString, and the diagnostic names the candidates considered.
func TestInvocationOverloadOnlyImportedPackagesContribute(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		private import IntegerFunctions::*;
		private import RealFunctions::*;
		attribute s = ToString("x");
	}`, "type.expr", "argument 1 of ToString expects Integer, found String (candidates: IntegerFunctions::ToString, RealFunctions::ToString)")
}

// Among the applicable candidates, the most specific parameter types win: the
// result of the call is typed by that candidate, which a binding then checks.
func TestInvocationOverloadResultTypeFollowsSelection(t *testing.T) {
	wantLibraryClean(t, `package P {`+numericImports+`
		attribute i : Natural = abs(-2);
		attribute b : Boolean = isZero(rect(0.0, 0.0));
	}`)
	wantLibraryDiag(t, `package P {`+numericImports+`
		attribute s : String = abs(-2);
	}`, "type.expr", "cannot bind Natural value to a feature typed by String")
}

// Strings and booleans take part: ToString has a String overload beside the
// numeric ones, and a Boolean argument selects the Boolean one.
func TestInvocationOverloadStringAndBoolean(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import StringFunctions::*;
		private import BooleanFunctions::*;
		private import IntegerFunctions::*;
		attribute a = ToString("x");
		attribute b = ToString(true);
		attribute c = ToString(7);
	}`)
}

// A collection argument binds to a collection parameter, which the sequence
// functions declare.
func TestInvocationOverloadCollections(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		attribute xs : Integer[0..*] = (1, 2, 3);
		attribute n = size(xs);
		attribute h = head(xs);
	}`)
}

// Quantity arguments select by declared type: a mass binds to the candidate
// taking a mass, not to the one taking a length, though both are Real-valued.
func TestInvocationOverloadQuantities(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import ISQ::*;
		private import SI::*;
		package A { calc def weigh { in x : MassValue; return : Boolean = true; } }
		package B { calc def weigh { in x : LengthValue; return : String = "long"; } }
		package C {
			private import A::*;
			private import B::*;
			attribute mass : MassValue = 2 [kg];
			attribute l : LengthValue = 3 [m];
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `w : Boolean = weigh(mass);`))
	wantLibraryClean(t, fmt.Sprintf(model, `w : String = weigh(l);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `w : String = weigh(mass);`),
		"type.expr", "cannot bind Boolean value to a feature typed by String")
}

// Declared types neither of which conforms to the other never bind (a mass fits no volume
// overload), a broader quantity binds either way, and a Collection parameter takes any sequence.
func TestInvocationOverloadDisjointDeclaredTypes(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import ISQ::*;
		private import SI::*;
		package A { calc def weigh { in x : VolumeValue; return : Boolean = true; } }
		package B { calc def weigh { in x : LengthValue; return : String = "long"; } }
		package D { calc def single { in x : VolumeValue; return : Boolean = true; } }
		package C {
			private import A::*;
			private import B::*;
			private import D::*;
			attribute mass : MassValue = 2 [kg];
			attribute q : Quantities::ScalarQuantityValue = 3 [m];
			attribute %s
		}
	}`
	wantLibraryDiag(t, fmt.Sprintf(model, `w = weigh(mass);`), "type.expr",
		"argument 1 of weigh expects VolumeValue, found MassValue (candidates: P::A::weigh, P::B::weigh)")
	wantLibraryDiag(t, fmt.Sprintf(model, `w = weigh(x = mass);`), "type.expr",
		"argument x of weigh expects VolumeValue, found MassValue (candidates: P::A::weigh, P::B::weigh)")
	wantLibraryDiag(t, fmt.Sprintf(model, `w = single(mass);`), "type.expr",
		"argument 1 of single expects VolumeValue, found MassValue")
	wantLibraryClean(t, fmt.Sprintf(model, `w = weigh(q);`))
	wantLibraryClean(t, fmt.Sprintf(model, `w = single(q);`))
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import CollectionFunctions::*;
		calc def Cnt { in x : Integer[0..*]; return : Boolean = contains(x, 2); }
		calc def Has { in c : Collections::Collection; return : Boolean = true; }
		calc def Use { in x : Integer[0..*]; return : Boolean = Has(x); }
	}`)
}

// An Element parameter takes the element any argument names, and sits between a
// declared type and Anything in specificity; the result type tells which overload won.
func TestInvocationOverloadElementParameter(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import KerML::Root::Element;
		private import Base::Anything;
		part def Telescope;
		part def GroundStation :> Telescope;
		part def Antenna;
		part telescope : Telescope;
		part station : GroundStation;
		part antenna : Antenna;
		package A { calc def pick { in e : Element; return : Integer = 1; } }
		package B { calc def pick { in t : Telescope; return : String = "t"; } }
		package C { calc def pick { in a : Anything; return : Boolean = true; } }
		package E { calc def single { in e : Element; return : Integer = 5; } }
		package Use {
			private import A::*;
			private import B::*;
			private import C::*;
			private import E::*;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `v : Integer = single(telescope);`))
	wantLibraryClean(t, fmt.Sprintf(model, `v : Integer = single(e = station);`))
	wantLibraryClean(t, fmt.Sprintf(model, `v : String = pick(station);`))
	wantLibraryClean(t, fmt.Sprintf(model, `v : Integer = pick(antenna);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `v : Boolean = pick(antenna);`), "type.expr",
		"Boolean")
}

// Two candidates the arguments fit equally, neither more specific than the
// other, are an ambiguity the checker reports by name rather than resolving.
func TestInvocationOverloadAmbiguous(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; in y : Real; return : Integer = 1; } }
		package B { calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute tied = pick(1, 2);
		}
	}`, "invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
}

// An ambiguity names only the candidates none is more specific than: a broader
// overload the arguments also fit is beaten, so it is not among the tied ones.
func TestInvocationOverloadAmbiguityNamesOnlyTheTiedBest(t *testing.T) {
	diags := libraryDiags(t, `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		package A { calc def pick { in x : P::Mass; return : Integer = 1; } }
		package B { calc def pick { in x : P::Mass; return : Integer = 2; } }
		package C { calc def pick { in x : Real; return : Integer = 3; } }
		package D {
			private import A::*;
			private import B::*;
			private import C::*;
			attribute m : Mass;
			attribute tied = pick(m);
		}
	}`)
	want := "call of pick is ambiguous between P::A::pick, P::B::pick"
	if len(diags) != 1 || diags[0].Code != "invocation-ambiguous" || diags[0].Message != want {
		t.Fatalf("expected one invocation-ambiguous diagnostic %q, got %v", want, diags)
	}
}

// With no applicable candidate the first is checked as before, and the report
// names every candidate considered.
func TestInvocationOverloadNoneApplicable(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; in y : Real; return : Integer = 1; } }
		package B { calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute none = pick("a", 2);
		}
	}`, "type.expr", `argument 1 of pick expects Integer, found String (candidates: P::A::pick, P::B::pick)`)
}

// A non-callable declaration found before the callable one does not take the
// report: the call is checked against the callable and its argument mismatch shown.
func TestInvocationOverloadNoneApplicableReportsAgainstTheCallable(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		package A { attribute def pick; }
		package B { calc def pick { in x : Integer; return : Integer = x; } }
		package C {
			private import A::*;
			private import B::*;
			attribute none = pick("a");
		}
	}`, "type.expr", `argument 1 of pick expects Integer, found String (candidates: P::A::pick, P::B::pick)`)
}

// A candidate the arguments fit but leave a default-less parameter of is applicable:
// the call binds to it and advises of the omission rather than reporting a mismatch
// against a candidate the arguments do not fit.
func TestInvocationOverloadOmittedInputSelectsTheFittingCandidate(t *testing.T) {
	wantLibraryWarning(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package B { calc def pick { in s : String; in y : Real; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute partial = pick("a");
		}
	}`, CodeUnboundParameter, "pick leaves parameter y unbound, so the call cannot be evaluated")
}

// A candidate the arguments bind completely beats one they leave a parameter of,
// whether they conform to its parameter or may only fit it at run time.
func TestInvocationOverloadCompleteCandidateBeatsOmission(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Real; in y : Real; return : Real = x + y; } }
		package B { calc def pick { in x : Integer; return : Integer = x; } }
		package C {
			private import A::*;
			private import B::*;
			attribute r : Real = 2.5;
			attribute exact = pick(2);
			attribute loose = pick(r);
		}
	}`)
}

// Two candidates the arguments fit equally, each left with a parameter unbound, tie:
// nothing written tells them apart, and a call writing no argument at all least of all.
func TestInvocationOverloadOmittedInputsAreAmbiguous(t *testing.T) {
	for _, call := range []string{"pick(1.0)", "pick()"} {
		wantLibraryDiag(t, `package P {
			private import ScalarValues::*;
			package A { calc def pick { in x : Real; in y : Real; return : Real = x + y; } }
			package B { calc def pick { in x : Real; in z : String; return : Real = x; } }
			package C {
				private import A::*;
				private import B::*;
				attribute tied = `+call+`;
			}
		}`, "invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
	}
}

// A feature typed by a calc performs that calc, so it is a candidate beside a same-named
// calc: each call is checked against the one its argument fits, and only a call neither
// fits is reported, naming both.
func TestInvocationOverloadFeatureTypedByABehavior(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : String; return : Integer = 1; } }
		package B { calc def Twice { in x : Integer; return : Integer = 2 * x; } ref pick : Twice; }
		package C {
			private import A::*;
			private import B::*;
			attribute i : Integer = pick(3);
			attribute s : Integer = pick("s");
			%s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(src, ""))
	wantLibraryDiag(t, fmt.Sprintf(src, `attribute b : Integer = pick(true);`),
		"type.expr", "argument 1 of pick expects String, found Boolean (candidates: P::A::pick, P::B::pick)")
}

// A bodiless calc def specializing a library function is called through its own
// effective inputs — renamed, defaulted, re-multiplied or added by position — as is
// a feature typed by it; the runtime binds the same way (see runtime tests).
func TestInvocationOverloadRedeclaredLibraryInputs(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		private import RealFunctions::sqrt;
		private import RealFunctions::max;
		private import SequenceFunctions::isEmpty;
		calc def Renamed :> sqrt { in y :>> x; }
		calc def Defaulted :> sqrt { in x :>> x = 16.0; }
		calc def ByPosition :> sqrt { in q : Real; }
		calc def Floor :> max { in floor :>> y = 0.0; }
		calc def Empty :> isEmpty { in items :>> seq; }
		calc def Required :> isEmpty { in seq :>> seq [1]; }
		ref renamed : Renamed;
		ref defaulted : Defaulted;
		attribute a : Real = Renamed(y = 16.0);
		attribute b : Real = Renamed(16.0);
		attribute c : Real = Defaulted();
		attribute d : Real = Defaulted(x = 25.0);
		attribute e : Real = ByPosition(4.0);
		attribute f : Real = Floor(x = -3.0);
		attribute g : Real = Floor(x = 5.0, floor = 7.0);
		attribute h : Boolean = Empty();
		attribute i : Boolean = Empty(items = 3);
		attribute j : Real = renamed(y = 16.0);
		attribute k : Real = defaulted();
		%s
	}`
	wantLibraryClean(t, fmt.Sprintf(src, ""))
	wantLibraryDiag(t, fmt.Sprintf(src, `attribute z : Real = Renamed(x = 16.0);`),
		"type.expr", `Renamed has no parameter named "x"`)
	wantLibraryWarning(t, fmt.Sprintf(src, `attribute z : Real = Renamed();`),
		CodeUnboundParameter, "Renamed leaves parameter y unbound")
	wantLibraryWarning(t, fmt.Sprintf(src, `attribute z : Real = Floor(2.0);`),
		CodeUnboundParameter, "Floor leaves parameter x unbound")
	wantLibraryWarning(t, fmt.Sprintf(src, `attribute z : Boolean = Required();`),
		CodeUnboundParameter, "Required leaves parameter seq unbound")
}

// An argument of unknown type keeps every candidate applicable: the call selects only when
// one remains or specificity settles it, else an advisory names the ones left open.
func TestInvocationOverloadUnknownArgumentType(t *testing.T) {
	wantLibraryWarning(t, `package P {`+numericImports+`
		attribute untyped;
		attribute a = abs(untyped);
	}`, "invocation-ambiguous",
		"call of abs is undetermined between IntegerFunctions::abs, RealFunctions::abs, RationalFunctions::abs, ComplexFunctions::abs")
	wantLibraryWarning(t, `package P {`+numericImports+`
		attribute untyped;
		attribute m = max(untyped, 2);
	}`, "invocation-ambiguous",
		"call of max is undetermined between IntegerFunctions::max, RealFunctions::max, RationalFunctions::max")

	const model = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; in n : Integer; return : Integer = 1; } }
		package B { calc def pick { in x : Real; in n : Integer; return : Integer = 2; } }
		package F { calc def pick { in x : Integer; in flag : Boolean; return : Integer = 3; } }
		package D { calc def scale { in x; in by : Integer; return : Integer = 1; } }
		package E { calc def scale { in x; in by : Real; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			private import F::*;
			private import D::*;
			private import E::*;
			attribute untyped;
			attribute %s
		}
	}`
	// A known argument fitting one candidate alone selects it.
	wantLibraryClean(t, fmt.Sprintf(model, `one = pick(untyped, true);`))
	// Both fit the known argument; the unknown one settles nothing, so they tie.
	wantLibraryWarning(t, fmt.Sprintf(model, `two = pick(untyped, 3);`), "invocation-ambiguous",
		"call of pick is undetermined between P::A::pick, P::B::pick")
	// Candidates typing the unknown argument alike are ordered by the known one.
	wantLibraryClean(t, fmt.Sprintf(model, `s : Integer = scale(untyped, 3);`))
	// Under an unknown argument the result type is unknown: no binding is judged.
	wantLibraryWarning(t, fmt.Sprintf(model, `r : String = pick(untyped, 3);`), "invocation-ambiguous",
		"call of pick is undetermined")
}

// A parameter declared without a type is typed Anything, the least specific type: a
// candidate typing that parameter wins over it, and a parameter whose type did not
// resolve is undetermined, so lookup order keeps deciding.
func TestInvocationOverloadUntypedParameterIsLeastSpecific(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Real; in y%s; return : String = "loose"; } }
		package B { calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, ``, `typed : Integer = pick(1, 2);`))
	wantLibraryDiag(t, fmt.Sprintf(model, ``, `wrong : String = pick(1, 2);`),
		"type.expr", "cannot bind Integer value to a feature typed by String")
	wantLibraryClean(t, fmt.Sprintf(model, ``, `loose : String = pick(1, "b");`))
	diags := libraryDiags(t, fmt.Sprintf(model, ` : Missing`, `first : String = pick(1, 2);`))
	if len(diags) != 1 || diags[0].Code != "unresolved" || !strings.Contains(diags[0].Message, "Missing") {
		t.Fatalf("expected only the unresolved parameter type, got %v", diags)
	}
}

// A parameter typed `Anything` in writing is the same least specific parameter as one
// declaring no type: the two spellings tie, and either loses to a typed parameter.
func TestInvocationOverloadExplicitAnythingEqualsUntyped(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import Base::Anything;
		package A { calc def pick { in x : Real; in y : Anything; return : String = "a"; } }
		package B { calc def pick { in x : Real; in y%s; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute w : Real = 1.0;
			attribute v : Integer = 2;
			attribute %s
		}
	}`
	wantLibraryDiag(t, fmt.Sprintf(model, ``, `r = pick(w, v);`),
		"invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
	wantLibraryDiag(t, fmt.Sprintf(model, ``, `r = pick(1, 2);`),
		"invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
	wantLibraryClean(t, fmt.Sprintf(model, ` : Integer`, `r : Integer = pick(w, v);`))
	wantLibraryDiag(t, fmt.Sprintf(model, ` : Integer`, `r : String = pick(w, v);`),
		"type.expr", "cannot bind Integer value to a feature typed by String")
}

// Candidates each narrower on a different parameter are incomparable, so the call is
// ambiguous however the least specific parameter is spelt; typing both wins outright.
func TestInvocationOverloadCrossedSpecificityIsAmbiguous(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import Base::Anything;
		attribute def Foo;
		package A { calc def pick { in x%s; in y : Real; return : String = "a"; } }
		package B { calc def pick { in x : Foo; in y%s; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute f : Foo;
			attribute w : Real = 1.0;
			attribute r%s = pick(f, w);
		}
	}`
	for _, spelling := range [][2]string{{``, ``}, {` : Anything`, ` : Anything`}, {` : Anything`, ``}, {``, ` : Anything`}} {
		wantLibraryDiag(t, fmt.Sprintf(model, spelling[0], spelling[1], ``),
			"invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
	}
	wantLibraryClean(t, fmt.Sprintf(model, ` : Foo`, ` : Anything`, ` : String`))
	wantLibraryDiag(t, fmt.Sprintf(model, ` : Foo`, ` : Anything`, ` : Integer`),
		"type.expr", "cannot bind String value to a feature typed by Integer")
}

// A called name resolves as any name does (KerML 1.1 §8.2.3.5.4, §7.2.5.4): an owned calc
// hides the library's import, a nested import stands ahead of the enclosing calc.
func TestInvocationOverloadModelShadowsLibrary(t *testing.T) {
	wantLibraryDiag(t, `package P {`+numericImports+`
		calc def abs { in x : String; return : String = x; }
		attribute a = abs(-2.5);
	}`, "type.expr", "argument 1 of abs expects String, found Rational")
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		calc def sqrt { in x : String; return : String = x; }
		attribute a = sqrt("x");
	}`)
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		calc def abs { in x : String; return : String = x; }
		package Inner {
			private import RealFunctions::*;
			attribute a : Real = abs(-2.5);
		}
	}`)
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		calc def abs { in x : String; return : String = x; }
		package Inner {
			private import RealFunctions::*;
			attribute a : Real = abs("x");
		}
	}`, "type.expr", "argument 1 of abs expects Real, found String")
}

// An action performed as a usage's value selects among actions only: a same-named
// calc that fits the argument better is not what the action usage runs, so the call
// is checked against the action it does run.
func TestInvocationPerformedActionSelectsAmongActions(t *testing.T) {
	const actions = `
		package A {
			private import ScalarValues::*;
			calc def tag { in x : Integer; return : Integer = x + 1; }
		}
		package B {
			private import ScalarValues::*;
			action def tag { in x : %s; out code : Integer; }
		}
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			action def Outer {
				first start;
				action call = tag(3);
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	wantLibraryClean(t, fmt.Sprintf(actions, "Real"))
	wantLibraryDiag(t, fmt.Sprintf(actions, "String"), "type.expr", "argument 1 of tag expects String, found Natural")
}

// An evaluated call prefers a calc it fits over a same-named action whose input fits
// the argument more closely: the expression's result is the calc's, so its type is
// checked against that calc, whichever import names it first. An action's inputs
// fitting where no calc's do does not make it the call: the arguments are reported
// against the calc, as the runtime would refuse to evaluate the action.
func TestInvocationEvaluatedCallPrefersACalc(t *testing.T) {
	const src = `
		package A {
			private import ScalarValues::*;
			action def pick { in x : Integer; out r : Integer; }
		}
		package B {
			private import ScalarValues::*;
			calc def pick { in x : %s; return : String = "s"; }
		}
		package test {
			private import ScalarValues::*;
			%s
			attribute v : Integer = 3;
			attribute %s
		}
	`
	for _, imports := range []string{
		"private import A::*; private import B::*;",
		"private import B::*; private import A::*;",
	} {
		wantLibraryClean(t, fmt.Sprintf(src, "Real", imports, `s : String = pick(v);`))
		wantLibraryDiag(t, fmt.Sprintf(src, "Real", imports, `i : Integer = pick(v);`), "type.expr", "String")
		wantLibraryDiag(t, fmt.Sprintf(src, "Boolean", imports, `i : Integer = pick(v);`),
			"type.expr", "argument 1 of pick expects Boolean, found Integer (candidates: ")
	}
}

// An action usage's value that names only calcs, one or several, performs no
// action: the checker refuses it as the runtime does, and a call that names no
// behavior at all keeps its own diagnostic.
func TestInvocationPerformedActionMustNameAnAction(t *testing.T) {
	const calcs = `
		package A {
			private import ScalarValues::*;
			calc def tag { in x : Integer; return : Integer = x + 1; }
		}
		package B {
			private import ScalarValues::*;
			calc def tag { in x : String; return : Integer = 2; }
		}
		package test {
			private import ScalarValues::*;
			private import A::*;
			%s
			part def Wheel;
			action def Outer {
				first start;
				action call = %s;
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	for _, imports := range []string{``, `private import B::*;`} {
		wantLibraryDiag(t, fmt.Sprintf(calcs, imports, `tag(3)`), "usage-reference-kind", "Must reference an action.")
	}
	wantLibraryDiag(t, fmt.Sprintf(calcs, ``, `Wheel()`), "invocation-not-behavior", "Must invoke a behavior or a behavioral feature")
}

// Function libraries are not implicitly imported: a bare call to any library function
// is unresolved until its package is imported, and the diagnostic offers those imports.
func TestInvocationUnimportedLibraryFunction(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute r = sqrt(4.0);
	}`, "unresolved", "unresolved reference: sqrt — did you mean RealFunctions::sqrt")
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute e = exp(1.0);
	}`, "unresolved", "unresolved reference: exp")
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute e = nosuchfunction(1.0);
	}`, "unresolved", "unresolved reference: nosuchfunction")
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import RealFunctions::*;
		attribute r = sqrt(4.0);
		attribute q = RealFunctions::sqrt(4.0);
	}`)
}

// A qualified library name resolves whatever the model imports, and its
// arguments are type-checked against the declaration it denotes.
func TestInvocationQualifiedLibraryFunctionArgumentsChecked(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute r = RealFunctions::sqrt("four");
	}`, "type.expr", "argument 1 of sqrt expects Real, found String")
}

// A named argument binds by the parameter's effective name, which is what
// tells two same-arity candidates apart.
func TestInvocationOverloadNamedArguments(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in width : Integer; return : Integer = 1; } }
		package B { calc def pick { in height : Integer; return : String = "h"; } }
		package C {
			private import A::*;
			private import B::*;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `w : Integer = pick(width = 2);`))
	wantLibraryClean(t, fmt.Sprintf(model, `h : String = pick(height = 2);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `d = pick(depth = 2);`),
		"type.expr", "candidates: P::A::pick, P::B::pick")
}

// A named argument written twice is refused, for one candidate and for several, as
// the evaluator refuses it; no candidate applies, so the report names them all.
func TestInvocationOverloadRepeatedNamedArgumentIsRefused(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package B { calc def pick { in x : String; return : String = "s"; } }
		package C {
			private import A::*;
			private import B::*;
			attribute %s
		}
		calc def one { in x : Integer; return : Integer = x; }
		attribute %s
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `s : String = pick(x = "s");`, `o : Integer = one(x = 1);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `s : String = pick(x = 1, x = "s");`, `o : Integer = one(x = 1);`),
		"type.expr", `pick binds parameter "x" twice (candidates: P::A::pick, P::B::pick)`)
	wantLibraryDiag(t, fmt.Sprintf(model, `s : String = pick(x = "s");`, `o : Integer = one(x = 1, x = 1);`),
		"type.expr", `one binds parameter "x" twice`)
}

// Two same-named calcs one wildcard import surfaces are both candidates.
func TestInvocationOverloadSiblingsThroughOneWildcardImport(t *testing.T) {
	const model = `package Lib {
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = x; }
		calc def pick { in x : String; return : String = x; }
	}
	package P {
		private import ScalarValues::*;
		private import Lib::*;
		attribute i : Integer = pick(2);
		attribute s : String = pick("s");
	}`
	for _, d := range libraryDiags(t, model) {
		if d.Severity != SeverityWarning || d.Code != "name-conflict" {
			t.Fatalf("expected only the owned name-conflict warnings, got %v", d)
		}
	}
}

// Parameters typed by sibling specializations of one scalar type are told apart
// by the argument's declared type; a bare scalar is not an ambiguity.
func TestInvocationOverloadSiblingScalarTypes(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		attribute def Volume :> Real;
		package A { calc def label { in m : Mass; return : String = "mass"; } }
		package B { calc def label { in v : Volume; return : Integer = 1; } }
		package C {
			private import A::*;
			private import B::*;
			attribute kg : Mass = 3.0;
			attribute litre : Volume = 2.0;
			attribute r : Real = 1.0;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `m : String = label(kg);`))
	wantLibraryClean(t, fmt.Sprintf(model, `v : Integer = label(litre);`))
	wantLibraryClean(t, fmt.Sprintf(model, `first : String = label(r);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `wrong : Integer = label(kg);`),
		"type.expr", "cannot bind String value to a feature typed by Integer")
}

// A named argument is checked against the parameter it names as a positional
// one is against its position, and a parameter without a default left unnamed is advised of.
func TestInvocationNamedArgumentsAreTypeChecked(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		attribute def Volume :> Real;
		calc def density { in m : Mass; in v : Volume; in scale : Real = 1.0; return : Real = m / v * scale; }
		attribute kg : Mass = 3.0;
		attribute litre : Volume = 2.0;
		attribute %s
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `ok : Real = density(m = kg, v = litre);`))
	wantLibraryClean(t, fmt.Sprintf(model, `scaled : Real = density(v = litre, m = kg, scale = 2);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `s : Real = density(m = "3", v = litre);`),
		"type.expr", `argument m of density expects Real, found String`)
	wantLibraryDiag(t, fmt.Sprintf(model, `b : Real = density(m = kg, v = litre, scale = true);`),
		"type.expr", `argument scale of density expects Real, found Boolean`)
	wantLibraryWarning(t, fmt.Sprintf(model, `partial : Real = density(m = kg);`),
		CodeUnboundParameter, `density leaves parameter v unbound`)
}

// Candidates come through re-exporting public imports and from general types; an
// inherited member hides a same-named import, as for any other name.
func TestInvocationOverloadReexportedAndInheritedCandidates(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package B { calc def pick { in x : String; return : String = x; } }
		package Both {
			public import A::*;
			public import B::*;
		}
		package C {
			private import Both::*;
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
	const inherited = `package P {
		private import ScalarValues::*;
		package B { calc def pick { in x : String; return : String = x; } }
		part def Base {
			calc def pick { in x : Integer; return : Integer = 1; }
		}
		part def Derived :> Base {
			private import B::*;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(inherited, `i : Integer = pick(2);`))
	wantLibraryDiag(t, fmt.Sprintf(inherited, `s = pick("s");`),
		"type.expr", "argument 1 of pick expects Integer, found String")
}

// A qualified call reaches every overload its qualifier surfaces through public
// imports or generals, not only the first; an owned overload hides them.
func TestInvocationOverloadQualifiedReexportedAndInheritedCandidates(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package B { calc def pick { in x : String; return : String = x; } }
		package Both {
			public import A::*;
			public import B::*;
		}
		attribute i : Integer = Both::pick(2);
		attribute s : String = Both::pick("s");
	}`)
	diags := libraryDiags(t, `package P {
		private import ScalarValues::*;
		part def ByNumber { calc def pick { in x : Integer; return : Integer = x; } }
		part def ByText { calc def pick { in x : String; return : String = x; } }
		part def Derived :> ByNumber, ByText;
		attribute i : Integer = Derived::pick(2);
		attribute s : String = Derived::pick("s");
	}`)
	if len(diags) != 1 || diags[0].Code != "name-conflict" || diags[0].Severity != SeverityWarning {
		t.Fatalf("expected only the inherited name-conflict warning, got %v", diags)
	}
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package Both {
			public import A::*;
			calc def pick { in x : String; return : String = x; }
		}
		attribute i : String = Both::pick(2);
	}`, "type.expr", "argument 1 of pick expects String, found Natural")
}

// A member the qualifier inherits hides the same name its own public import surfaces,
// so the imported overload is not a candidate even when only it fits the argument.
func TestInvocationOverloadQualifiedInheritedHidesImported(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		package Lib { calc def pick { in x : Integer; return : Integer = 1; } }
		part def Base { calc def pick { in x : String; return : Integer = 2; } }
		part def T :> Base { public import Lib::pick; }
		attribute %s
	}`
	wantLibraryClean(t, fmt.Sprintf(src, `s : Integer = T::pick("s");`))
	wantLibraryDiag(t, fmt.Sprintf(src, `i : Integer = T::pick(3);`),
		"type.expr", "argument 1 of pick expects String, found Natural")
}

// A general type's protected and public imports each contribute their overloads to
// the bodies specializing it, not only the first import's.
func TestInvocationOverloadCandidatesThroughInheritedImports(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = x; } }
		package B { calc def pick { in x : String; return : String = x; } }
		part def Base {
			protected import A::*;
			%s import B::*;
		}
		part def Derived :> Base {
			attribute i : Integer = pick(2);
			attribute s = pick("s");
		}
		part p : Base {
			attribute i : Integer = pick(2);
			attribute s = pick("s");
		}
	}`
	for _, visibility := range []string{"protected", "public"} {
		for _, d := range libraryDiags(t, fmt.Sprintf(src, visibility)) {
			if d.Code != "name-conflict" || d.Severity != SeverityWarning {
				t.Fatalf("%s import: expected only name-conflict warnings, got %v", visibility, d)
			}
		}
	}
	// A private import is not inherited, so only A's overload is visible.
	diags := libraryDiags(t, fmt.Sprintf(src, "private"))
	if len(diags) != 2 {
		t.Fatalf("expected one rejected call per body, got %v", diags)
	}
	for _, d := range diags {
		if d.Code != "type.expr" || d.Message != "argument 1 of pick expects Integer, found String" {
			t.Fatalf("unexpected diagnostic %v", d)
		}
	}
}

// A collection literal is typed by the type its elements share, so overloads
// differing by element type select; a mixed one is of unknown type, which keeps
// every candidate open.
func TestInvocationOverloadSelectsByCollectionLiteralElementType(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		package A { calc def count { in xs : String[*]; return : Integer = 1; } }
		package B { calc def count { in xs : Integer[*]; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute n : Integer = count(%s);
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(src, `("a", "b")`))
	wantLibraryClean(t, fmt.Sprintf(src, `(1, 2, 3)`))
	wantLibraryWarning(t, fmt.Sprintf(src, `(1, "b")`), "invocation-ambiguous",
		"call of count is undetermined between P::A::count, P::B::count")
	// An empty sequence has no element to type: every candidate takes it alike.
	wantLibraryClean(t, fmt.Sprintf(src, `()`))
	wantLibraryClean(t, fmt.Sprintf(src, `null`))
	wantLibraryDiag(t, fmt.Sprintf(src, `(true, false)`),
		"type.expr", "argument 1 of count expects String, found Boolean (candidates: P::A::count, P::B::count)")

	// Features typed by sibling types select by the declared type their elements share.
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		attribute def Volume :> Real;
		package A { calc def total { in ms : P::Mass[*]; return : Integer = 1; } }
		package B { calc def total { in vs : P::Volume[*]; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute m1 : Mass; attribute m2 : Mass; attribute v : Volume;
			attribute n : Integer = total((m1, m2));
			attribute k : Integer = total((v));
		}
	}`)
}

// Every general type, owned member and recursive-import descendant contributes its
// declaration; two sharing a name is a warning, not a lost candidate.
func TestInvocationOverloadCandidatesFromEveryGeneralAndRecursiveImport(t *testing.T) {
	diags := libraryDiags(t, `package P {
		private import ScalarValues::*;
		part def ByNumber { calc def pick { in x : Integer; return : Integer = x; } }
		part def ByText { calc def pick { in x : String; return : String = x; } }
		part def Both :> ByNumber, ByText {
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
	if len(diags) != 1 || diags[0].Code != "name-conflict" || diags[0].Severity != SeverityWarning {
		t.Fatalf("expected only the inherited name-conflict warning, got %v", diags)
	}
	diags = libraryDiags(t, `package P {
		private import ScalarValues::*;
		part def Base {
			calc def pick { in x : Integer; return : Integer = x; }
			calc def pick { in x : String; return : String = x; }
		}
		part def Derived :> Base {
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
	for _, d := range diags {
		if d.Code != "name-conflict" || d.Severity != SeverityWarning {
			t.Fatalf("expected only name-conflict warnings for Base's overloads, got %v", diags)
		}
	}
	diags = libraryDiags(t, `package P {
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = x; }
		calc def pick { in x : String; return : String = x; }
		attribute i : Integer = pick(2);
		attribute s : String = pick("s");
		attribute qi : Integer = P::pick(2);
		attribute qs : String = P::pick("s");
	}`)
	if len(diags) != 2 {
		t.Fatalf("expected only the two owned name-conflict warnings, got %v", diags)
	}
	for _, d := range diags {
		if d.Code != "name-conflict" || d.Severity != SeverityWarning {
			t.Fatalf("expected only owned name-conflict warnings, got %v", diags)
		}
	}
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		package Lib {
			package Numbers { calc def pick { in x : Integer; return : Integer = x; } }
			package Text { calc def pick { in x : String; return : String = x; } }
		}
		package C {
			private import Lib::**;
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
}

// A performed action's input is omitted when it may hold no value or a default reaches
// it along its redefinitions; an input with neither left unbound is advised of.
func TestInvocationPerformedActionOptionalInputs(t *testing.T) {
	const outer = `
		package test {
			private import ScalarValues::*;
			private import B::*;
			action def Outer {
				first start;
				action call = tag();
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	wantLibraryClean(t, `package B {
		private import ScalarValues::*;
		action def tag { in x : Integer[0..1]; out code : Integer; }
	}`+outer)
	wantLibraryClean(t, `package B {
		private import ScalarValues::*;
		action def base { in x : Integer = 3; out code : Integer; }
		action def tag :> base { in x : Integer :>> x; }
	}`+outer)
	wantLibraryWarning(t, `package B {
		private import ScalarValues::*;
		action def base { in x : Integer = 3; out code : Integer; }
		action def tag :> base { in x : Integer :>> x; in y : Integer; }
	}`+outer, CodeUnboundParameter, "tag leaves parameter y unbound")
}

// A requirement's subject is its first parameter, and a call may omit it when its
// declaration or a subject it redefines supplies a default or admits no value; a
// subject with neither stays required, so the omission leaves the call ambiguous.
func TestInvocationOverloadDefaultedSubjectIsOptional(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		part def Widget { attribute m : Real; }
		part light : Widget { attribute :>> m = 1.0; }
		package A {
			requirement def Light {
				%s
				in limit : Real;
				require constraint { limit > 0.0 }
			}
		}
		package B {
			requirement def Light {
				subject w : Widget;
				in limit : Real;
				in slack : Real;
				require constraint { limit + slack > 0.0 }
			}
		}
		package C {
			private import A::*;
			private import B::*;
			attribute ok : Boolean = Light(limit = 2.0);
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `subject w : Widget default = light;`))
	wantLibraryClean(t, fmt.Sprintf(model, `subject w : Widget[0..1];`))
	wantLibraryDiag(t, fmt.Sprintf(model, `subject w : Widget;`),
		"invocation-ambiguous", "call of Light is ambiguous between P::A::Light, P::B::Light")
}

// A usage redefining a subject without restating a value inherits its default, so a
// call of the usage may omit the subject as a call of its definition may.
func TestInvocationRedefinedSubjectInheritsDefault(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		part def Widget { attribute m : Real; }
		part light : Widget { attribute :>> m = 1.0; }
		requirement def Light {
			subject w : Widget default = light;
			in limit : Real default = 5.0;
			require constraint { w.m < limit }
		}
		requirement lightEnough : Light { subject :>> w; in :>> limit = 0.5; }
		attribute ok : Boolean = lightEnough();
	}`)
}
