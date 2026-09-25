package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// constraintDiags parses src, indexes it, runs the full default registry, and
// returns only diagnostics whose Source is "constraint".
func constraintDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	all := Analyze("<t>", root, nil, idx)
	var out []diag.Diagnostic
	for _, d := range all {
		if d.Source == "constraint" {
			out = append(out, d)
		}
	}
	return out
}

func hasCode(diags []diag.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestConstraintDirectSpecializationCycle(t *testing.T) {
	diags := constraintDiags(t, "part def A specializes A;")
	if !hasCode(diags, "specialization-cycle") {
		t.Fatalf("expected specialization-cycle diagnostic, got %v", diags)
	}
}

func TestConstraintTransitiveSpecializationCycle(t *testing.T) {
	diags := constraintDiags(t, "part def A specializes B; part def B specializes A;")
	// Both A and B are in the cycle; expect a diagnostic for each.
	n := 0
	for _, d := range diags {
		if d.Code == "specialization-cycle" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected 2 specialization-cycle diagnostics, got %d: %v", n, diags)
	}
}

// A classifier specializing a nested member that specializes it back, directly
// or through siblings, is a cycle (Xpect CircleInheritance, CircleProblem5).
func TestConstraintNestedMemberSpecializationCycle(t *testing.T) {
	for _, src := range []string{
		`package Test1 {
			classifier <'A_Id'> A specializes A::B {
				classifier <'B_Id'> B specializes A {}
			}
		}`,
		`package Test1 {
			classifier A specializes D {
				classifier B specializes C {}
			}
			classifier C specializes A {}
			classifier D specializes A::B {}
		}`,
	} {
		if !hasCode(constraintDiagsKerML(t, src), "specialization-cycle") {
			t.Fatalf("expected a specialization cycle for %q", src)
		}
	}
}

func TestConstraintNoCycleAcyclicOK(t *testing.T) {
	diags := constraintDiags(t, "part def Vehicle; part def Car specializes Vehicle;")
	if hasCode(diags, "specialization-cycle") {
		t.Fatalf("unexpected specialization-cycle diagnostic, got %v", diags)
	}
}

func TestConstraintMultiplicityRangeInverted(t *testing.T) {
	diags := constraintDiags(t, "part def C { part b [5..2]; }")
	if !hasCode(diags, "multiplicity-range") {
		t.Fatalf("expected multiplicity-range diagnostic, got %v", diags)
	}
}

func TestConstraintMultiplicityRangeValidOK(t *testing.T) {
	diags := constraintDiags(t, "part def C { part a [2..5]; part c [1..*]; }")
	if hasCode(diags, "multiplicity-range") {
		t.Fatalf("unexpected multiplicity-range diagnostic, got %v", diags)
	}
}

func TestEnumNamedValuesStillReportDuplicateNames(t *testing.T) {
	const src = `enum def Colors {
		enum red;
		enum red;
		enum x = 60.0;
		enum x = 80.0;
	}`
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	var duplicates []diag.Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Message == "Duplicate of other owned member name" {
			duplicates = append(duplicates, d)
		}
	}
	if len(duplicates) != 4 {
		t.Fatalf("got %d duplicate-name diagnostics, want 4: %v", len(duplicates), duplicates)
	}
}

func TestConstraintSubsettingMultiplicityExceeds(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..3]; part many subsets cap [0..10]; }")
	if !hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("expected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

func TestConstraintSubsettingMultiplicityConformsOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..10]; part few subsets cap [0..3]; }")
	if hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("unexpected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

func TestConstraintSubsettingUnboundedSupersetOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..*]; part many subsets cap [0..100]; }")
	if hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("unexpected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

func TestConstraintSubsettingInfiniteSubsetOfFiniteExceeds(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..5]; part many subsets cap [0..*]; }")
	if !hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("expected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

// --- V-C3 §4.3/§4.6: connector and flow ends ---

func TestConstraintConnectionBinaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part a; part b; connection conn connect a to b; }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("unexpected connector-ends diagnostic, got %v", diags)
	}
}

func TestConstraintConnectionNaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part a; part b; part d; connection conn connect (a, b, d); }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("n-ary connection should be allowed, got %v", diags)
	}
}

// The constraint tier sees every end an n-ary clause declares, so its arity
// rules count the real arity rather than a truncated one.
func TestConstraintConnectionNaryEndCountReachesTheChecker(t *testing.T) {
	src := "part def C { part a; part b; part c; part d; connection conn connect (a, b, c, d); }"
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	syms := idx.LookupQualified("C::conn")
	if len(syms) != 1 {
		t.Fatalf("expected one symbol for C::conn, got %d", len(syms))
	}
	u, ok := syms[0].Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("C::conn declared by %T, want *ast.Usage", syms[0].Decl)
	}
	if len(u.ConnectorEnds) != 4 {
		t.Fatalf("connector ends at the constraint tier = %d, want 4", len(u.ConnectorEnds))
	}
	if hasCode(constraintDiags(t, src), "connector-ends") {
		t.Fatalf("a four-end connection should be allowed")
	}
}

// The anonymous inline form reaches the arity checker with its real end count.
func TestConstraintAnonymousNaryConnectionEndCountReachesTheChecker(t *testing.T) {
	src := "part def C { part a; part b; part c; connect (a, b, c); }"
	if hasCode(constraintDiags(t, src), "connector-ends") {
		t.Fatalf("a three-end anonymous connection should be allowed")
	}
	if !hasCode(constraintDiags(t, "part def C { part a; connect (a); }"), "connector-ends") {
		t.Fatalf("a one-end anonymous connection should be reported")
	}
}

func TestConstraintConnectionSingleEndFails(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part a; connection conn connect (a); }")
	if !hasCode(diags, "connector-ends") {
		t.Fatalf("expected connector-ends diagnostic for single-end connection, got %v", diags)
	}
}

func TestConstraintInterfaceBinaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { port p; port q; interface i connect p to q; }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("unexpected connector-ends diagnostic, got %v", diags)
	}
}

func TestConstraintUntypedInterfaceNaryIsClean(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { port p; port q; port r; interface i connect (p, q, r); }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("untyped interfaces are n-ary, got %v", diags)
	}
}

func TestConstraintBinaryInterfaceNaryFails(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { port p; port q; port r; interface i : Interfaces::BinaryInterface connect (a ::> p, b ::> q, c ::> r); }")
	if !hasCode(diags, "connector-ends") {
		t.Fatalf("expected connector-ends diagnostic for n-ary binary interface, got %v", diags)
	}
}

func TestConstraintNamedNaryInterfaceIsAccepted(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { port p; port q; port r; interface i connect (a ::> p, b ::> q, c ::> r); }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("unexpected connector-ends diagnostic for named n-ary interface, got %v", diags)
	}
}

func TestConstraintAllocationBinaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part f; part g; allocation al allocate f to g; }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("unexpected connector-ends diagnostic, got %v", diags)
	}
}

func TestConstraintAllocationNaryFails(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part f; part g; part h; allocation al allocate (f, g, h); }")
	if !hasCode(diags, "connector-ends") {
		t.Fatalf("expected connector-ends diagnostic for n-ary allocation, got %v", diags)
	}
}

func TestConstraintFlowCompleteOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { item Fuel; part a { out item outFuel : Fuel; } part b { in item inFuel : Fuel; } flow f of Fuel from a.outFuel to b.inFuel; }")
	if hasCode(diags, "flow-ends") {
		t.Fatalf("unexpected flow-ends diagnostic, got %v", diags)
	}
}

// A payload-only flow declares no ends. SysML v2 §8.2.2.16 makes the
// `'from' … 'to' …` part of a FlowDeclaration optional, and §8.4.12.2 requires a
// message to have no owned flowEnds, so this is well formed.
func TestConstraintFlowPayloadOnlyHasNoEndsOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { item Fuel; flow f of Fuel; }")
	if hasCode(diags, "flow-ends") {
		t.Fatalf("unexpected flow-ends diagnostic for payload-only flow, got %v", diags)
	}
}

func TestConstraintMessageWithoutEndsOK(t *testing.T) {
	diags := constraintDiags(t,
		"item def SetSpeed; occurrence def I { message setSpeedMessage of SetSpeed; }")
	if hasCode(diags, "flow-ends") {
		t.Fatalf("unexpected flow-ends diagnostic for a message without ends, got %v", diags)
	}
}

// A flow naming a source but no target is a parse error, pinned by the parser's
// `flow_source_without_target` negative case, not by this tier.

// --- V-C4 Track 4 Task 13: typing conformance ---

// Subsetting does not require the subsetting feature's declared type to conform
// to the subsetted feature's type. Per KerML 8.3.3.3.4, the types of a Feature
// "are derived from its typings and the types of its subsettings", so the
// co-domain KerML 8.3.3.3.10 requires to specialize the subsetted co-domain is
// the intersection of both — it always does, whether or not the declared types
// are related (KerML 7.3.4.4: a subsetting feature can "add additional feature
// types").
func TestConstraintSubsettingConformingTypeOK(t *testing.T) {
	src := `
		attribute def Vehicle;
		attribute def Car specializes Vehicle;
		
		attribute vehicles : Vehicle[*];
		attribute myCar : Car subsets vehicles;
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "typing-conformance") {
		t.Fatalf("expected no typing-conformance diagnostic for valid conformance, got %v", diags)
	}
}

func TestConstraintSubsettingUnrelatedTypeOK(t *testing.T) {
	src := `
		attribute def Vehicle;
		attribute def Animal;
		
		attribute vehicles : Vehicle[*];
		attribute myPet : Animal subsets vehicles;
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "typing-conformance") {
		t.Fatalf("unexpected typing-conformance diagnostic: subsetting intersects types, got %v", diags)
	}
}

// The occurrence shape from the OMG `Model Library Example`: `Cause` does not
// specialize `Situation`, yet `causes :> situations` is well formed.
func TestConstraintSubsettingOccurrenceUnrelatedTypeOK(t *testing.T) {
	src := `
		abstract occurrence def Situation;
		abstract occurrence situations : Situation[*] nonunique;
		abstract occurrence def Cause;
		abstract occurrence causes : Cause[*] nonunique :> situations;
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "typing-conformance") {
		t.Fatalf("unexpected typing-conformance diagnostic on the model library shape, got %v", diags)
	}
}

// --- V-C4 Track 4 Task 14: redefinition validation ---

func TestConstraint_RedefinitionValid(t *testing.T) {
	src := `
		attribute def SpeedType;
		attribute def Vehicle {
			attribute speed : SpeedType;
		}
		attribute def Car specializes Vehicle {
			attribute speed : SpeedType :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "redefinition-no-inherited") || hasCode(diags, "redefinition-type-mismatch") {
		t.Fatalf("expected no redefinition diagnostic for valid redefinition, got %v", diags)
	}
}

func TestConstraint_RedefinitionUsesFeaturingType(t *testing.T) {
	src := `
		package P {
			class Base { feature x; }
			class T specializes Base;
			feature x :>> Base::x featured by T;
		}
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("featured-by type inherits x, got %v", diags)
	}
}

func TestConstraint_PackageLevelRedefinitionHasNoInheritedOwner(t *testing.T) {
	src := `
		package P {
			feature x :>> x;
		}
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("package-level feature has no inherited owner, got %v", diags)
	}
}

func TestConstraint_PackageLevelUnfeaturedRedefinitionExemptsNoInheritedRule(t *testing.T) {
	// A package-level feature has no featuring type, so no type can inherit it.
	diags := constraintDiags(t, `
		package P {
			feature x;
			feature y :>> x;
		}
	`)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("unfeatured package-level target should not trigger the rule: %v", diags)
	}
}

func TestConstraint_RedefinitionNoInheritedMember(t *testing.T) {
	src := `
		attribute def SpeedType;
		attribute def Vehicle {
			attribute speed : SpeedType;
		}
		attribute def Car {
			attribute speed : SpeedType :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("expected redefinition-no-inherited diagnostic, got %v", diags)
	}
}

// A member inherited through a chain of specializations is inherited, so
// redefining it is valid however far up it was declared.
func TestConstraint_RedefinitionInheritedTransitively(t *testing.T) {
	src := `
		attribute def SpeedType;
		attribute def Vehicle {
			attribute speed : SpeedType;
		}
		attribute def Car specializes Vehicle;
		attribute def RaceCar specializes Car {
			attribute speed : SpeedType :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("a member inherited through Car is inherited, got %v", diags)
	}
}

// A member of a nested definition redefines what the definition it is nested in
// inherits, not what the outer definition does.
func TestConstraint_RedefinitionInNestedDefinition(t *testing.T) {
	src := `
		attribute def SpeedType;
		part def Base {
			attribute speed : SpeedType;
		}
		part def Outer {
			part def Inner specializes Base {
				attribute speed : SpeedType :>> Base::speed;
			}
		}
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("Inner inherits speed from Base, got %v", diags)
	}
}

func TestConstraint_RedefinitionTypeMismatch(t *testing.T) {
	src := `
		attribute def SpeedType;
		attribute def NameType;
		attribute def Vehicle {
			attribute speed : SpeedType;
		}
		attribute def Car specializes Vehicle {
			attribute speed : NameType :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "redefinition-type-mismatch") {
		t.Fatalf("expected redefinition-type-mismatch diagnostic, got %v", diags)
	}
	// KerML defines no such constraint and the pinned pilot validator is silent:
	// the redefining type joins the redefined one's, so the report is advisory.
	for _, d := range diags {
		if d.Code == "redefinition-type-mismatch" && d.Severity != diag.SeverityWarning {
			t.Errorf("redefinition-type-mismatch is %v, want a warning", d.Severity)
		}
	}
}

// A redefinition narrowing to a subtype conforms; one keeping the redefined
// feature's type and adding none is silent as well.
func TestConstraint_RedefinitionConformingTypeStaysSilent(t *testing.T) {
	src := `
		part def A; part def A2 :> A;
		part def X { part p : A; }
		part def Y :> X { part :>> p : A2; }
		part def Z :> X { part :>> p; }
	`
	if diags := constraintDiags(t, src); hasCode(diags, "redefinition-type-mismatch") {
		t.Fatalf("expected no redefinition-type-mismatch, got %v", diags)
	}
}

// ShapeItems redefines `faces : StructuredSurface` as `Polygon` and
// `PlanarSurface`, which the pinned pilot validator accepts: analyzed against
// the library the file carries no error, and only advisory type reports.
func TestConstraint_ShapeItemsRedefinitionsAreNotErrors(t *testing.T) {
	const name = "Domain Libraries/Geometry/ShapeItems.sysml"
	src, err := libs.DefaultSource().Read(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	mismatches := 0
	for _, d := range w9cLibraryDiags(t, string(src), false) {
		if d.Severity == diag.SeverityError {
			t.Errorf("error on ShapeItems: %s", d.Message)
		}
		if d.Code == "redefinition-type-mismatch" {
			mismatches++
		}
	}
	if mismatches != 2 {
		t.Errorf("redefinition-type-mismatch on ShapeItems = %d, want the two faces redefinitions", mismatches)
	}
}

func TestConstraint_RedefinitionMultiplicityInvalid(t *testing.T) {
	src := `
		attribute def SpeedType;
		part def Vehicle {
			attribute speed : SpeedType[1..2];
		}
		part def Car specializes Vehicle {
			attribute speed : SpeedType[0..5] :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "redefinition-multiplicity") {
		t.Fatalf("expected redefinition-multiplicity diagnostic, got %v", diags)
	}
}

// `[*]` is `0..*` in a redefinition: it keeps an inherited `0..*` but loosens an
// inherited `1..*`, dropping its lower bound to 0.
func TestConstraint_RedefinitionUnboundedMultiplicity(t *testing.T) {
	tests := []struct {
		name      string
		inherited string
		wantDiag  bool
	}{
		{"redefines optional collection", "0..*", false},
		{"redefines required collection", "1..*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `
				attribute def SpeedType;
				part def Vehicle {
					attribute speed : SpeedType[` + tt.inherited + `];
				}
				part def Car specializes Vehicle {
					attribute speed : SpeedType[*] :>> Vehicle::speed;
				}
			`
			diags := constraintDiags(t, src)
			if got := hasCode(diags, "redefinition-multiplicity"); got != tt.wantDiag {
				t.Fatalf("redefinition-multiplicity = %v, want %v (diags %v)", got, tt.wantDiag, diags)
			}
		})
	}
}

// --- V-C4 Track 4 Integration: typing conformance + redefinition ---

func TestConstraint_Track4Integration(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
		codes   []string
	}{
		{
			name: "valid redefinition with multiplicity narrowing",
			src: `
				attribute def SpeedType;
				attribute def Vehicle {
					attribute speed : SpeedType[1..2];
				}
				attribute def Car specializes Vehicle {
					attribute speed : SpeedType[1..1] :>> Vehicle::speed;
				}
				attribute myCar : Car;
			`,
			wantErr: false,
		},
		{
			name: "redefinition without inheritance",
			src: `
				attribute def SpeedType;
				attribute def Animal {
					attribute speed : SpeedType;
				}
				attribute def Vehicle {
					attribute speed : SpeedType;
				}
				attribute def Car specializes Vehicle {
					attribute speed : SpeedType redefines Animal::speed;
				}
				attribute myCar : Car;
			`,
			wantErr: true,
			codes:   []string{"redefinition-no-inherited"},
		},
		{
			name: "redefinition multiplicity violation",
			src: `
				attribute def SpeedType;
				attribute def Vehicle {
					attribute speed : SpeedType[1..2];
				}
				attribute def Car specializes Vehicle {
					attribute speed : SpeedType[0..5] :>> Vehicle::speed;
				}
				attribute myCar : Car;
			`,
			wantErr: true,
			codes:   []string{"redefinition-multiplicity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := constraintDiags(t, tt.src)
			hasError := len(diags) > 0
			if hasError != tt.wantErr {
				t.Fatalf("wantErr=%v, got diagnostics: %v", tt.wantErr, diags)
			}
			if tt.wantErr {
				for _, code := range tt.codes {
					if !hasCode(diags, code) {
						t.Errorf("expected code %q, got: %v", code, diags)
					}
				}
			}
		})
	}
}

// A member redefining two features derives no name, so its value reaches
// neither; the diagnostic is a warning because the declaration is well-formed.
func TestConstraintUnnamedRedefinitionValue(t *testing.T) {
	src := `
		attribute def N;
		part def B { attribute x : N; attribute y : N; }
		part def A :> B {
			attribute <sn> redefines x, y = 9;
		}
	`
	diags := constraintDiags(t, src)
	var got *diag.Diagnostic
	for i, d := range diags {
		if d.Code == "redefinition-no-derived-name" {
			got = &diags[i]
		}
	}
	if got == nil {
		t.Fatalf("expected redefinition-no-derived-name diagnostic, got %v", diags)
	}
	if got.Severity != diag.SeverityWarning {
		t.Errorf("severity = %v, want warning", got.Severity)
	}
	want := "a member redefining x and y derives no name, so this value is bound to the short name <sn> only; declare a name or redefine one feature"
	if got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
}

// The symbol spelling reports identically, and a member with no short name takes
// the first target's name, so the value is unreachable by the others'.
func TestConstraintUnnamedRedefinitionValueSpellings(t *testing.T) {
	cases := []struct {
		name   string
		member string
		want   string
	}{
		{"keyword_short_name", "attribute <sn> redefines x, y = 9;", "bound to the short name <sn> only"},
		{"symbol_short_name", "attribute <sn> :>> x :>> y = 9;", "bound to the short name <sn> only"},
		{"keyword_anonymous", "attribute redefines x, y = 9;", "is not reachable by name"},
		{"symbol_anonymous", "attribute :>> x :>> y = 9;", "is not reachable by name"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			src := "attribute def N;\npart def B { attribute x : N; attribute y : N; }\n" +
				"part def A :> B {\n" + tt.member + "\n}"
			diags := constraintDiags(t, src)
			if !hasCode(diags, "redefinition-no-derived-name") {
				t.Fatalf("expected redefinition-no-derived-name, got %v", diags)
			}
			for _, d := range diags {
				if d.Code == "redefinition-no-derived-name" && !strings.Contains(d.Message, tt.want) {
					t.Errorf("message %q does not contain %q", d.Message, tt.want)
				}
			}
		})
	}
}

// A single redefinition derives its name from the target, so its value binds
// and nothing is reported.
func TestConstraintSingleRedefinitionValueOK(t *testing.T) {
	src := `
		attribute def N;
		part def B { attribute x : N; }
		part def A :> B {
			attribute <sn> redefines x = 5;
			attribute named redefines x = 6;
		}
	`
	if diags := constraintDiags(t, src); hasCode(diags, "redefinition-no-derived-name") {
		t.Fatalf("unexpected redefinition-no-derived-name, got %v", diags)
	}
}

// Two redefinitions without a value derive no name either, but nothing is lost.
func TestConstraintUnnamedRedefinitionNoValueOK(t *testing.T) {
	src := `
		attribute def N;
		part def B { attribute x : N; attribute y : N; }
		part def A :> B { attribute <sn> redefines x, y; }
	`
	if diags := constraintDiags(t, src); hasCode(diags, "redefinition-no-derived-name") {
		t.Fatalf("unexpected redefinition-no-derived-name, got %v", diags)
	}
}

// TestConstraintInterfaceEndConjugation covers SysML v2 §7.12.2: the ports at
// the two ends of an interface must have conjugate directed features, which one
// conjugated end (~P) supplies and two like-typed ends do not.
func TestConstraintInterfaceEndConjugation(t *testing.T) {
	const ports = `port def P { in item cmd; out item tlm; }
`
	conjugated := constraintDiags(t, ports+`interface def I {
		end a : P;
		end b : ~P;
	}`)
	if hasCode(conjugated, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for conjugate ends: %v", conjugated)
	}

	mismatched := constraintDiags(t, ports+`interface def I {
		end a : P;
		end b : P;
	}`)
	if !hasCode(mismatched, "port-conjugation") {
		t.Errorf("expected port-conjugation diagnostic for like-typed ends, got %v", mismatched)
	}

	// A port with no directed features imposes nothing.
	undirected := constraintDiags(t, `port def U { attribute x; }
	interface def I {
		end a : U;
		end b : U;
	}`)
	if hasCode(undirected, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for undirected ports: %v", undirected)
	}

	// Conjugation constrains directed features only, so ports holding different
	// undirected features still line up.
	extra := constraintDiags(t, `port def A { attribute pressure; out item flow; }
	port def B { in item flow; }
	interface def I {
		end a : A;
		end b : B;
	}`)
	if hasCode(extra, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for conjugate directed features: %v", extra)
	}
}

// An interface's own flow pairs differently named directed features, so the
// same-name fallback does not apply.
func TestConstraintInterfaceFlowPairsDirectedFeatures(t *testing.T) {
	complementary := constraintDiags(t, `port def Source { out item sent; }
	port def Target { in item received; }
	interface def Link {
		end a : Source;
		end b : Target;
		flow a.sent to b.received;
	}`)
	if hasCode(complementary, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for paired flow features: %v", complementary)
	}

	reversed := constraintDiags(t, `port def Source { out item sent; }
	port def Target { in item received; }
	interface def Link {
		end a : Target;
		end b : Source;
		flow b.sent to a.received;
	}`)
	if hasCode(reversed, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for reversed paired flow features: %v", reversed)
	}

	nonComplementary := constraintDiags(t, `port def Source { out item sent; }
	port def Target { out item received; }
	interface def Link {
		end a : Source;
		end b : Target;
		flow a.sent to b.received;
	}`)
	if !hasCode(nonComplementary, "port-conjugation") {
		t.Errorf("expected port-conjugation diagnostic for non-complementary flow features, got %v", nonComplementary)
	}

	unpaired := constraintDiags(t, `port def Source { out item sent; }
	port def Target { in item received; }
	interface def Link {
		end a : Source;
		end b : Target;
		flow a.sent to b;
	}`)
	if !hasCode(unpaired, "port-conjugation") {
		t.Errorf("expected port-conjugation diagnostic for unpaired flow feature, got %v", unpaired)
	}
}

// A `variant` whose owner is not a variation offers no choice, so it is an error
// as it is in the reference.
func TestConstraintVariantOutsideVariation(t *testing.T) {
	src := `
		part def Widget {
			variant attribute misplaced = 1.0;
		}
	`
	diags := constraintDiags(t, src)
	var got *diag.Diagnostic
	for i, d := range diags {
		if d.Code == "variant-outside-variation" {
			got = &diags[i]
		}
	}
	if got == nil {
		t.Fatalf("expected variant-outside-variation diagnostic, got %v", diags)
	}
	if got.Severity != diag.SeverityError {
		t.Errorf("severity = %v, want error", got.Severity)
	}
	if got.Message != msgVariantOutsideVariation {
		t.Errorf("message = %q, want %q", got.Message, msgVariantOutsideVariation)
	}
}

// A variant of a variation is exactly what `variant` is for, so nothing is
// reported for it.
func TestConstraintVariantInsideVariationOK(t *testing.T) {
	src := `
		part def Widget {
			variation attribute pick {
				variant attribute cheap = 1.0;
				variant attribute rich = 2.0;
			}
		}
	`
	if diags := constraintDiags(t, src); hasCode(diags, "variant-outside-variation") {
		t.Fatalf("unexpected variant-outside-variation, got %v", diags)
	}
}

// A usage typed by a variation definition, and one redefining a variation usage,
// are variation points without restating the modifier.
func TestConstraintVariantUnderInheritedVariationOK(t *testing.T) {
	src := `
		part def Engine;
		variation part def EngineChoice :> Engine;
		part def Car {
			part engine : EngineChoice {
				variant part electric : Engine;
			}
		}
		abstract part refined : Car {
			part :>> engine {
				variant part petrol : Engine;
			}
		}
	`
	if diags := constraintDiags(t, src); hasCode(diags, "variant-outside-variation") {
		t.Fatalf("unexpected variant-outside-variation, got %v", diags)
	}
}

// A view body's satisfy must name a viewpoint: only a viewpoint frames concerns,
// so a satisfy naming a plain requirement claims a conformance nothing checks.
func TestConstraintViewSatisfyNonViewpointRequirement(t *testing.T) {
	src := `
		part def Vehicle;
		part vehicle : Vehicle;
		requirement spec { subject s : Vehicle; }
		view v { expose vehicle; satisfy spec; }
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "view-satisfy-viewpoint") {
		t.Fatalf("expected view-satisfy-viewpoint diagnostic, got %v", diags)
	}
}

func TestConstraintViewSatisfyViewpointOK(t *testing.T) {
	src := `
		part def Vehicle;
		part vehicle : Vehicle;
		viewpoint def VP;
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; }
	`
	if diags := constraintDiags(t, src); hasCode(diags, "view-satisfy-viewpoint") {
		t.Fatalf("unexpected view-satisfy-viewpoint, got %v", diags)
	}
}

// A view body may satisfy a requirement of a stated subject — the stdlib's
// `View` does, as `satisfy requirement viewpointConformance by that` — which
// asserts that requirement rather than conformance to a viewpoint.
func TestConstraintViewSatisfyRequirementBySubjectOK(t *testing.T) {
	src := `
		part def Vehicle;
		part vehicle : Vehicle;
		requirement def VehicleSpecification { subject s : Vehicle; }
		requirement spec : VehicleSpecification;
		view v {
			expose vehicle;
			satisfy requirement conformance : VehicleSpecification by vehicle;
			satisfy spec by vehicle;
		}
	`
	if diags := constraintDiags(t, src); hasCode(diags, "view-satisfy-viewpoint") {
		t.Fatalf("unexpected view-satisfy-viewpoint, got %v", diags)
	}
}

// A satisfy outside a view body is an ordinary requirement satisfaction, which
// this rule says nothing about.
func TestConstraintSatisfyOutsideAViewIsNotChecked(t *testing.T) {
	src := `
		part def Vehicle;
		part vehicle : Vehicle;
		requirement spec { subject s : Vehicle; }
		part holder { satisfy spec by vehicle; }
	`
	if diags := constraintDiags(t, src); hasCode(diags, "view-satisfy-viewpoint") {
		t.Fatalf("unexpected view-satisfy-viewpoint, got %v", diags)
	}
}
