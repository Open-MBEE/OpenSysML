package opensysml_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
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
