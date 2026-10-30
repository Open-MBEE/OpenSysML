package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// queryModel declares a document query over a small part tree, so -run-query
// decides both ways: rows reported, and a failure that gates a build.
const queryModel = `package Observatory {
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
		part mount : Subsystem {
			attribute redefines mass = 15.0;
		}
	}

	calc def HeavySubsystems :> Query {
		in root : Element;
		Project(
			source = WhereFeature(
				source = WhereType(
					source = Descendants(source = root, maxDepth = 3),
					type = "PartUsage"
				),
				'feature' = "mass",
				operator = ">=",
				value = "10"
			),
			properties = ("name", "mass")
		)
	}

	calc def NamedTelescopeParts :> Query {
		in root : Element;
		in pattern : String;
		WhereName(source = OwnedElements(source = root), operator = "startsWith", value = pattern)
	}

	calc def DefaultedTelescopeParts :> NamedTelescopeParts {
		in redefines root = telescope;
		in redefines pattern default "op";
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

// TestRunQueryFlag checks the scripted surface of document queries: rows on
// stdout for a query that ran, repeatability, and an unresolved status when
// the query could not be run at all.
func TestRunQueryFlag(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, queryModel, "-run-query", "HeavySubsystems root=Observatory::telescope"),
		0, "✓ Query Observatory::HeavySubsystems returned 1 row",
		"Columns: name, mass",
		"Row 1: Observatory::telescope::mount",
		`name = "mount"`,
		"mass = 15.0")

	// A binding expression may contain unquoted spaces.
	wantReport(t, check(t, binary, queryModel, "-run-query", `NamedTelescopeParts root=telescope pattern="m" + "o"`),
		0, "✓ Query Observatory::NamedTelescopeParts returned 1 row")

	// Omitted parameters take their declared defaults; an explicit binding overrides one.
	wantReport(t, check(t, binary, queryModel, "-run-query", "DefaultedTelescopeParts"),
		0, "✓ Query Observatory::DefaultedTelescopeParts returned 1 row",
		"Row 1: Observatory::telescope::optics")
	wantReport(t, check(t, binary, queryModel, "-run-query", `DefaultedTelescopeParts pattern="mo"`),
		0, "✓ Query Observatory::DefaultedTelescopeParts returned 1 row",
		"Row 1: Observatory::telescope::mount")

	// The flag repeats, and a later failure gates the run.
	wantReport(t, check(t, binary, queryModel,
		"-run-query", "HeavySubsystems root=telescope",
		"-run-query", "NoSuchQuery root=telescope"),
		2, "✓ Query Observatory::HeavySubsystems returned 1 row", "NoSuchQuery")

	wantReport(t, check(t, binary, queryModel, "-run-query", "HeavySubsystems"),
		2, "requires binding root")

	// A composed query runs through its invoked definition.
	wantReport(t, check(t, binary, queryModel, "-run-query", "ComposedQuery root=telescope"),
		0, "✓ Query Observatory::ComposedQuery returned 1 row",
		"Row 1: Observatory::telescope::mount")

	// A named relationship traversal reports its related elements.
	wantReport(t, check(t, binary, queryModel, "-run-query", "RelatedQuery root=Subsystem"),
		0, "✓ Query Observatory::RelatedQuery returned 2 rows",
		"Row 1: Observatory::telescope::optics",
		"Row 2: Observatory::telescope::mount")

	// An unsupported relationship kind is surfaced, not silently skipped.
	wantReport(t, check(t, binary, queryModel, "-run-query", "UnknownRelatedQuery root=telescope"),
		2, `does not support relationship kind "containment"`)
}

// derivedModel declares attributes whose values are expressions over other
// features, bound through type- and usage-level redefinitions of the carrier.
const derivedModel = `package DerivedRepro {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	part def Stage {
		attribute dryMass :> ISQ::mass;
		attribute propellantMass :> ISQ::mass;
		attribute mass :> ISQ::mass = dryMass + propellantMass;
	}
	part def FirstStage :> Stage {
		attribute :>> dryMass default = 130000 [kg];
		attribute :>> propellantMass = 2160000 [kg];
	}
	part def Vehicle {
		part s1 : FirstStage;
		part s2 : FirstStage {
			attribute :>> dryMass = 120000 [kg];
		}
		attribute liftoffMass :> ISQ::mass = s1.mass + s2.mass;
	}
	part rocket : Vehicle;

	calc def Masses :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "dryMass", "mass")
		)
	}
	calc def Vehicles :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "liftoffMass")
		)
	}
}
`

// TestRunQueryDerivedValues checks that -run-query reports attributes derived
// from sibling features and feature chains, evaluated for each row's element.
func TestRunQueryDerivedValues(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, derivedModel, "-run-query", "DerivedRepro::Masses root=DerivedRepro::Vehicle"),
		0, "✓ Query DerivedRepro::Masses returned 2 rows",
		"Columns: name, dryMass, mass",
		"Row 1: DerivedRepro::Vehicle::s1",
		"dryMass = 130000 [kg]",
		"mass = 2290000 [kg]",
		"Row 2: DerivedRepro::Vehicle::s2",
		"dryMass = 120000 [kg]",
		"mass = 2280000 [kg]")

	wantReport(t, check(t, binary, derivedModel, "-run-query", "DerivedRepro::Vehicles root=DerivedRepro"),
		0, "✓ Query DerivedRepro::Vehicles returned 1 row",
		"Row 1: DerivedRepro::rocket",
		"liftoffMass = 4570000 [kg]")
}

// TestRunQueryJSONOutcome checks that a query that could not be run reports the
// QVTS record behind its error as checks[].outcome, in the shape qvts records.
func TestRunQueryJSONOutcome(t *testing.T) {
	binary := buildCLI(t)
	model := `package Observatory {
	private import DocumentQueries::*;
	part telescope;
	calc def Misspelled :> Query { Named(qualifiedName = "Observatory::telescop") }
}`
	got := check(t, binary, model, "-run-query", "Misspelled", "-json")
	var report struct {
		Checks []struct {
			Status  string   `json:"status"`
			Lines   []string `json:"lines"`
			Outcome struct {
				Outcome  string `json:"outcome"`
				Step     string `json:"step"`
				Failures []struct {
					Expected string `json:"expected"`
					Frontier struct {
						At      string     `json:"at"`
						Members [][]string `json:"members"`
						Nearest string     `json:"nearest"`
					} `json:"frontier"`
				} `json:"failures"`
			} `json:"outcome"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if len(report.Checks) != 1 || report.Checks[0].Status != "unresolved" {
		t.Fatalf("checks = %+v, want one unresolved", report.Checks)
	}
	outcome := report.Checks[0].Outcome
	if outcome.Outcome != "pathError" || outcome.Step != "Named(Observatory::telescop)" || len(outcome.Failures) != 1 {
		t.Fatalf("outcome = %+v", outcome)
	}
	frontier := outcome.Failures[0].Frontier
	if frontier.At != "Observatory" || frontier.Nearest != "telescope" || len(frontier.Members) == 0 || len(frontier.Members[0]) != 2 {
		t.Errorf("frontier = %+v, want Observatory, its members as [name, metaclass] pairs, nearest telescope", frontier)
	}
	if lines := report.Checks[0].Lines; len(lines) < 2 || !strings.HasPrefix(lines[1], "  path error at Named(") {
		t.Errorf("lines = %q, want the record under the error line", lines)
	}
}

// TestRunQueryJSON checks that a query's verdict is reported as data a build
// step reads: its row count and projected columns as named values, and its table.
func TestRunQueryJSON(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, queryModel, "-run-query", "HeavySubsystems root=telescope", "-json")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	var report struct {
		Status string `json:"status"`
		Checks []struct {
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Values  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"values"`
			Table *struct {
				Columns []string `json:"columns"`
				Rows    []struct {
					Element string     `json:"element"`
					Cells   [][]string `json:"cells"`
				} `json:"rows"`
			} `json:"table"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if report.Status != "holds" || len(report.Checks) != 1 {
		t.Fatalf("report = %q with %d checks, want holds with 1:\n%s", report.Status, len(report.Checks), got.stdout)
	}
	values := map[string]string{}
	for _, value := range report.Checks[0].Values {
		values[value.Name] = value.Value
	}
	if values["rows"] != "1" || values["columns"] != "name, mass" {
		t.Errorf("values = %v, want rows = 1 and columns = name, mass", report.Checks[0].Values)
	}
	table := report.Checks[0].Table
	if table == nil || strings.Join(table.Columns, ",") != "name,mass" || len(table.Rows) != 1 {
		t.Fatalf("table = %+v, want columns name, mass and one row", table)
	}
	row := table.Rows[0]
	if row.Element != "Observatory::telescope::mount" || len(row.Cells) != 2 ||
		strings.Join(row.Cells[0], "|") != `"mount"` || strings.Join(row.Cells[1], "|") != "15.0" {
		t.Errorf("row = %+v, want mount with cells [\"mount\"] and [15.0]", row)
	}
}

// objectQueryModel declares queries a run can answer about an object it
// creates: the parts an object holds, and the objects a session holds by type.
const objectQueryModel = `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Integer = 30;
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
}
`

// TestRunQueryOverObjects checks that -instantiate puts the object under its
// name for -run-query: the query runs over the object's parts by path and
// reads their feature values, and Objects enumerates what the run holds.
func TestRunQueryOverObjects(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, objectQueryModel, "-run-query", "Parts root=car"),
		0, "✓ Query Garage::Parts returned 0 rows")

	wantReport(t, check(t, binary, objectQueryModel, "-instantiate", "Garage::car", "-run-query", "Parts root=car"),
		0, "✓ Query Garage::Parts returned 2 rows",
		"Columns: name, pressure",
		"Row 1: Garage::car.wheels[1] (#2)",
		`name = "wheels[1]"`,
		"pressure = 30",
		"Row 2: Garage::car.wheels[2] (#3)")

	wantReport(t, check(t, binary, objectQueryModel, "-instantiate", "Garage::car", "-run-query", "Parts root=#1"),
		0, "✓ Query Garage::Parts returned 2 rows", "Row 1: #1.wheels[1] (#2)")

	wantReport(t, check(t, binary, objectQueryModel, "-instantiate", "Garage::car", "-run-query", "Wheels"),
		0, "✓ Query Garage::Wheels returned 2 rows",
		"Row 1: Garage::car.wheels[1] (#2)", "Row 2: Garage::car.wheels[2] (#3)")

	// Without -instantiate the run holds no object to enumerate.
	wantReport(t, check(t, binary, objectQueryModel, "-run-query", "Wheels"),
		0, "✓ Query Garage::Wheels returned 0 rows")
}

// verdictQueryModel declares a car whose constraint holds and whose wheels'
// constraint fails, with a query over the verdicts about it.
const verdictQueryModel = `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Integer = 30;
		assert constraint inflated { pressure >= 35 }
	}

	part def Car {
		attribute mass : Integer = 1500;
		part wheels : Wheel[2];
		assert constraint light { mass < 2000 }
	}

	part car : Car;

	calc def Checks :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("path", "verdict"))
	}
}
`

// TestRunQueryReportsVerdicts checks that -run-query reports one row per
// assertion about the bound object and the objects it holds, over the
// instantiated object or, without -instantiate, the element as declared.
func TestRunQueryReportsVerdicts(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, verdictQueryModel, "-run-query", "Checks root=car"),
		0, "✓ Query Garage::Checks returned 3 rows",
		"Columns: path, verdict",
		"Row 1: assert constraint light on Garage::car: holds",
		`path = "Garage::car"`,
		`verdict = "holds"`,
		"Row 2: assert constraint inflated on Garage::car.wheels[1]: violated",
		`verdict = "violated"`)

	wantReport(t, check(t, binary, verdictQueryModel, "-instantiate", "Garage::car", "-run-query", "Checks root=#1"),
		0, "✓ Query Garage::Checks returned 3 rows",
		"Row 3: assert constraint inflated on #1.wheels[2]: violated")
}

// stateQueryModel declares a lamp whose machine switches itself on the clock,
// with queries over the state it is in and the transitions its trace records.
const stateQueryModel = `package Blink {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;

	state def Blinker {
		entry; then off;
		state off;
		transition off_on first off accept after 1 [s] then on;
		state on;
		transition on_off first on accept after 2 [s] then off;
	}
	part def Lamp { exhibit state lp : Blinker; }
	part lamp : Lamp;

	calc def Standing :> Query {
		Project(source = States(source = Objects(type = "Lamp")), properties = ("path", "machine", "statePath"))
	}
	calc def Switched :> Query {
		in root : Element;
		Project(
			source = Events(source = root, kind = "transition", since = 1 [s], before = 3 [s]),
			properties = ("time", "from", "to")
		)
	}
}
`

// TestRunQueryOverStatesAndTrace checks -run-query runs after the behaviors named:
// states read where -advance left them, events read -trace's record or are refused.
func TestRunQueryOverStatesAndTrace(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, stateQueryModel, "-run-query", "Standing"),
		0, "✓ Query Blink::Standing returned 0 rows")

	wantReport(t, check(t, binary, stateQueryModel, "-instantiate", "lamp", "-run-query", "Standing"),
		0, "✓ Query Blink::Standing returned 1 row",
		"Row 1: Blink::lamp.lp in off",
		`statePath = "off"`)

	run := check(t, binary, stateQueryModel, "-trace", "-instantiate", "lamp", "-state", "lp lamp", "-advance", "4",
		"-run-query", "Standing", "-run-query", "Switched root=lamp")
	wantReport(t, run,
		0, "✓ Advanced to 4.0",
		"✓ Query Blink::Standing returned 1 row",
		"Row 1: Blink::lamp.lp in on",
		"✓ Query Blink::Switched returned 1 row",
		"Columns: time, from, to",
		"Row 1: t=1 Blink::lamp.lp: transition: off -> on (event: time)",
		"time = 1.0 [s]",
		`from = "off"`,
		`to = "on"`)
	if strings.Index(run.output(), "✓ Advanced to 4.0") > strings.Index(run.output(), "✓ Query Blink::Standing") {
		t.Errorf("queries ran before the behaviors:\n%s", run.output())
	}

	wantReport(t, check(t, binary, stateQueryModel, "-instantiate", "lamp", "-state", "lp lamp", "-advance", "4",
		"-run-query", "Switched root=lamp"),
		2, "query Blink::Switched operation events reads the session's trace, and this session records none")
}

// anonymousModel declares parts without names, which the prompt prints blank.
const anonymousModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	part def Subsystem;

	part telescope {
		part : Subsystem;
		part : Subsystem;
	}

	calc def AnonymousParts :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("elementId")
		)
	}
}
`

// TestRunQueryJSONTableSpellsAnonymousElementsByID checks that an element without a
// name is spelled in the table by its elementId — the id its own elementId cell reads.
func TestRunQueryJSONTableSpellsAnonymousElementsByID(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, anonymousModel, "-run-query", "AnonymousParts root=telescope", "-json")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	var report struct {
		Checks []struct {
			Table *struct {
				Columns []string `json:"columns"`
				Rows    []struct {
					Element string     `json:"element"`
					Cells   [][]string `json:"cells"`
				} `json:"rows"`
			} `json:"table"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if len(report.Checks) != 1 || report.Checks[0].Table == nil || len(report.Checks[0].Table.Rows) != 2 {
		t.Fatalf("want one check with a two-row table:\n%s", got.stdout)
	}
	seen := map[string]bool{}
	for _, row := range report.Checks[0].Table.Rows {
		if row.Element == "" || seen[row.Element] {
			t.Errorf("row element spelled %q, want a distinct non-empty elementId", row.Element)
		}
		seen[row.Element] = true
		if len(row.Cells) != 1 || strings.Join(row.Cells[0], "|") != strconv.Quote(row.Element) {
			t.Errorf("row %s: elementId cell = %v, want the element's own id", row.Element, row.Cells)
		}
	}
}
