package metaclassmap

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/census/grammar"
)

func TestAnalyzeDifferentialBuckets(t *testing.T) {
	const doc = "tests/parser/testdata/parse/map.sysml"
	const relationshipDoc = "tests/parser/testdata/parse/relationships.kerml"
	content := []byte("package Demo {\npart def A;\npart def B;\nattribute def C;\nimport Demo::*;\n}\n")
	relationshipContent := []byte("package P {\nclassifier A;\nclassifier B;\nspecialization Gen subtype A specializes B;\n}\n")
	shapes := []grammar.Shape{
		{Grammar: "Toy.xtext", Name: "Part", Kind: grammar.KindRule, Returns: "SysML::PartDefinition",
			Creates: []string{"SysML::PartDefinition"}, Anchors: []string{"part"}},
		{Grammar: "Toy.xtext", Name: "RelationshipNotReified", Kind: grammar.KindRule,
			Creates: []string{"SysML::OwningMembership"}, Anchors: []string{"part"}},
		{Grammar: "Toy.xtext", Name: "DifferentRelationship", Kind: grammar.KindRule,
			Creates: []string{"KerML::Subclassification"}, Anchors: []string{"specialization"}},
		{Grammar: "Toy.xtext", Name: "Shared", Kind: grammar.KindRule, Returns: "SysML::Shared",
			Creates: []string{"SysML::ActionDefinition"}, Anchors: []string{"part"}},
		{Grammar: "Toy.xtext", Name: "Disagree", Kind: grammar.KindRule, Returns: "SysML::Disagree",
			Creates: []string{"SysML::ActionDefinition"}, Anchors: []string{"attribute"}},
		{Grammar: "Toy.xtext", Name: "NoElementAtAnchor", Kind: grammar.KindRule, Returns: "SysML::NoElementAtAnchor",
			Creates: []string{"SysML::PartDefinition"}, Anchors: []string{"import"}},
		{Grammar: "Toy.xtext", Name: "NoAnchor", Kind: grammar.KindRule, Returns: "SysML::NoAnchor",
			Creates: []string{"SysML::PartDefinition"}, Anchors: []string{"absent"}},
		{Grammar: "Toy.xtext", Name: "NoInput", Kind: grammar.KindRule, Returns: "SysML::NoInput",
			Creates: []string{"SysML::PartDefinition"}, Anchors: []string{"part"}},
		{Grammar: "Toy.xtext", Name: "NoElement", Kind: grammar.KindFragment, Returns: "SysML::NoElement",
			Anchors: []string{"part"}},
	}
	rows := []grammar.Row{
		evidenceRow("Part", "part", 2, doc),
		evidenceRow("RelationshipNotReified", "part", 2, doc),
		evidenceRow("DifferentRelationship", "specialization", 4, relationshipDoc),
		evidenceRow("Shared", "part", 2, doc),
		evidenceRow("Disagree", "attribute", 4, doc),
		evidenceRow("NoElementAtAnchor", "import", 5, doc),
		evidenceRow("NoAnchor", "part", 2, doc),
		{Grammar: "Toy.xtext", Name: "NoInput", Kind: grammar.KindRule, Bucket: grammar.BucketNoEvidence},
		evidenceRow("NoElement", "part", 2, doc),
	}
	coverage := &grammar.Report{
		PilotTag: "fixture",
		Grammars: []grammar.GrammarReport{{Name: "Toy.xtext", Productions: rows}},
	}
	report, err := Analyze(coverage, shapes, map[string][]byte{
		doc: content, relationshipDoc: relationshipContent,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ bucket, reason string }{
		"Part":                   {"agree", ""},
		"RelationshipNotReified": {"undecided", "relationship-not-reified"},
		"DifferentRelationship":  {"disagree", ""},
		"Shared":                 {"undecided", "anchor-shared"},
		"Disagree":               {"disagree", ""},
		"NoElementAtAnchor":      {"undecided", "no-element-at-anchor"},
		"NoAnchor":               {"undecided", "no-anchor"},
		"NoInput":                {"undecided", "no-input"},
		"NoElement":              {"undecided", "no-element"},
	}
	for _, production := range report.Grammars[0].Productions {
		expect := want[production.Shape.Name]
		if production.Differential.Bucket != expect.bucket || production.Differential.Reason != expect.reason {
			t.Errorf("%s differential = %+v, want %s/%s", production.Shape.Name,
				production.Differential, expect.bucket, expect.reason)
		}
	}
}

func TestAnalyzeReflectiveChecks(t *testing.T) {
	const doc = "tests/parser/testdata/parse/reflect.sysml"
	content := []byte("package Demo {\npart def A;\n}\n")
	shape := grammar.Shape{
		Grammar: "Toy.xtext", Name: "Part", Kind: grammar.KindRule, Line: 7,
		Returns: "SysML::NotAType", ReturnsDefaulted: true,
		Actions: []grammar.Action{{Metaclass: "SysML::AlsoMissing"}},
		Assignments: []grammar.Assignment{{
			Feature: "ownedRelationship", Op: "+=", Value: "Name", Owners: []string{"SysML::PartDefinition"},
		}, {
			Feature: "notAFeature", Op: "=", Value: "'x'", Owners: []string{"SysML::PartDefinition"},
		}, {
			Feature: "feature", Op: "=", Value: "'x'", Owners: []string{"SysML::UnknownType"},
		}},
	}
	coverage := &grammar.Report{
		PilotTag: "fixture",
		Grammars: []grammar.GrammarReport{{Name: "Toy.xtext", Productions: []grammar.Row{
			evidenceRow("Part", "part", 2, doc),
		}}},
	}
	report, err := Analyze(coverage, []grammar.Shape{shape}, map[string][]byte{doc: content})
	if err != nil {
		t.Fatal(err)
	}
	production := report.Grammars[0].Productions[0]
	missingTypes := 0
	for _, check := range production.Types {
		if check.Status == "missing" {
			missingTypes++
		}
	}
	if missingTypes != 3 {
		t.Errorf("missing types = %d, want missing return, action, and assignment owner", missingTypes)
	}
	if len(production.Assignments) != 3 ||
		production.Assignments[0].Status != "found" ||
		production.Assignments[0].DeclaredBy != "KerML::Root::Element" ||
		production.Assignments[1].Status != "missing" ||
		production.Assignments[2].Status != "owner-missing" {
		t.Errorf("assignment checks = %+v", production.Assignments)
	}
	if report.Totals.Assignments != 3 || report.Totals.AssignmentsFound != 1 ||
		report.Totals.AssignmentsMissing != 1 || report.Totals.OwnerMissing != 1 ||
		report.Grammars[0].Totals.OwnerMissing != 1 || report.Totals.TypesByStatus["missing"] != 3 {
		t.Errorf("totals = %+v", report.Totals)
	}
	summary := report.Summary()
	if len(summary.MissingTypes) != 3 ||
		summary.MissingTypes[0].Name != "SysML::NotAType" ||
		len(summary.OwnerMissingAssignments) != 1 ||
		summary.OwnerMissingAssignments[0] != (FeatureFinding{
			Grammar: "Toy.xtext", Production: "Part", Line: 7,
			Feature: "feature", Owner: "SysML::UnknownType",
		}) {
		t.Errorf("summary findings = %+v", summary)
	}
	if !bytes.Contains([]byte(report.Text()), []byte("Assignments: 3 total, 1 found, 1 missing, 1 owner-missing")) {
		t.Errorf("text summary omits owner-missing count:\n%s", report.Text())
	}
	if !bytes.Contains([]byte(report.Markdown()), []byte("owner missing")) {
		t.Errorf("Markdown table omits owner-missing assignment:\n%s", report.Markdown())
	}
}

func TestAnalyzeEnumLiterals(t *testing.T) {
	const doc = "tests/parser/testdata/parse/enums.sysml"
	coverage := &grammar.Report{
		PilotTag: "fixture",
		Grammars: []grammar.GrammarReport{{Name: "Toy.xtext", Productions: []grammar.Row{
			{Grammar: "Toy.xtext", Name: "VisibilityKind", Kind: grammar.KindEnum, Bucket: grammar.BucketNoEvidence},
		}}},
	}
	shape := grammar.Shape{
		Grammar: "Toy.xtext", Name: "VisibilityKind", Kind: grammar.KindEnum, Returns: "KerML::VisibilityKind",
		Assignments: []grammar.Assignment{
			{Feature: "private", Op: "=", Value: "'['", Owners: []string{"KerML::VisibilityKind"}},
			{Feature: "bogus", Op: "=", Value: "'private'", Owners: []string{"KerML::VisibilityKind"}},
		},
	}
	report, err := Analyze(coverage, []grammar.Shape{shape}, map[string][]byte{doc: []byte("package Empty {}\n")})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Grammars[0].Productions[0].Assignments
	if len(got) != 2 || got[0].Status != "found" || got[1].Status != "missing" ||
		report.Totals.EnumLiteralsFound != 1 || report.Totals.EnumLiteralsMissing != 1 {
		t.Errorf("enum checks = %+v, totals %+v", got, report.Totals)
	}
}

func TestReportWritesAreDeterministic(t *testing.T) {
	report := &Report{
		PilotTag: "fixture",
		Totals:   newTotals(),
		Grammars: []GrammarReport{{Name: "A.xtext", Totals: newTotals()}},
	}
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	if err := writeReports(first, report, nil); err != nil {
		t.Fatal(err)
	}
	if err := writeReports(second, report, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"grammar-metaclass-map.json", "grammar-metaclass-map-tables.md", "grammar-metaclass-map.txt",
	} {
		left, err := os.ReadFile(filepath.Join(first, name))
		if err != nil {
			t.Fatal(err)
		}
		right, err := os.ReadFile(filepath.Join(second, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(left, right) {
			t.Errorf("%s differs across report writes", name)
		}
	}
}

func evidenceRow(name, literal string, line int, file string) grammar.Row {
	return grammar.Row{
		Grammar: "Toy.xtext", Name: name, Kind: grammar.KindRule,
		Bucket: grammar.BucketEvidence, Required: []string{literal},
		Evidence: []grammar.Citation{{Literal: literal, File: file, Line: line}},
	}
}
