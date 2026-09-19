package repl

import (
	"reflect"
	"strings"
	"testing"
)

// docQueryModel declares document queries over a small part tree: a projecting
// query, one relying on a default, one redefining inherited defaults, one
// composing another by name, and one traversing named relationships, plus one
// naming an unsupported kind.
const docQueryModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}

	part telescope {
		part optics : Subsystem {
			attribute redefines mass = 8.5;
		}
		part segmentControl : Subsystem {
			attribute redefines mass = 20.0;
		}
		part mount : Subsystem {
			attribute redefines mass = 15.0;
		}
	}

	calc def HeavySubsystems :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = WhereFeature(
					source = WhereType(
						source = Descendants(source = root, maxDepth = 3),
						type = "PartUsage"
					),
					'feature' = "mass",
					operator = ">=",
					value = "10"
				),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "mass")
		)
	}

	calc def PlainCalc {
		in x : Real;
		x
	}

	calc def NamedSubsystems :> Query {
		in root : Element;
		in pattern : String default "mo";
		WhereName(source = OwnedElements(source = root), operator = "startsWith", value = pattern)
	}

	calc def DefaultedSubsystems :> NamedSubsystems {
		in redefines root = telescope;
		in redefines pattern default "seg";
	}

	calc def ComposedQuery :> Query {
		in root : Element;
		HeavySubsystems(root = root)
	}

	calc def RelatedQuery :> Query {
		in root : Element;
		RelatedElements(
			source = root,
			relationshipKind = "typing",
			direction = "incoming",
			maxDepth = 1
		)
	}

	calc def UnknownRelatedQuery :> Query {
		in root : Element;
		RelatedElements(
			source = root,
			relationshipKind = "containment",
			direction = "outgoing",
			maxDepth = 1
		)
	}
}
`

func docQuerySession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(docQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	return s
}

func TestRunQueryProjectsOrderedRows(t *testing.T) {
	s := docQuerySession(t)
	got := run(t, s, "%run-query HeavySubsystems root=Observatory::telescope")
	wants(t, got,
		"✓ Query Observatory::HeavySubsystems returned 2 rows",
		"Columns: name, mass",
		"Row 1: Observatory::telescope::mount",
		`name = "mount"`,
		"mass = 15.0",
		"Row 2: Observatory::telescope::segmentControl",
		"mass = 20.0",
	)
	if strings.Index(got, "mount") > strings.Index(got, "segmentControl") {
		t.Errorf("rows are not in the query's order:\n%s", got)
	}
}

func TestRunDocumentQueryVerdict(t *testing.T) {
	s := docQuerySession(t)
	v := s.RunDocumentQuery("HeavySubsystems root=telescope")
	if !v.Holds() {
		t.Fatalf("verdict = %s: %v", v.Status, v.Lines)
	}
	values := map[string]string{}
	for _, nv := range v.Values {
		values[nv.Name] = nv.Value
	}
	if values["rows"] != "2" || values["columns"] != "name, mass" {
		t.Errorf("values = %v", v.Values)
	}
}

func TestRunQueryUsageAndUnknownName(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query"), "usage: %run-query <name> [<parameter>=<expression> ...]")
	wants(t, run(t, s, "%run-query NoSuchQuery"), "error:", "NoSuchQuery")
	wants(t, run(t, s, "%run-query HeavySubsystems telescope"),
		"error:", "<parameter>=<expression>")
}

func TestRunQueryRejectsNonQueryDefinition(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query PlainCalc x=1.0"),
		"error:", "not a document query", "DocumentQueries::Query")
}

func TestRunQuerySurfacesTypedExecutionFailures(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query HeavySubsystems"),
		"error:", "requires binding root")
	wants(t, run(t, s, "%run-query HeavySubsystems root=telescope depth=3"),
		"error:", "unknown binding depth")
	wants(t, run(t, s, "%run-query HeavySubsystems root=1"),
		"error:", "binding root has type integer, expected")
	// An unsupported relationship kind is a typed execution failure.
	wants(t, run(t, s, "%run-query UnknownRelatedQuery root=telescope"),
		"error:", `does not support relationship kind "containment"`)
}

func TestRunQueryTraversesNamedRelationships(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query RelatedQuery root=Subsystem"),
		"✓ Query Observatory::RelatedQuery returned 3 rows",
		"Row 1: Observatory::telescope::optics",
		"Row 2: Observatory::telescope::segmentControl",
		"Row 3: Observatory::telescope::mount",
	)
}

func TestRunQueryExecutesComposedQueries(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query ComposedQuery root=telescope"),
		"✓ Query Observatory::ComposedQuery returned 2 rows",
		"Columns: name, mass",
		"Row 1: Observatory::telescope::mount",
		"Row 2: Observatory::telescope::segmentControl")
}

func TestRunQueryUsesParameterDefaults(t *testing.T) {
	s := docQuerySession(t)
	// An omitted parameter takes its declared default; an explicit binding overrides it.
	wants(t, run(t, s, "%run-query NamedSubsystems root=telescope"),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: Observatory::telescope::mount")
	wants(t, run(t, s, `%run-query NamedSubsystems root=telescope pattern="op"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: Observatory::telescope::optics")
	// An element-naming default binds that element, so every parameter may be omitted.
	wants(t, run(t, s, "%run-query DefaultedSubsystems"),
		"✓ Query Observatory::DefaultedSubsystems returned 1 row",
		"Row 1: Observatory::telescope::segmentControl")
}

// The REPL looks names up in the document's own scope tree while the runtime
// model reads the index, so a parameter typed by a declaration of the session
// must accept that declaration's values whichever tree each came through.
func TestRunQueryConformsAcrossScopeTrees(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package Site {
	private import DocumentQueries::*;
	enum def Color { red; green; }
	part def Telescope;
	part def GroundStation :> Telescope;
	part telescope : Telescope { part optics; }
	part groundStation : GroundStation { part antenna; }
	calc def Painted :> Query {
		in hue : Color = Color::red;
		OwnedElements(source = telescope)
	}
	calc def Sited :> Query {
		in site : Telescope = groundStation;
		OwnedElements(source = telescope)
	}
}
`)
	if len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	wants(t, run(t, s, "%run-query Painted"),
		"✓ Query Site::Painted returned 1 row", "Row 1: Site::telescope::optics")
	wants(t, run(t, s, "%run-query Painted hue=Color::green"),
		"✓ Query Site::Painted returned 1 row")
	wants(t, run(t, s, "%run-query Painted hue=telescope"),
		"error:", "binding hue has type element, expected Site::Color")
	wants(t, run(t, s, "%run-query Sited"),
		"✓ Query Site::Sited returned 1 row")
	wants(t, run(t, s, "%run-query Sited site=groundStation"),
		"✓ Query Site::Sited returned 1 row")
	wants(t, run(t, s, "%run-query Sited site=telescope"),
		"✓ Query Site::Sited returned 1 row")
}

func TestRunQueryBindingExpressions(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, `%run-query NamedSubsystems root=telescope pattern="mo"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: Observatory::telescope::mount")
	wants(t, run(t, s, "%run-query HeavySubsystems root=noSuchName"),
		"error:", "binding root")
}

func TestRunQuerySpacedBindingExpressions(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, `%run-query NamedSubsystems root=telescope pattern="m" + "o"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: Observatory::telescope::mount")
	wants(t, run(t, s, `%run-query NamedSubsystems root=telescope pattern=("m" + "o")`),
		"✓ Query Observatory::NamedSubsystems returned 1 row")
	wants(t, run(t, s, `%run-query NamedSubsystems pattern="m" + "o" root=telescope`),
		"✓ Query Observatory::NamedSubsystems returned 1 row")
}

func TestRegroupBindings(t *testing.T) {
	cases := []struct {
		tokens []string
		want   []string
	}{
		{[]string{"limit=1", "+", "2"}, []string{"limit=1 + 2"}},
		{[]string{"a=1", "b=2"}, []string{"a=1", "b=2"}},
		{[]string{"a=(1", "+", "2)", "b=3"}, []string{"a=(1 + 2)", "b=3"}},
		{[]string{`s="a b"`, "+", `"c"`}, []string{`s="a b" + "c"`}},
		{[]string{"a=x", "==", "y"}, []string{"a=x == y"}},
		{[]string{"telescope"}, []string{"telescope"}},
	}
	for _, c := range cases {
		if got := regroupBindings(c.tokens); !reflect.DeepEqual(got, c.want) {
			t.Errorf("regroupBindings(%q) = %q, want %q", c.tokens, got, c.want)
		}
	}
}

func TestRunQueryListedInHelpAndCompletion(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%help"), "%run-query <name> [<p>=<expr>...]")
	comp := s.Complete("%run-que", len("%run-que"))
	found := false
	for _, cand := range comp.Candidates {
		if cand == "%run-query" {
			found = true
		}
	}
	if !found {
		t.Errorf("%%run-query is not completed: %v", comp.Candidates)
	}
}

// A measurement reference binds as the one unit declaration it names, so a query
// traverses from it; a unit composed of several names no element to bind.
func TestRunQueryBindsAMeasurementReference(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, `%run-query NamedSubsystems root=SI::km pattern="unit"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: SI::kilometre::unitConversion")
	wants(t, run(t, s, `%run-query NamedSubsystems root=SI::m / SI::s pattern="unit"`),
		"error:", "the measurement reference m/s names no single declaration to bind to a query parameter")
}

// A measurement scale binds as the declaration it is, so a query traverses from
// it; its placement binds as the transformation the library declares.
func TestRunQueryBindsACoordinateFrame(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, `%run-query NamedSubsystems root=SI::'°C_abs' pattern="unit"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: SI::'degree celsius (absolute temperature scale)'::unit")
	wants(t, run(t, s, `%run-query NamedSubsystems root=SI::'°C_abs'.transformation pattern="origin"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: SI::'degree celsius (absolute temperature scale)'::zeroDegreeCelsiusToKelvinShift::origin")
}

// objectQueryModel declares queries over a car whose wheels the session can
// instantiate: one over what an object owns, one over the objects a session
// holds, and one that only model elements answer.
const objectQueryModel = `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Integer = 30;
		action inflate {
			in delta : Integer;
			first set;
			action set {
				assign pressure := pressure + delta;
			}
		}
	}

	part def Car {
		part wheels : Wheel[2];
	}

	part car : Car;

	calc def Parts :> Query {
		in root : Element;
		Project(source = OwnedElements(source = root), properties = ("name", "pressure"))
	}

	calc def Wheels :> Query {
		Project(source = Objects(type = "Wheel"), properties = ("pressure"))
	}

	calc def Typing :> Query {
		in root : Element;
		RelatedElements(source = root, relationshipKind = "typing", direction = "outgoing", maxDepth = 1)
	}
}
`

func objectQuerySession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(objectQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	return s
}

// A held object wins over the element it was instantiated from, so the query
// runs over the object's parts under their paths; before instantiation the same
// binding is the element, whose usage owns nothing of its own.
func TestRunQueryBindsHeldObjectOverElement(t *testing.T) {
	s := objectQuerySession(t)
	wants(t, run(t, s, "%run-query Parts root=Garage::Car"),
		"✓ Query Garage::Parts returned 1 row",
		"Row 1: Garage::Car::wheels",
		"pressure = 30")
	wants(t, run(t, s, "%run-query Parts root=car"), "✓ Query Garage::Parts returned 0 rows")
	wants(t, run(t, s, "%instantiate Garage::car"), "Created instance")
	wants(t, run(t, s, "%run-query Parts root=car"),
		"✓ Query Garage::Parts returned 2 rows",
		"Row 1: Garage::car.wheels[1] (#2)",
		`name = "wheels[1]"`,
		"pressure = 30",
		"Row 2: Garage::car.wheels[2] (#3)")
	wants(t, run(t, s, "%run-query Parts root=Garage::car"), "returned 2 rows")
	wants(t, run(t, s, "%run-query Parts root=Garage::Car"), "returned 1 row", "Row 1: Garage::Car::wheels")
}

// `#id` and a path through a held object bind that object, and a query reads
// the value an object holds now, not the declared default.
func TestRunQueryBindsObjectByIDAndPath(t *testing.T) {
	s := objectQuerySession(t)
	wants(t, run(t, s, "%instantiate Garage::car"), "Created instance")
	wants(t, run(t, s, "%run-query Parts root=#1"), "returned 2 rows", "Row 1: #1.wheels[1] (#2)")
	wants(t, run(t, s, "%run-query Parts root=car.wheels[2]"), "returned 0 rows")
	wants(t, run(t, s, "%invoke car.wheels[2] inflate delta=5"), "Invoked inflate on object #3")
	got := run(t, s, "%run-query Wheels")
	wants(t, got,
		"✓ Query Garage::Wheels returned 2 rows",
		"Row 1: Garage::car.wheels[1] (#2)",
		"pressure = 30",
		"Row 2: Garage::car.wheels[2] (#3)",
		"pressure = 35")
	if strings.Index(got, "pressure = 30") > strings.Index(got, "pressure = 35") {
		t.Errorf("wheel rows are out of session order:\n%s", got)
	}
	wants(t, run(t, s, "%run-query Parts root=#99"), "error:", "binding root", "#99")
}

// A query over model elements only refuses an object row with the operation
// and the object it was given.
func TestRunQueryRefusesObjectRowsInElementOperations(t *testing.T) {
	s := objectQuerySession(t)
	wants(t, run(t, s, "%run-query Typing root=car"), "✓ Query Garage::Typing returned 1 row", "Garage::Car")
	wants(t, run(t, s, "%instantiate Garage::car"), "Created instance")
	wants(t, run(t, s, "%run-query Typing root=car"), "error:",
		"operation related-elements applies to model elements, not to object Garage::car")
}

// objectDocumentModel adds a document over the object query model: a table
// bound to the car and a list of every wheel the session holds.
const objectDocumentModel = objectQueryModel + `package Reports {
	private import DocumentQueries::*;
	private import Garage::*;

	part def CarReport :> Document {
		attribute redefines title = "Car Report";
		part parts : Table {
			calc rows : Parts {
				in root = car;
			}
		}
		part wheels : List {
			calc items : Wheels;
		}
	}
}
`

// A document renders over what the session holds: before instantiation its
// table shows the declared usage's parts (none of its own) and no objects; after
// it, the objects by path, with the values they hold now.
func TestRenderDocumentOverHeldObjects(t *testing.T) {
	s := NewSession()
	if res := s.Submit(objectDocumentModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	declared := run(t, s, "%render-document Reports::CarReport")
	wants(t, declared, "# Car Report", "| name | pressure |\n| --- | --- |")
	if strings.Contains(declared, "wheels\\[") || strings.Contains(declared, "- 30") {
		t.Errorf("a session holding nothing rendered objects:\n%s", declared)
	}

	wants(t, run(t, s, "%instantiate Garage::car"), "Created instance")
	wants(t, run(t, s, "%invoke car.wheels[2] inflate delta=5"), "Invoked inflate on object #3")
	wants(t, run(t, s, "%render-document Reports::CarReport"),
		"# Car Report",
		"| name | pressure |",
		`| wheels\[1\] | 30 |`,
		`| wheels\[2\] | 35 |`,
		"- 30\n- 35")
}

// verdictQueryModel declares a car whose own constraint holds and whose wheels'
// constraint fails once instantiated, with queries over the verdicts about it.
const verdictQueryModel = `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Integer = 30;
		assert constraint inflated { pressure >= 25 }
		action deflate {
			in delta : Integer;
			first set;
			action set {
				assign pressure := pressure - delta;
			}
		}
	}

	part def Car {
		attribute mass : Integer = 1500;
		part wheels : Wheel[2];
		assert constraint light { mass < 2000 }
	}

	part car : Car;

	requirement def LightCar {
		subject c : Car;
		require constraint { c.mass < 1800 }
	}
	requirement lightCar : LightCar;
	satisfy lightCar by car;

	calc def Checks :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("path", "verdict", "reason"))
	}

	calc def Failing :> Query {
		in root : Element;
		WhereFeature(source = Verdicts(source = root), 'feature' = "verdict", operator = "=", value = "violated")
	}
}
`

// Verdicts about a held object read it as it stands, one row per assertion on
// the object and the objects it holds; before instantiation the same binding is
// the element, checked as declared.
func TestRunQueryReportsVerdicts(t *testing.T) {
	s := NewSession()
	if res := s.Submit(verdictQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	wants(t, run(t, s, "%run-query Checks root=car"),
		"✓ Query Garage::Checks returned 4 rows",
		"Row 1: assert constraint light on Garage::car: holds",
		`path = "Garage::car"`,
		`verdict = "holds"`,
		"reason = (none)",
		"Row 2: satisfy lightCar by car on Garage::car: holds",
		"Row 3: assert constraint inflated on Garage::car.wheels[1]: holds")
	wants(t, run(t, s, "%run-query Failing root=car"), "✓ Query Garage::Failing returned 0 rows")
	wants(t, run(t, s, "%instantiate Garage::car"), "Created instance")
	wants(t, run(t, s, "%invoke car.wheels[2] deflate delta=10"), "Invoked deflate on object #3")
	wants(t, run(t, s, "%run-query Checks root=car"),
		"✓ Query Garage::Checks returned 4 rows",
		"Row 2: satisfy lightCar by car on Garage::car: holds",
		"Row 4: assert constraint inflated on Garage::car.wheels[2]: violated",
		`path = "Garage::car.wheels[2]"`,
		`verdict = "violated"`,
		`reason = "constraint inflated: assertion evaluated to false: pressure >= 25"`)
	wants(t, run(t, s, "%run-query Failing root=#1"),
		"✓ Query Garage::Failing returned 1 row",
		"Row 1: assert constraint inflated on #1.wheels[2]: violated")
	// Asked again, the object answers the same rows: the satisfaction's subject
	// binding classified the car, which restates no assertion about it.
	wants(t, run(t, s, "%run-query Checks root=car"),
		"✓ Query Garage::Checks returned 4 rows",
		"Row 1: assert constraint light on Garage::car: holds",
		"Row 2: satisfy lightCar by car on Garage::car: holds")
}

// nestedVerdictQueryModel nests a requirement in a part, so the case names it by
// a feature chain, and checks it from a part nested in the design.
const nestedVerdictQueryModel = `package Descent {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Engine { attribute thrust : Real; }
	requirement def ThrustMargin {
		subject engine : Engine;
		require constraint { engine.thrust >= 3000.0 }
	}
	part specification {
		requirement thrust : ThrustMargin;
	}
	part lander {
		part propulsion {
			part engine : Engine { attribute :>> thrust = 2800.0; }
			satisfy specification.thrust by engine;
		}
	}
	verification def FireEngine {
		subject engine : Engine;
		VerificationCases::PassIf(engine.thrust >= 3000.0)
	}
	verification hotFire : FireEngine {
		subject engine = lander.propulsion.engine;
		objective { verify specification.thrust; }
	}
	calc def Checks :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("kind", "verdict"))
	}
}`

// TestRunQueryVerdictsFindCasesVerifyingANestedRequirement pins that the cases verifying a
// requirement named by feature chain are reported for a declared root and a held object alike.
func TestRunQueryVerdictsFindCasesVerifyingANestedRequirement(t *testing.T) {
	s := NewSession()
	if res := s.Submit(nestedVerdictQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	wants(t, run(t, s, "%run-query Checks root=Descent::lander"),
		"✓ Query Descent::Checks returned 2 rows",
		"Row 1: satisfy specification::thrust by engine on Descent::lander.propulsion.engine: violated",
		"Row 2: verification Descent::hotFire on Descent::lander.propulsion.engine: violated",
		`kind = "verification"`)
	wants(t, run(t, s, "%instantiate Descent::lander"), "Created instance")
	wants(t, run(t, s, "%run-query Checks root=#1"),
		"✓ Query Descent::Checks returned 2 rows",
		"Row 2: verification Descent::hotFire on #1.propulsion.engine: violated")
	wants(t, run(t, s, "%validate #1"), "Verification Descent::hotFire verdict: fail")
}
