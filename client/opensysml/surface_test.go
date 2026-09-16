package opensysml_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

const behaviorSource = `package Test {
	private import ScalarValues::*;
	action addFive {
		attribute result : Integer = 0;
		first start;
		action inner {
			assign result := result + 5;
		}
		done;
		succession first start then inner;
		succession first inner then done;
	}
	state Machine {
		entry; then init;
		state init;
		state Running;

		succession first init then Running;
		succession first Running then done;
	}
}`

const verificationSource = `package Demo {
	part def Vehicle {
		attribute mass = 1500.0;

		constraint massLight {
			mass < 1300.0
		}

		requirement lightEnough {
			require constraint { mass < 2000.0 }
		}
	}

	part sedan : Vehicle {
		attribute :>> mass = 1200.0;
	}

	part analysis {
		assert satisfy sedan::lightEnough by sedan;
	}

	calc add {
		in x;
		in y;
		x + y
	}
}`

const validationSource = `package Demo {
	part def Engine {
		attribute power = 300.0;
		assert constraint { power < 200.0 }
	}

	part def Wheel {
		attribute pressure default = 32.0;
		assert constraint pressureOk { pressure >= 30.0 }
	}

	part def Car {
		attribute mass = 1500.0;
		part engine : Engine;
		part wheels : Wheel[2] {
			attribute :>> pressure = 20.0;
		}
		assert constraint massOk { mass < 2000.0 }
	}

	part car : Car;
	part sound : Wheel;

	part def Crate;
	part crate : Crate;
}`

const querySource = `package Demo {
	abstract part def Vehicle {
		attribute mass;
	}
	part def Wheel;
	part vehicle : Vehicle {
		part wheels : Wheel[4];
	}
	part spare : Wheel;
}`

const documentSource = `package Observatory {
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

	calc def Subsystems :> Query {
		in root : Element;
		WhereType(
			source = Descendants(source = root, maxDepth = 3),
			type = "PartUsage"
		)
	}

	calc def HeavySubsystems :> Query {
		in root : Element;
		in threshold : String;
		Project(
			source = OrderBy(
				source = WhereFeature(
					source = Subsystems(root = root),
					'feature' = "mass",
					operator = ">=",
					value = threshold
				),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "mass")
		)
	}

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part intro : Paragraph {
			attribute redefines text = "Mass rollup.";
		}

		part masses : Table {
			calc rows : HeavySubsystems {
				in root = telescope;
				in threshold = "10";
			}
		}
	}
}`

const verdictQuerySource = `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Engine {
		attribute power : Real = 300.0;
		assert constraint powerLow { power < 200.0 }
	}
	part def Car {
		attribute mass : Real = 1500.0;
		part engine : Engine;
		assert constraint massOk { mass < 2000.0 }
	}
	part car : Car;

	calc def Checks :> Query {
		in root : Element;
		Project(
			source = Verdicts(source = root),
			properties = ("path", "verdict")
		)
	}
}`

const editableSource = `package Demo {
	part def SC {
		attribute unitMass = 1000.0;
	}
	part sc : SC;
}`

func parse(t *testing.T, client opensysml.Client, source string) *opensysml.Model {
	t.Helper()
	model, err := client.ParseSource(context.Background(), source)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	return model
}

func TestExecuteActionAppliesTheInputsGiven(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, behaviorSource)
	run, err := client.ExecuteAction(context.Background(), model, "Test::addFive",
		map[string]opensysml.Value{"result": opensysml.Int(10)})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got, ok := run.Outputs["result"].(opensysml.Int); !ok || got != 15 {
		t.Errorf("result = %#v, want Int(15)", run.Outputs["result"])
	}
}

func TestExecuteActionRefusesAnUnsetInput(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, behaviorSource)
	_, err := client.ExecuteAction(context.Background(), model, "Test::addFive",
		map[string]opensysml.Value{"result": opensysml.Unset{}})
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}

func TestExecuteActionReportsAnUnknownActionInBand(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, behaviorSource)
	_, err := client.ExecuteAction(context.Background(), model, "Test::noSuchAction", nil)
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("err = %v, want ErrFailure", err)
	}
}

func TestExecuteStateTracesTheStatesVisited(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, behaviorSource)
	run, err := client.ExecuteState(context.Background(), model, "Test::Machine", nil)
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if strings.Join(run.Visited, ",") != "init,Running,done" {
		t.Errorf("visited = %v, want init, Running, done", run.Visited)
	}
}

func TestVerifyConstraintAnswersAVerdictAboutTheSubject(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	verification, err := client.VerifyConstraint(context.Background(), model, "Demo::Vehicle::massLight",
		opensysml.Against("Demo::sedan"))
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if verification.Verdict == nil {
		t.Fatal("verification carries no verdict")
	}
	if verification.Verdict.Undecided() {
		t.Fatalf("verdict is undecided: %s", verification.Verdict.Error)
	}
	if !verification.Verdict.Holds {
		t.Errorf("verdict = false for the subject's mass, condition %q", verification.Verdict.Condition)
	}
	if len(verification.Instances) == 0 {
		t.Error("verification names no instances")
	}
}

func TestAVerdictOfFalseIsAnAnswerNotAnError(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	verification, err := client.VerifyConstraint(context.Background(), model, "Demo::Vehicle::massLight")
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if verification.Verdict.Holds {
		t.Error("the declared mass of 1500 is not below 1300, yet the verdict holds")
	}
	if verification.Verdict.Condition == "" {
		t.Error("a false verdict names no condition")
	}
}

func TestVerifyRequirementHolds(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	verification, err := client.VerifyRequirement(context.Background(), model, "Demo::Vehicle::lightEnough",
		opensysml.Against("Demo::sedan"))
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if !verification.Verdict.Holds {
		t.Errorf("verdict = false, condition %q", verification.Verdict.Condition)
	}
}

func TestVerifyingASymbolOfAnotherKindIsUndecidedAndClassified(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	verification, err := client.VerifyConstraint(context.Background(), model, "Demo::sedan")
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if !verification.Verdict.Undecided() {
		t.Fatalf("verdict = %+v, want undecided for a part named where a constraint belongs",
			verification.Verdict)
	}
	if verification.Verdict.Reason != opensysml.ReasonWrongKind {
		t.Errorf("reason = %v, want %v", verification.Verdict.Reason, opensysml.ReasonWrongKind)
	}
}

func TestACalcOfAnotherKindIsAClassifiedFailure(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	_, err := client.EvaluateCalc(context.Background(), model, "Demo::sedan")
	var refused *opensysml.VerifyError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %T (%v), want *VerifyError", err, err)
	}
	if refused.Reason != opensysml.ReasonWrongKind {
		t.Errorf("reason = %v, want %v", refused.Reason, opensysml.ReasonWrongKind)
	}
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Error("a VerifyError does not match ErrFailure")
	}
}

func TestVerifySatisfactionAnswersEveryAssertion(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	satisfaction, err := client.VerifySatisfaction(context.Background(), model, "Demo::analysis")
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(satisfaction.Verdicts) == 0 {
		t.Fatal("no assertion was evaluated")
	}
	if !satisfaction.Holds() {
		t.Errorf("verdicts = %+v, want all holding", satisfaction.Verdicts)
	}
}

func TestValidateInstanceAnswersEveryAssertionOnEveryObject(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, validationSource)
	validation, err := client.ValidateInstance(context.Background(), model, "Demo::car")
	if err != nil {
		t.Fatalf("ValidateInstance: %v", err)
	}
	if validation.Valid() || !validation.Violated() {
		t.Errorf("valid=%v violated=%v, want a violated object", validation.Valid(), validation.Violated())
	}
	if validation.Summary == nil || validation.Summary.Kind != "object" || validation.Summary.Holds {
		t.Errorf("summary = %+v, want an object verdict of false", validation.Summary)
	}
	paths := map[string]bool{}
	for _, verdict := range validation.Verdicts {
		paths[verdict.InstancePath] = true
	}
	for _, path := range []string{"", "engine", "wheels[1]", "wheels[2]"} {
		if !paths[path] {
			t.Errorf("no verdict about the object at %q; verdicts = %+v", path, validation.Verdicts)
		}
	}
	if validation.Bounded {
		t.Error("a finite object tree was reported bounded")
	}
	if len(validation.Instances) == 0 {
		t.Error("no instance graph returned for the object")
	}

	sound, err := client.ValidateInstance(context.Background(), model, "Demo::sound")
	if err != nil {
		t.Fatalf("ValidateInstance: %v", err)
	}
	if !sound.Valid() || sound.Violated() {
		t.Errorf("verdicts = %+v, want a valid object", sound.Verdicts)
	}
}

func TestValidateInstanceTakesNoSubject(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, validationSource)
	_, err := client.ValidateInstance(context.Background(), model, "Demo::car", opensysml.Against("Demo::sound"))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Fatalf("err = %v, want an invalid argument", err)
	}
}

func TestValidateInstanceStatingNoAssertionIsNotValid(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, validationSource)
	validation, err := client.ValidateInstance(context.Background(), model, "Demo::crate")
	if err != nil {
		t.Fatalf("ValidateInstance: %v", err)
	}
	if len(validation.Verdicts) != 0 || validation.Valid() || validation.Violated() {
		t.Errorf("verdicts=%+v valid=%v violated=%v, want nothing decided", validation.Verdicts, validation.Valid(), validation.Violated())
	}
	if validation.Summary == nil || validation.Summary.Holds || validation.Summary.Error == "" {
		t.Errorf("summary = %+v, want one that says no assertion was stated", validation.Summary)
	}
}

func TestValidateInstanceOfNothingFails(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, validationSource)
	_, err := client.ValidateInstance(context.Background(), model, "Demo::nosuch")
	var refused *opensysml.VerifyError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a VerifyError", err)
	}
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Error("a VerifyError does not match ErrFailure")
	}

	for _, symbol := range []string{"Demo", "Demo::Car::mass"} {
		_, err := client.ValidateInstance(context.Background(), model, symbol)
		if !errors.As(err, &refused) || refused.Reason != opensysml.ReasonWrongKind {
			t.Errorf("ValidateInstance(%s) = %v, want a VerifyError of the wrong kind", symbol, err)
		}
	}
}

func TestEvaluateCalcAppliesPositionalArguments(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	calculation, err := client.EvaluateCalc(context.Background(), model, "Demo::add",
		opensysml.Int(2), opensysml.Int(3))
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if got, ok := calculation.Result.(opensysml.Int); !ok || got != 5 {
		t.Errorf("result = %#v, want Int(5)", calculation.Result)
	}
}

func TestQuerySelectsWithATypedFilter(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, querySource)
	elements, err := client.Query(context.Background(), model, opensysml.Query{
		Scope:  []string{"Demo"},
		Select: []string{"name", "qualifiedName"},
		Where: opensysml.All(
			opensysml.Equals("@type", "PartUsage"),
			opensysml.Equals("name", "spare").Not(),
		),
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(elements) == 0 {
		t.Fatal("no element matched")
	}
	for _, element := range elements {
		if element.Type != "PartUsage" {
			t.Errorf("element %s is a %s, which the filter excludes", element.ID, element.Type)
		}
		if element.Properties["name"] == "spare" {
			t.Error("the negated condition did not exclude spare")
		}
	}
}

func TestNotOfACompositeNegatesEveryOperand(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, querySource)
	// Not(Any(a, b)) selects what neither matches, so no wheel and no vehicle.
	elements, err := client.Query(context.Background(), model, opensysml.Query{
		Scope:  []string{"Demo"},
		Select: []string{"name"},
		Where: opensysml.Any(
			opensysml.Equals("name", "spare"),
			opensysml.Equals("name", "wheels"),
		).Not(),
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	for _, element := range elements {
		if name := element.Properties["name"]; name == "spare" || name == "wheels" {
			t.Errorf("%q matched a negated disjunction", name)
		}
	}
}

func TestQueryOSLCSelectsElements(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, querySource)
	elements, err := client.QueryOSLC(context.Background(), model,
		`oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`)
	if err != nil {
		t.Fatalf("QueryOSLC: %v", err)
	}
	if len(elements) == 0 {
		t.Error("no element matched an OSLC query for part usages")
	}
}

func TestRunDocumentQueryBindsItsParameters(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, documentSource)
	rows, err := client.RunDocumentQuery(context.Background(), model, "Observatory::HeavySubsystems",
		opensysml.Bind("root", opensysml.Element{ID: "Observatory::telescope"}),
		opensysml.Bind("threshold", opensysml.String("10")))
	if err != nil {
		t.Fatalf("RunDocumentQuery: %v", err)
	}
	if strings.Join(rows.Columns, ",") != "name,mass" {
		t.Errorf("columns = %v, want name, mass", rows.Columns)
	}
	if len(rows.Rows) != 1 {
		t.Fatalf("rows = %d, want the one subsystem at or above 10", len(rows.Rows))
	}
	if got := opensysml.CellText(rows.Rows[0].Cells[0][0]); got != "mount" {
		t.Errorf("first cell = %q, want mount", got)
	}
}

func TestADocumentQueryBindingRefusesInfinity(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, documentSource)
	_, err := client.RunDocumentQuery(context.Background(), model, "Observatory::Subsystems",
		opensysml.Bind("root", opensysml.Infinity{}))
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}

func TestRunDocumentQueryAnswersVerdictRows(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verdictQuerySource)
	rows, err := client.RunDocumentQuery(context.Background(), model, "Garage::Checks",
		opensysml.Bind("root", opensysml.Element{ID: "Garage::car"}))
	if err != nil {
		t.Fatalf("RunDocumentQuery: %v", err)
	}
	if len(rows.Rows) != 2 {
		t.Fatalf("rows = %d, want the car's constraint and its engine's", len(rows.Rows))
	}
	want := []opensysml.DocumentVerdict{
		{
			Assertion: opensysml.Element{ID: "Garage::Car::massOk", Type: "ConstraintUsage"},
			Kind:      "constraint", Text: "assert constraint massOk", Path: "Garage::car", Status: "holds",
		},
		{
			Assertion: opensysml.Element{ID: "Garage::Engine::powerLow", Type: "ConstraintUsage"},
			Kind:      "constraint", Text: "assert constraint powerLow", Path: "Garage::car.engine", Status: "violated",
		},
	}
	for i, row := range rows.Rows {
		if row.Verdict == nil {
			t.Fatalf("row %d carries no verdict", i)
		}
		got := *row.Verdict
		if got.Status == "violated" && (got.Condition == "" || got.Reason == "") {
			t.Errorf("row %d violated without its condition and reason: %+v", i, got)
		}
		got.Condition, got.Reason = "", ""
		if !reflect.DeepEqual(got, want[i]) {
			t.Errorf("row %d verdict = %+v, want %+v", i, got, want[i])
		}
		if row.Element != want[i].Assertion {
			t.Errorf("row %d element = %+v, want the assertion %+v", i, row.Element, want[i].Assertion)
		}
		if path := opensysml.CellText(row.Cells[0][0]); path != want[i].Path {
			t.Errorf("row %d path cell = %q, want %q", i, path, want[i].Path)
		}
	}
	if got := rows.Rows[1].Verdict.String(); got != "assert constraint powerLow on Garage::car.engine: violated" {
		t.Errorf("String() = %q", got)
	}
}

func TestADocumentQueryBindingRefusesAVerdict(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verdictQuerySource)
	_, err := client.RunDocumentQuery(context.Background(), model, "Garage::Checks",
		opensysml.Bind("root", opensysml.DocumentVerdict{Kind: "constraint"}))
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}

func TestCellTextRendersEveryCellKind(t *testing.T) {
	for _, testcase := range []struct {
		cell opensysml.Cell
		want string
	}{
		{opensysml.Element{ID: "Demo::sedan", Type: "PartUsage"}, "Demo::sedan"},
		{opensysml.String("mount"), "mount"},
		{opensysml.Int(15), "15"},
		{opensysml.Real(8.5), "8.5"},
		{opensysml.Bool(true), "true"},
		{opensysml.Infinity{}, "*"},
		{opensysml.DocumentVerdict{Text: "assert constraint massOk", Path: "Garage::car", Status: "holds"},
			"assert constraint massOk on Garage::car: holds"},
		{nil, ""},
	} {
		if got := opensysml.CellText(testcase.cell); got != testcase.want {
			t.Errorf("CellText(%#v) = %q, want %q", testcase.cell, got, testcase.want)
		}
	}
}

func TestRenderDocumentAnswersMarkdown(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, documentSource)
	markdown, err := client.RenderDocument(context.Background(), model, "Observatory::MassReport")
	if err != nil {
		t.Fatalf("RenderDocument: %v", err)
	}
	if !strings.Contains(markdown, "Telescope Mass Report") {
		t.Errorf("rendered document does not carry its title:\n%s", markdown)
	}
	if !strings.Contains(markdown, "mount") {
		t.Errorf("rendered document does not carry its table rows:\n%s", markdown)
	}
}

func TestConvertRefusesAFromFormatForAParsedModel(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, editableSource)
	_, err := client.Convert(context.Background(), model, opensysml.FormatSysML,
		opensysml.WithFromFormat(opensysml.FormatTTL))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Fatalf("err = %v (%T), want an invalid-argument StatusError", err, err)
	}
}

func TestModelOKIsFalseWithoutAModel(t *testing.T) {
	var model *opensysml.Model
	if model.OK() {
		t.Error("a nil model reports itself usable")
	}
}

func TestConvertWritesTheModelInAnotherNotation(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, editableSource)
	conversion, err := client.Convert(context.Background(), model, opensysml.FormatTurtle)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if conversion.Content == "" {
		t.Error("conversion carries no content")
	}
	if conversion.From != opensysml.FormatSysML || conversion.To != opensysml.FormatTTL {
		t.Errorf("conversion = %s to %s, want sysml to ttl", conversion.From, conversion.To)
	}
	if !conversion.Experimental || conversion.ExperimentalNotice == "" {
		t.Error("an RDF conversion does not report itself as experimental")
	}
}

func TestAFormatAliasIsAnsweredCanonically(t *testing.T) {
	client := newClient(t)
	conversion, err := client.ConvertSource(context.Background(), editableSource, opensysml.FormatText,
		opensysml.WithFromFormat(opensysml.FormatKerML))
	if err != nil {
		t.Fatalf("ConvertSource: %v", err)
	}
	if conversion.From != opensysml.FormatSysML || conversion.To != opensysml.FormatSysML {
		t.Errorf("conversion = %s to %s, want both answered as sysml", conversion.From, conversion.To)
	}
	if conversion.Experimental {
		t.Error("a notation-to-notation conversion reports itself as experimental")
	}
}

func TestConvertSourceReadsInlineContent(t *testing.T) {
	client := newClient(t)
	conversion, err := client.ConvertSource(context.Background(), editableSource, opensysml.FormatSysML,
		opensysml.WithFromFormat(opensysml.FormatSysML))
	if err != nil {
		t.Fatalf("ConvertSource: %v", err)
	}
	if !strings.Contains(conversion.Content, "SC") {
		t.Errorf("conversion does not carry the converted model:\n%s", conversion.Content)
	}
}

func TestConvertingToNoFormatIsInvalid(t *testing.T) {
	client := newClient(t)
	_, err := client.ConvertSource(context.Background(), editableSource, "")
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}

func TestApplyEditsAnswersTheEditedSource(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, editableSource)
	result, err := client.ApplyEdits(context.Background(), model,
		opensysml.SetValue{Target: "Demo::SC::unitMass", Value: "1050.0"})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if !strings.Contains(result.Content, "1050.0") {
		t.Errorf("edited source does not carry the new value:\n%s", result.Content)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("applied = %d edits, want 1", len(result.Applied))
	}
	if result.Applied[0].OldText != "1000.0" || result.Applied[0].NewText != "1050.0" {
		t.Errorf("applied = %+v, want 1000.0 replaced by 1050.0", result.Applied[0])
	}
}

func TestApplyEditsListsTheOneDocumentUnderTheParsesName(t *testing.T) {
	client := newClient(t)
	model, err := client.ParseDocuments(context.Background(),
		[]opensysml.Document{{Name: "demo.sysml", Content: editableSource}})
	if err != nil {
		t.Fatalf("ParseDocuments: %v", err)
	}
	result, err := client.ApplyEdits(context.Background(), model,
		opensysml.SetValue{Target: "Demo::SC::unitMass", Value: "1050.0"})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if len(result.Documents) != 1 || result.Documents[0].Name != "demo.sysml" {
		t.Fatalf("documents = %+v, want the one document named demo.sysml", result.Documents)
	}
	if result.Documents[0].Content != result.Content || result.Content == "" {
		t.Errorf("content = %q, documents[0].content = %q, want the same notation", result.Content, result.Documents[0].Content)
	}
	if len(result.Applied) != 1 || result.Applied[0].Document != "demo.sysml" {
		t.Errorf("applied = %+v, want one edit in demo.sysml", result.Applied)
	}
}

const (
	multiDocLibrary = "package Lib {\n    part def Engine;\n    part def Wheel;\n}\n"
	multiDocUser    = "package Car {\n    part engine : Lib::Engine;\n    part wheel : Lib::Wheel;\n}\n"
)

func parseTwo(t *testing.T, client opensysml.Client) *opensysml.Model {
	t.Helper()
	model, err := client.ParseDocuments(context.Background(), []opensysml.Document{
		{Name: "lib.sysml", Content: multiDocLibrary},
		{Name: "car.sysml", Content: multiDocUser},
	})
	if err != nil {
		t.Fatalf("ParseDocuments: %v", err)
	}
	return model
}

func TestApplyEditsRenamesAcrossDocumentsAndLeavesContentEmpty(t *testing.T) {
	client := newClient(t)
	model := parseTwo(t, client)
	result, err := client.ApplyEdits(context.Background(), model,
		opensysml.Rename{Target: "Lib::Engine", NewName: "Motor"})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if result.Content != "" {
		t.Errorf("content = %q, want empty for a model of two documents", result.Content)
	}
	if len(result.Documents) != 2 || result.Documents[0].Name != "lib.sysml" || result.Documents[1].Name != "car.sysml" {
		t.Fatalf("documents = %+v, want lib.sysml then car.sysml", result.Documents)
	}
	if !strings.Contains(result.Documents[0].Content, "part def Motor") ||
		!strings.Contains(result.Documents[1].Content, "engine : Lib::Motor") {
		t.Errorf("the rename did not reach both documents:\n%s\n%s", result.Documents[0].Content, result.Documents[1].Content)
	}
	for _, applied := range result.Applied {
		if applied.Document == "" {
			t.Errorf("applied edit %+v names no document", applied)
		}
	}
}

func TestApplyDocumentEditsTargetsTheNamedDocument(t *testing.T) {
	client := newClient(t)
	model := parseTwo(t, client)
	result, err := client.ApplyDocumentEdits(context.Background(), model, "car.sysml",
		opensysml.Rename{Target: "Car::wheel", NewName: "tyre"})
	if err != nil {
		t.Fatalf("ApplyDocumentEdits: %v", err)
	}
	if len(result.Documents) != 1 || result.Documents[0].Name != "car.sysml" {
		t.Fatalf("documents = %+v, want car.sysml alone", result.Documents)
	}
	if result.Content != "" {
		t.Errorf("content = %q, want empty for a model of two documents", result.Content)
	}
	_, err = client.ApplyDocumentEdits(context.Background(), model, "nope.sysml",
		opensysml.Rename{Target: "Car::wheel", NewName: "tyre"})
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("an unknown document: err = %v, want CodeInvalidArgument", err)
	}
}

func TestApplyEditsRefusalNamesReferrersByDocument(t *testing.T) {
	client := newClient(t)
	model := parseTwo(t, client)
	_, err := client.ApplyEdits(context.Background(), model,
		opensysml.Delete{Target: "Lib::Engine"})
	var refused *opensysml.EditError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %T (%v), want *EditError", err, err)
	}
	if refused.Failure != opensysml.EditFailureDeleteReferenced {
		t.Fatalf("failure = %v, want %v", refused.Failure, opensysml.EditFailureDeleteReferenced)
	}
	want := []opensysml.Referrer{{Name: "Car::engine", Document: "car.sysml"}}
	if !reflect.DeepEqual(refused.Referrers, want) {
		t.Errorf("referrers = %+v, want %+v", refused.Referrers, want)
	}
	if !reflect.DeepEqual(refused.Referring, []string{"Car::engine (car.sysml)"}) {
		t.Errorf("referring = %q, want the referrer qualified by its document", refused.Referring)
	}
}

// A service without edit_documents edits one document alone and answers Content
// alone, so a client checks the capability before reading Documents, before
// editing a model of several documents, and before naming a document.
func TestApplyEditsWithoutEditDocumentsAnswersContentAlone(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityEditDocuments})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := dialClient(t, server.URL)
	ctx := context.Background()

	info, err := client.ServerInfo(ctx)
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if info.Has(opensysml.CapabilityEditDocuments) || !info.Has(opensysml.CapabilityApplyEdits) {
		t.Fatalf("capabilities = %v, want apply_edits without edit_documents", info.Capabilities)
	}

	one := parse(t, client, editableSource)
	result, err := client.ApplyEdits(ctx, one, opensysml.SetValue{Target: "Demo::SC::unitMass", Value: "1050.0"})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if !strings.Contains(result.Content, "1050.0") || len(result.Documents) != 0 {
		t.Errorf("content=%q documents=%+v, want the notation in Content alone", result.Content, result.Documents)
	}
	if len(result.Applied) != 1 || result.Applied[0].Document != "" {
		t.Errorf("applied = %+v, want one edit naming no document", result.Applied)
	}

	two := parseTwo(t, client)
	_, err = client.ApplyEdits(ctx, two, opensysml.Rename{Target: "Lib::Engine", NewName: "Motor"})
	if !errors.Is(err, opensysml.CodeFailedPrecondition) || !strings.Contains(err.Error(), opensysml.CapabilityEditDocuments) {
		t.Errorf("a model of two: err = %v, want CodeFailedPrecondition naming edit_documents", err)
	}
	_, err = client.ApplyDocumentEdits(ctx, two, "car.sysml", opensysml.Rename{Target: "Car::wheel", NewName: "tyre"})
	if !errors.Is(err, opensysml.CodeUnimplemented) || !strings.Contains(err.Error(), opensysml.CapabilityEditDocuments) {
		t.Errorf("naming a document: err = %v, want CodeUnimplemented naming edit_documents", err)
	}
}

func TestApplyEditsRefusesAllOrNothingWithAClassifiedFailure(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, editableSource)
	_, err := client.ApplyEdits(context.Background(), model,
		opensysml.SetValue{Target: "Demo::SC::unitMass", Value: "1050.0"},
		opensysml.SetValue{Target: "Demo::SC::noSuchAttribute", Value: "1.0"})
	var refused *opensysml.EditError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %T (%v), want *EditError", err, err)
	}
	if refused.Failure != opensysml.EditFailureUnknownTarget {
		t.Errorf("failure = %v, want %v", refused.Failure, opensysml.EditFailureUnknownTarget)
	}
}

func TestApplyEditsRenamesAndRewritesTheReferences(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, editableSource)
	result, err := client.ApplyEdits(context.Background(), model,
		opensysml.Rename{Target: "Demo::SC", NewName: "Spacecraft"})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if strings.Contains(result.Content, ": SC") {
		t.Errorf("the reference to the renamed element still reads SC:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "part sc : Spacecraft") {
		t.Errorf("the reference was not rewritten:\n%s", result.Content)
	}
}

func TestApplyEditsRefusesDeletingAReferencedElement(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, editableSource)
	_, err := client.ApplyEdits(context.Background(), model,
		opensysml.Delete{Target: "Demo::SC"})
	var refused *opensysml.EditError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %T (%v), want *EditError", err, err)
	}
	if refused.Failure != opensysml.EditFailureDeleteReferenced {
		t.Fatalf("failure = %v, want %v", refused.Failure, opensysml.EditFailureDeleteReferenced)
	}
	if len(refused.Referring) == 0 {
		t.Error("the refusal names nothing that refers to the element")
	}
}

func TestApplyEditsAddsAMember(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, editableSource)
	result, err := client.ApplyEdits(context.Background(), model,
		opensysml.AddMember{Owner: "Demo::SC", Kind: "attribute", Name: "margin", Value: "50.0"})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if !strings.Contains(result.Content, "attribute margin") {
		t.Errorf("the member was not added:\n%s", result.Content)
	}
}

func TestEveryOperationIsRefusedAfterClose(t *testing.T) {
	client, err := opensysml.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	model := parse(t, client, verificationSource)
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"ExecuteAction": func() error {
			_, err := client.ExecuteAction(ctx, model, "Demo::add", nil)
			return err
		},
		"ExecuteState": func() error {
			_, err := client.ExecuteState(ctx, model, "Demo::Machine", nil)
			return err
		},
		"ExploreAction": func() error {
			_, err := client.ExploreAction(ctx, model, "Demo::add", nil)
			return err
		},
		"ExploreState": func() error {
			_, err := client.ExploreState(ctx, model, "Demo::Machine", nil)
			return err
		},
		"ExploreAnalysis": func() error {
			_, err := client.ExploreAnalysis(ctx, model, "Demo::analysis")
			return err
		},
		"VerifyConstraint": func() error {
			_, err := client.VerifyConstraint(ctx, model, "Demo::Vehicle::massLight")
			return err
		},
		"VerifyRequirement": func() error {
			_, err := client.VerifyRequirement(ctx, model, "Demo::Vehicle::lightEnough")
			return err
		},
		"VerifySatisfaction": func() error {
			_, err := client.VerifySatisfaction(ctx, model, "Demo::analysis")
			return err
		},
		"ValidateInstance": func() error {
			_, err := client.ValidateInstance(ctx, model, "Demo::sedan")
			return err
		},
		"EvaluateCalc": func() error {
			_, err := client.EvaluateCalc(ctx, model, "Demo::add", opensysml.Int(1), opensysml.Int(2))
			return err
		},
		"Query": func() error {
			_, err := client.Query(ctx, model, opensysml.Query{})
			return err
		},
		"QueryOSLC": func() error {
			_, err := client.QueryOSLC(ctx, model, `name="sedan"`)
			return err
		},
		"RunDocumentQuery": func() error {
			_, err := client.RunDocumentQuery(ctx, model, "Demo::Query")
			return err
		},
		"RenderDocument": func() error {
			_, err := client.RenderDocument(ctx, model, "Demo::Report")
			return err
		},
		"Convert": func() error {
			_, err := client.Convert(ctx, model, opensysml.FormatText)
			return err
		},
		"ConvertFile": func() error {
			_, err := client.ConvertFile(ctx, "model.sysml", opensysml.FormatText)
			return err
		},
		"ConvertSource": func() error {
			_, err := client.ConvertSource(ctx, editableSource, opensysml.FormatText)
			return err
		},
		"ApplyEdits": func() error {
			_, err := client.ApplyEdits(ctx, model, opensysml.Delete{Target: "Demo::sedan"})
			return err
		},
		"ApplyDocumentEdits": func() error {
			_, err := client.ApplyDocumentEdits(ctx, model, "<content>", opensysml.Delete{Target: "Demo::sedan"})
			return err
		},
	} {
		if err := call(); !errors.Is(err, opensysml.CodeUnavailable) {
			t.Errorf("%s after Close: err = %v, want CodeUnavailable", name, err)
		}
	}
}

func TestANilModelIsInvalidForEveryOperation(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"ExecuteAction": func() error {
			_, err := client.ExecuteAction(ctx, nil, "Demo::add", nil)
			return err
		},
		"VerifyConstraint": func() error {
			_, err := client.VerifyConstraint(ctx, nil, "Demo::c")
			return err
		},
		"EvaluateCalc": func() error {
			_, err := client.EvaluateCalc(ctx, nil, "Demo::add")
			return err
		},
		"Query": func() error {
			_, err := client.Query(ctx, nil, opensysml.Query{})
			return err
		},
		"RunDocumentQuery": func() error {
			_, err := client.RunDocumentQuery(ctx, nil, "Demo::Query")
			return err
		},
		"ApplyEdits": func() error {
			_, err := client.ApplyEdits(ctx, nil, opensysml.Delete{Target: "Demo::sedan"})
			return err
		},
		"ApplyDocumentEdits": func() error {
			_, err := client.ApplyDocumentEdits(ctx, nil, "<content>", opensysml.Delete{Target: "Demo::sedan"})
			return err
		},
	} {
		if err := call(); !errors.Is(err, opensysml.CodeInvalidArgument) {
			t.Errorf("%s with a nil model: err = %v, want CodeInvalidArgument", name, err)
		}
	}
}

func TestAModelThatNoParseAnsweredIsInvalid(t *testing.T) {
	client := newClient(t)
	_, err := client.VerifyConstraint(context.Background(), &opensysml.Model{}, "Demo::c")
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}
