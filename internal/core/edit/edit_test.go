package edit

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// libraryIndex builds an index holding the standard library and no document, as
// the service does, so a fixture resolves ISQ and SI the way a real model does.
func libraryIndex(t *testing.T) *symbols.Index {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.ExpandWildcardImports()
	return idx
}

// load parses a fixture the way the service parses a model, and returns it ready
// to edit.
func load(t *testing.T, fixture string) Model {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return loadContent(t, fixture, string(content))
}

func loadContent(t *testing.T, name, content string) Model {
	t.Helper()
	sf := source.New(name, []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()
	idx := libraryIndex(t)
	idx.AddDocument(name, root)
	var sem []passes.Diagnostic
	if len(p.Diagnostics) == 0 {
		sem = passes.Analyze(name, root, nil, idx)
	}
	return Model{
		Source:     sf,
		Root:       root,
		Index:      idx,
		ParseDiags: p.Diagnostics,
		SemDiags:   sem,
		NewIndex:   func() *symbols.Index { return libraryIndex(t) },
	}
}

// loadWorkspace is loadContent with sibling workspace documents indexed beside
// the edited one, the way the language server edits a document of several.
func loadWorkspace(t *testing.T, name, content string, siblings map[string]string) Model {
	t.Helper()
	m := loadContent(t, name, content)
	parsed := map[string]*ast.RootNamespace{}
	for sibling, text := range siblings {
		parsed[sibling] = parser.New(source.New(sibling, []byte(text))).ParseFile()
		m.Index.AddDocument(sibling, parsed[sibling])
	}
	m.Index.AddDocument(name, m.Root)
	m.Index.ExpandWildcardImports()
	if len(m.ParseDiags) == 0 {
		m.SemDiags = passes.Analyze(name, m.Root, nil, m.Index)
	}
	m.NewIndex = func() *symbols.Index {
		idx := libraryIndex(t)
		for sibling, root := range parsed {
			idx.AddDocument(sibling, root)
		}
		return idx
	}
	return m
}

// requireClean fails when a fixture does not start out valid: a test about what
// an edit introduced says nothing if the original was already broken.
func requireClean(t *testing.T, m Model) {
	t.Helper()
	if len(m.ParseDiags) > 0 {
		t.Fatalf("fixture does not parse: %v", m.ParseDiags)
	}
	for _, d := range m.SemDiags {
		if d.Severity == passes.SeverityError {
			t.Fatalf("fixture is not valid: %s at %v", d.Message, d.Span)
		}
	}
}

func TestAddMemberIntoBodyAndRoot(t *testing.T) {
	m := loadContent(t, "add.sysml", "package P {\n}\n")
	res, err := Apply(m, []Operation{
		AddMember("P", "part def", "Wheel"),
		AddMember("", "part def", "Vehicle"),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "package P {\n    part def Wheel;\n}\npart def Vehicle;\n"
	if string(res.Content) != want {
		t.Fatalf("content = %q, want %q", res.Content, want)
	}
}

func TestAddMemberBodylessOwner(t *testing.T) {
	m := loadContent(t, "add.sysml", "part def Vehicle;\n")
	res, err := Apply(m, []Operation{AddMember("Vehicle", "part", "wheel")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := string(res.Content), "part def Vehicle {\n    part wheel;\n}\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestAddMemberSameOwnerPreservesRequestOrder(t *testing.T) {
	m := loadContent(t, "add.sysml", "package P {\n}\n")
	res, err := Apply(m, []Operation{
		AddMember("P", "part def", "First"),
		AddMember("P", "part def", "Second"),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := string(res.Content),
		"package P {\n    part def First;\n    part def Second;\n}\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestDeleteOwnCommentAndLine(t *testing.T) {
	m := loadContent(t, "delete.sysml", "package P {\n    // keep this\n    part def Keep;\n\n    // remove this\n    part def Gone;\n}\n")
	res, err := Apply(m, []Operation{Delete("P::Gone", false)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "package P {\n    // keep this\n    part def Keep;\n}\n"
	if got := string(res.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

// applyOne applies a single operation and returns the edited content.
func applyOne(t *testing.T, m Model, op Operation) *Result {
	t.Helper()
	res, err := Apply(m, []Operation{op})
	if err != nil {
		t.Fatalf("Apply(%v): %v", op, err)
	}
	return res
}

// assertOnlySpanChanged checks that every byte outside the applied ranges is the
// byte the original had there.
func assertOnlySpanChanged(t *testing.T, m Model, res *Result) {
	t.Helper()
	want := m.Source.Bytes()
	got := res.Content
	// Rebuild the original from the result by undoing each applied edit, which
	// only succeeds if nothing else moved.
	rebuilt := got
	for i := len(res.Applied) - 1; i >= 0; i-- {
		a := res.Applied[i]
		shift := 0
		for _, other := range res.Applied {
			if other.Span.Offset < a.Span.Offset {
				shift += len(other.NewText) - other.Span.Len
			}
		}
		at := a.Span.Offset + shift
		if at+len(a.NewText) > len(rebuilt) || string(rebuilt[at:at+len(a.NewText)]) != a.NewText {
			t.Fatalf("applied edit %d is not at offset %d of the result", i, at)
		}
		rebuilt = append(append(append([]byte{}, rebuilt[:at]...), a.OldText...), rebuilt[at+len(a.NewText):]...)
	}
	if !bytes.Equal(rebuilt, want) {
		t.Fatalf("bytes outside the edited spans changed:\n--- original\n%s\n--- undone\n%s", want, rebuilt)
	}
}

// editError returns the *Error a refusal carries.
func editError(t *testing.T, err error) *Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected a refusal, got none")
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("error %v is not an *edit.Error", err)
	}
	return e
}

func TestSetValueReplacesOnlyTheValueSpan(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	res := applyOne(t, m, SetValue("Demo::SC::unitMass", "1050.0[SI::kg]"))

	if !strings.Contains(string(res.Content), "attribute unitMass : ISQ::MassValue default = 1050.0[SI::kg];") {
		t.Fatalf("value not set:\n%s", res.Content)
	}
	if strings.Contains(string(res.Content), "1000.0[SI::kg]") {
		t.Fatalf("old value still present:\n%s", res.Content)
	}
	assertOnlySpanChanged(t, m, res)

	if len(res.Applied) != 1 {
		t.Fatalf("applied %d edits, want 1", len(res.Applied))
	}
	a := res.Applied[0]
	if a.OldText != "1000.0[SI::kg]" || a.NewText != "1050.0[SI::kg]" || a.Target != "Demo::SC::unitMass" {
		t.Fatalf("applied edit reports %+v", a)
	}

	// The edited model parses with the diagnostics the original had.
	after := loadContent(t, "spacecraft.sysml", string(res.Content))
	requireClean(t, after)
}

func TestSetValuePreservesCommentsAndBlankLines(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	res := applyOne(t, m, SetValue("Demo::SC::unitMass", "1050.0[SI::kg]"))

	for _, want := range []string{
		"// Spacecraft mass model.\n",
		"\t/*\n\t * The spacecraft, with its mass properties.\n\t */\n",
		"\t\t// dry mass, as a quantity with a unit\n",
		"\n\t\tattribute label : ScalarValues::String = \"flight-1\";\n",
	} {
		if !strings.Contains(string(res.Content), want) {
			t.Fatalf("edited source lost %q:\n%s", want, res.Content)
		}
	}
}

// A comment or a line break written between a value and its `;` survives the
// value being changed: only the value's own tokens are spliced, not the node
// span, which runs on to the next token.
func TestSetValueKeepsWhatFollowsTheValue(t *testing.T) {
	m := loadContent(t, "trivia.sysml",
		"package Demo {\n\tattribute mass = 1000.0 /* measured on the bench */ ;\n"+
			"\tattribute count = 2\n\t\t;\n}\n")
	requireClean(t, m)

	res := applyOne(t, m, SetValue("Demo::mass", "2000.0"))
	if got := res.Applied[0].OldText; got != "1000.0" {
		t.Fatalf("replaced %q, want just the value", got)
	}
	if !strings.Contains(string(res.Content), "= 2000.0 /* measured on the bench */ ;") {
		t.Fatalf("the comment after the value was lost:\n%s", res.Content)
	}

	res = applyOne(t, m, SetValue("Demo::count", "3"))
	if !strings.Contains(string(res.Content), "attribute count = 3\n\t\t;") {
		t.Fatalf("the layout before the ';' was lost:\n%s", res.Content)
	}
}

func TestSetValueAddsValueToValuelessFeature(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	res := applyOne(t, m, SetValue("Demo::SC::margin", "50.0[SI::kg]"))

	if !strings.Contains(string(res.Content), "attribute margin : ISQ::MassValue = 50.0[SI::kg];") {
		t.Fatalf("value not added:\n%s", res.Content)
	}
	assertOnlySpanChanged(t, m, res)
	if a := res.Applied[0]; a.Span.Len != 0 || a.OldText != "" {
		t.Fatalf("adding a value should be an insertion, got %+v", a)
	}
	requireClean(t, loadContent(t, "spacecraft.sysml", string(res.Content)))
}

func TestSetValueKinds(t *testing.T) {
	cases := []struct {
		name   string
		target string
		value  string
		want   string
	}{
		{"quantity", "Demo::SC::unitMass", "2.5[SI::kg]", "attribute unitMass : ISQ::MassValue default = 2.5[SI::kg];"},
		{"string", "Demo::SC::label", `"flight-2"`, `attribute label : ScalarValues::String = "flight-2";`},
		{"boolean", "Demo::SC::active", "false", "attribute active : ScalarValues::Boolean = false;"},
		{"feature reference", "Demo::SC::total", "unitMass", "attribute total : ISQ::MassValue = unitMass;"},
		{"expression", "Demo::SC::total", "unitMass + margin", "attribute total : ISQ::MassValue = unitMass + margin;"},
		{"nested deeply", "Demo::SC::avionics::board::count", "4", "attribute count : ScalarValues::Integer = 4;"},
		{"through redefines", "Demo::sc::unitMass", "1300.0[SI::kg]", "attribute redefines unitMass = 1300.0[SI::kg];"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := load(t, "spacecraft.sysml")
			requireClean(t, m)
			res := applyOne(t, m, SetValue(tc.target, tc.value))
			if !strings.Contains(string(res.Content), tc.want) {
				t.Fatalf("want %q in:\n%s", tc.want, res.Content)
			}
			assertOnlySpanChanged(t, m, res)
			requireClean(t, loadContent(t, "spacecraft.sysml", string(res.Content)))
		})
	}
}

func TestApplyManyEditsInOnePass(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	res, err := Apply(m, []Operation{
		SetValue("Demo::SC::unitMass", "1050.0[SI::kg]"),
		SetValue("Demo::SC::margin", "10.0[SI::kg]"),
		SetValue("Demo::sc::unitMass", "1400.0[SI::kg]"),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, want := range []string{
		"attribute unitMass : ISQ::MassValue default = 1050.0[SI::kg];",
		"attribute margin : ISQ::MassValue = 10.0[SI::kg];",
		"attribute redefines unitMass = 1400.0[SI::kg];",
	} {
		if !strings.Contains(string(res.Content), want) {
			t.Fatalf("want %q in:\n%s", want, res.Content)
		}
	}
	assertOnlySpanChanged(t, m, res)
	if len(res.Applied) != 3 {
		t.Fatalf("applied %d edits, want 3", len(res.Applied))
	}
	for i, a := range res.Applied {
		if a.OperationIndex != i {
			t.Fatalf("applied[%d] reports operation %d", i, a.OperationIndex)
		}
	}
	requireClean(t, loadContent(t, "spacecraft.sysml", string(res.Content)))
}

// Every operation in a request sees the result of the ones before it: a
// declaration an earlier operation adds or renames is there for a later one to
// name, and one an earlier operation removes is gone.
func TestOrderedOperationsSeeEarlierResults(t *testing.T) {
	cases := []struct {
		name string
		ops  []Operation
		want string
	}{
		{
			"add then rename",
			[]Operation{AddMember("P", "part", "x"), Rename("P::x", "y")},
			"package P {\n    part def Base;\n    part y;\n}\n",
		},
		{
			"add then set value",
			[]Operation{AddMember("P", "attribute", "n"), SetValue("P::n", "3")},
			"package P {\n    part def Base;\n    attribute n = 3;\n}\n",
		},
		{
			"add then delete",
			[]Operation{AddMember("P", "part", "x"), Delete("P::x", false)},
			"package P {\n    part def Base;\n}\n",
		},
		{
			"add, type it, then connect it",
			[]Operation{
				AddMember("P", "part", "a"),
				AddMember("P", "part", "b"),
				AddConnection("P", "connection", "a", "b", "c"),
			},
			"package P {\n    part def Base;\n    part a;\n    part b;\n    connection c connect a to b;\n}\n",
		},
		{
			"rename then add into the new name",
			[]Operation{Rename("P::Base", "Root"), AddMember("P::Root", "part", "x")},
			"package P {\n    part def Root {\n        part x;\n    }\n}\n",
		},
		{
			"rename then rename again",
			[]Operation{Rename("P::Base", "Mid"), Rename("P::Mid", "Last")},
			"package P {\n    part def Last;\n}\n",
		},
		{
			"delete then add a namesake",
			[]Operation{Delete("P::Base", false), AddMember("P", "part def", "Base")},
			"package P {\n    part def Base;\n}\n",
		},
		{
			"add, delete, then reuse the name for another kind",
			[]Operation{
				AddMember("P", "part", "x"),
				Delete("P::x", false),
				AddMember("P", "attribute", "x"),
			},
			"package P {\n    part def Base;\n    attribute x;\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "ordered.sysml", "package P {\n    part def Base;\n}\n")
			res, err := Apply(m, tc.ops)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			got := string(res.Content)
			if got != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
			requireClean(t, loadContent(t, "ordered.sysml", got))
		})
	}
}

// A value set and then deleted with its declaration is one request in order,
// not two edits of the same bytes.
func TestOrderedOperationsMayRewriteTheSameBytes(t *testing.T) {
	m := loadContent(t, "ordered.sysml", "package P {\n    attribute n = 1;\n    attribute k = 2;\n}\n")
	res, err := Apply(m, []Operation{SetValue("P::n", "3"), Delete("P::n", false)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := string(res.Content), "package P {\n    attribute k = 2;\n}\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

// An operation naming what an earlier one removed is refused, not resolved
// against the original text.
func TestOrderedOperationsDoNotSeeRemovedNames(t *testing.T) {
	m := loadContent(t, "ordered.sysml", "package P {\n    part def Base;\n    part def Other;\n}\n")
	_, err := Apply(m, []Operation{
		Rename("P::Base", "Root"),
		Rename("P::Base", "Again"),
	})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || e.OperationIndex != 1 {
		t.Fatalf("got %v, want unknown-target at operation 1", err)
	}
	_, err = Apply(m, []Operation{
		Delete("P::Other", false),
		SetValue("P::Other", "1"),
	})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || e.OperationIndex != 1 {
		t.Fatalf("got %v, want unknown-target at operation 1", err)
	}
}

func TestRenameRewritesTheNameTokenOnly(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	res := applyOne(t, m, Rename("Demo::SC::avionics::board::count", "boardCount"))

	if !strings.Contains(string(res.Content), "attribute boardCount : ScalarValues::Integer = 2;") {
		t.Fatalf("name not rewritten:\n%s", res.Content)
	}
	assertOnlySpanChanged(t, m, res)
	requireClean(t, loadContent(t, "spacecraft.sysml", string(res.Content)))
}

// A referenced rename used to be refused; it now rewrites the references, so
// this pins the rewriting instead of the old refusal.
func TestRenameRewritesReferences(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	res := applyOne(t, m, Rename("Demo::SC::unitMass", "dryMass"))

	got := string(res.Content)
	for _, want := range []string{
		"attribute dryMass : ISQ::MassValue default = 1000.0[SI::kg];",
		"attribute total : ISQ::MassValue = dryMass;",
		"attribute redefines dryMass = 1200.0[SI::kg];",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rename did not produce %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "unitMass") {
		t.Fatalf("old name survives the rename:\n%s", got)
	}
	if len(res.Applied) != 3 {
		t.Fatalf("applied %d edits, want 3 (declaration and two references)", len(res.Applied))
	}
	assertOnlySpanChanged(t, m, res)
	requireClean(t, loadContent(t, "spacecraft.sysml", got))
}

func TestRefusals(t *testing.T) {
	cases := []struct {
		name    string
		ops     []Operation
		failure Failure
		message string
	}{
		{
			name:    "no operations",
			ops:     nil,
			failure: FailureNoOperations,
		},
		{
			name:    "unknown target",
			ops:     []Operation{SetValue("Demo::SC::nothing", "1")},
			failure: FailureUnknownTarget,
			message: "no element named",
		},
		{
			name:    "target outside this source",
			ops:     []Operation{SetValue("ScalarValues::String", "1")},
			failure: FailureUnknownTarget,
			message: "not in this model's source",
		},
		{
			name:    "target carries no value",
			ops:     []Operation{SetValue("Demo::SC", "1")},
			failure: FailureNotValued,
		},
		{
			name:    "value does not parse",
			ops:     []Operation{SetValue("Demo::SC::unitMass", "1050.0[SI::kg")},
			failure: FailureInvalidValue,
		},
		{
			name:    "value is not one expression",
			ops:     []Operation{SetValue("Demo::SC::unitMass", "1050.0 kg")},
			failure: FailureInvalidValue,
		},
		{
			name:    "value is empty",
			ops:     []Operation{SetValue("Demo::SC::unitMass", "   ")},
			failure: FailureInvalidValue,
		},
		{
			name:    "value names nothing",
			ops:     []Operation{SetValue("Demo::SC::total", "missingFeature")},
			failure: FailureResultInvalid,
		},
		{
			name: "overlapping edits",
			ops: []Operation{
				SetValue("Demo::SC::unitMass", "1050.0[SI::kg]"),
				SetValue("Demo::SC::unitMass", "1100.0[SI::kg]"),
			},
			failure: FailureOverlappingEdits,
		},
		{
			name:    "no declared name to rename",
			ops:     []Operation{Rename("Demo::sc::unitMass", "dryMass")},
			failure: FailureNotNamed,
		},
		{
			name:    "new name is not an identifier",
			ops:     []Operation{Rename("Demo::SC::avionics::board::count", "2count")},
			failure: FailureInvalidName,
		},
		{
			name:    "new name is a keyword",
			ops:     []Operation{Rename("Demo::SC::avionics::board::count", "part")},
			failure: FailureInvalidName,
		},
		{
			name:    "new name is empty",
			ops:     []Operation{Rename("Demo::SC::avionics::board::count", "")},
			failure: FailureInvalidName,
		},
		{
			name:    "new name is a sibling's name",
			ops:     []Operation{Rename("Demo::SC::margin", "label")},
			failure: FailureInvalidName,
			message: "already means Demo::SC::label",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := load(t, "spacecraft.sysml")
			requireClean(t, m)
			res, err := Apply(m, tc.ops)
			if res != nil {
				t.Fatalf("refused edit returned content:\n%s", res.Content)
			}
			e := editError(t, err)
			if e.Failure != tc.failure {
				t.Fatalf("failure is %s (%s), want %s", e.Failure, e.Message, tc.failure)
			}
			if tc.message != "" && !strings.Contains(e.Message, tc.message) {
				t.Fatalf("message %q does not mention %q", e.Message, tc.message)
			}
		})
	}
}

func TestRefusedEditLeavesNothingApplied(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	res, err := Apply(m, []Operation{
		SetValue("Demo::SC::unitMass", "1050.0[SI::kg]"),
		SetValue("Demo::SC::nothing", "1"),
	})
	if res != nil {
		t.Fatalf("a refused request returned content:\n%s", res.Content)
	}
	if e := editError(t, err); e.Failure != FailureUnknownTarget {
		t.Fatalf("failure is %s, want unknown-target", e.Failure)
	}
	if !bytes.Contains(m.Source.Bytes(), []byte("1000.0[SI::kg]")) {
		t.Fatal("the model's own source was modified")
	}
}

func TestResultInvalidCarriesDiagnostics(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)

	_, err := Apply(m, []Operation{SetValue("Demo::SC::total", "missingFeature")})
	e := editError(t, err)
	if e.Failure != FailureResultInvalid {
		t.Fatalf("failure is %s, want result-invalid", e.Failure)
	}
	if len(e.Diagnostics) == 0 {
		t.Fatalf("refusal carries no diagnostics: %s", e.Message)
	}
}

// A model that already has errors is still editable: only the errors an edit
// introduces refuse it.
func TestPreExistingErrorsDoNotRefuseAnEdit(t *testing.T) {
	m := loadContent(t, "broken.sysml", "package Demo {\n\tattribute x : ISQ::MassValue = 1.0[SI::kg];\n\tattribute y : Missing::Type;\n}\n")
	if len(m.ParseDiags) > 0 {
		t.Fatalf("fixture does not parse: %v", m.ParseDiags)
	}
	if len(errorsOnly(m.SemDiags)) == 0 {
		t.Fatal("fixture was expected to carry a name-resolution error")
	}

	res := applyOne(t, m, SetValue("Demo::x", "2.0[SI::kg]"))
	if !strings.Contains(string(res.Content), "= 2.0[SI::kg];") {
		t.Fatalf("value not set:\n%s", res.Content)
	}
}

// A syntax error elsewhere in a model does not make its editable parts
// uneditable: the analysis of the edited notation is gated by its own parse, as
// the original's was.
func TestSyntaxErrorElsewhereDoesNotRefuseAnEdit(t *testing.T) {
	m := loadContent(t, "broken.sysml",
		"package Demo {\n\tattribute x : ISQ::MassValue = 1.0[SI::kg];\n"+
			"\tattribute y : Missing::Type;\n\tattribute z = 1 + ;\n}\n")
	if len(m.ParseDiags) == 0 {
		t.Fatal("fixture was expected to carry a syntax error")
	}
	if len(m.SemDiags) > 0 {
		t.Fatalf("a model with parse errors is not analyzed: %v", m.SemDiags)
	}

	res := applyOne(t, m, SetValue("Demo::x", "2.0[SI::kg]"))
	if !strings.Contains(string(res.Content), "= 2.0[SI::kg];") {
		t.Fatalf("value not set:\n%s", res.Content)
	}
	// The edit is still validated: it may not add a syntax error of its own.
	if _, err := Apply(m, []Operation{SetValue("Demo::x", "1 +")}); err == nil {
		t.Fatal("unparsable value was accepted")
	}
}

// An edit that happens to repair a model's only syntax error is not refused for
// the errors the model already had: a parse failure means the original was never
// analyzed, so its baseline is analyzed here rather than assumed clean.
func TestEditRepairingTheOnlySyntaxErrorIsNotRefused(t *testing.T) {
	m := loadContent(t, "broken.sysml",
		"package Demo {\n\tattribute y : Missing::Type;\n\tattribute z = 1 + ;\n}\n")
	if len(m.ParseDiags) == 0 {
		t.Fatal("fixture was expected to carry a syntax error")
	}

	res := applyOne(t, m, SetValue("Demo::z", "2"))
	// The space the fixture wrote before its ';' is not the value's, so it stays.
	if !strings.Contains(string(res.Content), "attribute z = 2 ;") {
		t.Fatalf("value not set:\n%s", res.Content)
	}
	if strings.Contains(string(res.Content), "Missing::Type = ") {
		t.Fatalf("the wrong declaration was edited:\n%s", res.Content)
	}
}

// A rename onto a name the element's own position already resolves to is
// refused: the name would resolve to the renamed element instead, so expressions
// the caller never mentioned would quietly read something else. Re-analysis
// cannot catch it, because the name still resolves.
func TestRenameShadowingAnOuterNameIsRefused(t *testing.T) {
	m := loadContent(t, "shadow.sysml", "package Demo {\n\tattribute x = 1;\n\tpart def P {\n"+
		"\t\tattribute y = 2;\n\t\tattribute z = x;\n\t}\n}\n")
	requireClean(t, m)

	_, err := Apply(m, []Operation{Rename("Demo::P::y", "x")})
	e := editError(t, err)
	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s, want invalid-name", e.Failure)
	}
	if !strings.Contains(e.Message, "already means Demo::x") {
		t.Fatalf("refusal does not name what the new name already means: %s", e.Message)
	}
}

// Members reached through inheritance count, both as a name a rename would take
// over and as a reference a rename would break: the lookups run over a resolver
// with a semantic model attached, which is what makes them visible.
func TestRenameSeesInheritedMembers(t *testing.T) {
	const src = "package Demo {\n\tpart def Base {\n\t\tattribute mass = 1.0;\n\t}\n" +
		"\tpart def P :> Base {\n\t\tattribute m = 2.0;\n\t\tattribute q = mass;\n\t}\n}\n"

	m := loadContent(t, "inherit.sysml", src)
	requireClean(t, m)
	_, err := Apply(m, []Operation{Rename("Demo::P::m", "mass")})
	e := editError(t, err)
	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "already means Demo::Base::mass") {
		t.Fatalf("refusal does not name the inherited feature: %s", e.Message)
	}

	// The inherited feature is referenced by name from P, and the rename rewrites
	// that reference too.
	m = loadContent(t, "inherit.sysml", src)
	res := applyOne(t, m, Rename("Demo::Base::mass", "weight"))
	got := string(res.Content)
	if !strings.Contains(got, "attribute weight = 1.0;") ||
		!strings.Contains(got, "attribute q = weight;") {
		t.Fatalf("inherited reference not rewritten:\n%s", got)
	}
	requireClean(t, loadContent(t, "inherit.sysml", got))
}

func TestSemanticValidationSkippedWithoutIndexSource(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)
	m.NewIndex = nil

	// Without a library index the edited notation can only be checked for
	// syntax, so a name that resolves to nothing is not refused here.
	if _, err := Apply(m, []Operation{SetValue("Demo::SC::total", "missingFeature")}); err != nil {
		t.Fatalf("syntax-only validation refused a parsable value: %v", err)
	}
	// A value that does not parse is still refused.
	if _, err := Apply(m, []Operation{SetValue("Demo::SC::total", "1 +")}); err == nil {
		t.Fatal("unparsable value was accepted")
	}
}
