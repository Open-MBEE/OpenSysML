package view

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestMatrixGoldenForms(t *testing.T) {
	rendering := render(t, "matrix.sysml", "MatrixViews::allRelations")
	if rendering.Kind != KindMatrix || rendering.Stated != "view def GridView" {
		t.Fatalf("matrix = %q (%q), want matrix (view def GridView)", rendering.Kind, rendering.Stated)
	}
	for _, tc := range []struct {
		name string
		form Form
	}{
		{name: "text", form: FormText},
		{name: "markdown", form: FormMarkdown},
		{name: "csv", form: FormCSV},
		{name: "tsv", form: FormTSV},
	} {
		t.Run(tc.name, func(t *testing.T) {
			artifact, err := rendering.Write(tc.form)
			if err != nil {
				t.Fatalf("Write(%s): %v", tc.form, err)
			}
			checkGolden(t, filepath.Join("testdata", "matrix."+tc.name+".golden"), artifact)
		})
	}
	row, column := -1, -1
	for i, cells := range rendering.Rows {
		if cells[0] == "MatrixModel::vehicle" {
			row = i
		}
	}
	for i, name := range rendering.Columns {
		if name == "MatrixModel::r1" {
			column = i
		}
	}
	if row < 0 || column < 1 || rendering.Rows[row][column] != "satisfy, refine, dependency" {
		t.Fatalf("vehicle/r1 cell or axis missing: columns=%v rows=%v", rendering.Columns, rendering.Rows)
	}
}

func TestGridViewShortNameWithRelationshipFilterUsesMatrix(t *testing.T) {
	r, idx := loadFixture(t, "matrix.sysml")
	rendering, err := r.Render(lookup(t, idx, "MatrixViews::shortNameGridView"))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if rendering.Kind != KindMatrix || rendering.Stated != "view def GridView" || len(rendering.Rows) == 0 {
		t.Fatalf("short-name GridView rendering = %#v, want a nonempty matrix", rendering.Data())
	}
}

func TestNonMatrixFilterResolutionDoesNotChangeExposure(t *testing.T) {
	r, idx := loadSources(t, []string{"filter-condition-isolation.sysml"}, [][]byte{[]byte(`import StandardViewDefinitions::*;
package P { part a; }
view v : GridView {
	filter @SysML::SuccessionAsUsage;
	expose P::**;
}`)})
	rendering, err := r.Render(lookup(t, idx, "v"))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if rendering.Kind != KindTable || !rendering.Empty() || len(rendering.Rows) != 0 {
		t.Fatalf("rendering kind=%s, rows=%v, want an empty table", rendering.Kind, rendering.Rows)
	}
	const wantReason = "the view exposes nothing; the rendering is empty"
	for _, form := range []Form{FormText, FormMarkdown, FormCSV, FormTSV} {
		artifact, err := rendering.Write(form)
		if err != nil {
			t.Fatalf("Write(%s): %v", form, err)
		}
		if reason := rendering.EmptyReason(); reason != wantReason {
			t.Errorf("EmptyReason() after Write(%s) = %q, want %q", form, reason, wantReason)
		}
		switch form {
		case FormText, FormMarkdown:
			if !strings.Contains(artifact, wantReason) {
				t.Errorf("%s output omitted empty reason %q:\n%s", form, wantReason, artifact)
			}
		case FormCSV, FormTSV:
			if strings.TrimSpace(artifact) == "" || strings.Contains(strings.TrimSpace(artifact), "\n") {
				t.Errorf("%s output = %q, want only the table header", form, artifact)
			}
		}
	}
}

func TestMatrixContributorsIncludeEveryExposedOwner(t *testing.T) {
	r, idx := loadFixture(t, "matrix.sysml")
	viewSym := lookup(t, idx, "MatrixViews::allRelations")
	verification := lookup(t, idx, "MatrixModel::vehicleVerification")
	var target *symbols.Symbol
	visited := map[*symbols.Scope]bool{}
	var findVerification func(*symbols.Scope)
	findVerification = func(scope *symbols.Scope) {
		if scope == nil || visited[scope] || target != nil {
			return
		}
		visited[scope] = true
		for _, member := range scope.AllMembers() {
			if keyword, edges := r.matrixEdges(member); member.Name == "r1" && keyword == "verify" && len(edges) != 0 {
				target = member
				return
			}
			findVerification(member.Scope)
		}
	}
	findVerification(verification.Scope)
	if target == nil {
		t.Fatal("vehicleVerification has no nested verify r1 usage")
	}

	exposed, err := r.model.ExposedMembers(viewSym)
	if err != nil {
		t.Fatalf("ExposedMembers: %v", err)
	}
	if !slices.ContainsFunc(exposed, func(elem *symbols.Symbol) bool {
		return symbols.SameElement(elem, target)
	}) {
		t.Fatal("verification usage is not exposed")
	}
	if keyword, edges := r.matrixEdges(target); keyword != "verify" || len(edges) == 0 {
		t.Fatalf("verification usage edges = %q, %v; want a drawn verify edge", keyword, edges)
	}

	rendering, err := r.Render(viewSym)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if name := r.notationName(target); strings.Contains(strings.Join(rendering.Notices, "\n"), name) {
		t.Fatalf("exposed verification usage with a drawn edge was left out: %s", name)
	}
}

func TestNamedExposedElementsIncludeImportedDerivationMembers(t *testing.T) {
	r, idx := loadFixture(t, "matrix.sysml")
	viewSym := lookup(t, idx, "MatrixViews::allRelations")
	exposed, err := r.model.ExposedElements(viewSym)
	if err != nil {
		t.Fatalf("ExposedElements: %v", err)
	}
	for _, elem := range exposed {
		if strings.HasSuffix(idx.GetFQN(elem), "DerivationConnections::derivations") {
			return
		}
	}
	t.Fatal("named ExposedElements path omitted DerivationConnections::derivations")
}

func TestGridViewMatrixSelectorsAndEmptyCases(t *testing.T) {
	r, idx := loadFixture(t, "matrix.sysml")
	cases := []struct {
		name string
		kind Kind
	}{
		{name: "MatrixViews::allocationMatrix", kind: KindMatrix},
		{name: "MatrixViews::inheritedAllocationMatrix", kind: KindMatrix},
		{name: "MatrixViews::directExposeFilterMatrix", kind: KindMatrix},
		{name: "MatrixViews::interfaceMatrix", kind: KindMatrix},
		{name: "MatrixViews::verificationDefinitionMatrix", kind: KindMatrix},
		{name: "MatrixViews::specializedRefinementMatrix", kind: KindMatrix},
		{name: "MatrixViews::specializedDerivationMatrix", kind: KindMatrix},
		{name: "MatrixViews::emptyMatrix", kind: KindMatrix},
		{name: "MatrixViews::nothingExposedMatrix", kind: KindMatrix},
		{name: "MatrixViews::unknownSelectorTable", kind: KindTable},
		{name: "MatrixViews::mixedSelectorMatrix", kind: KindMatrix},
		{name: "MatrixViews::negatedSelectorTable", kind: KindTable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendering, err := r.Render(lookup(t, idx, tc.name))
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if rendering.Kind != tc.kind {
				t.Fatalf("kind = %q, want %q", rendering.Kind, tc.kind)
			}
			switch tc.name {
			case "MatrixViews::allocationMatrix", "MatrixViews::inheritedAllocationMatrix":
				sourcePresent := slices.ContainsFunc(rendering.Rows, func(row []string) bool {
					return len(row) != 0 && row[0] == "MatrixModel::logical::ctrl"
				})
				if !sourcePresent ||
					!slices.Contains(rendering.Columns, "MatrixModel::vehicle::engine") ||
					!slices.Contains(rendering.Columns, "MatrixModel::vehicle") {
					t.Fatalf("allocation axes missing: columns=%v rows=%v", rendering.Columns, rendering.Rows)
				}
			case "MatrixViews::interfaceMatrix", "MatrixViews::verificationDefinitionMatrix",
				"MatrixViews::specializedRefinementMatrix", "MatrixViews::specializedDerivationMatrix":
				if len(rendering.Rows) == 0 {
					t.Fatalf("matrix has no rows: %v", rendering)
				}
			case "MatrixViews::emptyMatrix", "MatrixViews::mixedSelectorMatrix":
				if !rendering.Empty() || len(rendering.Columns) != 0 || len(rendering.Rows) != 0 || len(rendering.Notices) != 1 {
					t.Fatalf("empty matrix state = columns %v, rows %v, notices %v", rendering.Columns, rendering.Rows, rendering.Notices)
				}
				want := "2 exposed elements state no allocate relationship and are left out of the matrix: EmptyModel::unrelated, EmptyModel::secondUnrelated"
				if rendering.Notices[0] != want {
					t.Fatalf("empty matrix notice = %q, want %q", rendering.Notices[0], want)
				}
			case "MatrixViews::nothingExposedMatrix":
				if !rendering.Empty() || len(rendering.Notices) != 0 || rendering.EmptyReason() != "the view exposes nothing; the rendering is empty" {
					t.Fatalf("nothing-exposed state = %q, notices %v", rendering.EmptyReason(), rendering.Notices)
				}
			case "MatrixViews::unknownSelectorTable":
				if !slices.Equal(rendering.Columns, tableColumns) {
					t.Fatalf("unknown selector columns = %v, want table columns", rendering.Columns)
				}
			}
		})
	}
}

func TestExplicitTablePrecedesMatrixSelector(t *testing.T) {
	r, idx := loadSources(t, []string{"explicit-table.sysml"}, [][]byte{[]byte(`private import StandardViewDefinitions::*;
private import Views::asElementTable;
package ExplicitTableMatrix {
	part source;
	part target;
	allocation arc allocate source to target;
	view explicitTable : GridView {
		render asElementTable;
		filter @SysML::AllocationUsage;
		expose ExplicitTableMatrix::**;
	}
}`)})
	rendering, err := r.Render(lookup(t, idx, "ExplicitTableMatrix::explicitTable"))
	if err != nil {
		t.Fatal(err)
	}
	if rendering.Kind != KindTable || rendering.Stated != "render asElementTable" {
		t.Fatalf("explicit render kind = %q (%q), want table", rendering.Kind, rendering.Stated)
	}
}

func TestGridViewMatrixSelectorKeywords(t *testing.T) {
	r, idx := loadFixture(t, "matrix.sysml")
	cases := []struct {
		view string
		want []string
	}{
		{"MatrixViews::satisfySelectorMatrix", []string{"satisfy"}},
		{"MatrixViews::verificationUsageMatrix", []string{"verify"}},
		{"MatrixViews::verificationDefinitionMatrix", []string{"verify"}},
		{"MatrixViews::allocationMatrix", []string{"allocate"}},
		{"MatrixViews::connectionSelectorMatrix", []string{"allocate", "connect", "derive"}},
		{"MatrixViews::interfaceMatrix", []string{"connect"}},
		{"MatrixViews::dependencySelectorMatrix", []string{"refine", "dependency"}},
		{"MatrixViews::refinementMetadataMatrix", []string{"refine"}},
		{"MatrixViews::specializedRefinementMatrix", []string{"refine"}},
		{"MatrixViews::derivationMetadataMatrix", []string{"derive"}},
		{"MatrixViews::specializedDerivationMatrix", []string{"derive"}},
	}
	for _, tc := range cases {
		t.Run(tc.view, func(t *testing.T) {
			got := r.matrixShownKinds(lookup(t, idx, tc.view))
			if !slices.Equal(got, tc.want) {
				t.Errorf("matrixShownKinds = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMatrixRowsRetainSourceOrigins(t *testing.T) {
	rendering := render(t, "matrix.sysml", "MatrixViews::allRelations")
	if len(rendering.Rows) == 0 {
		t.Fatal("matrix has no rows")
	}
	if len(rendering.Rows) != len(rendering.RowOrigins) {
		t.Fatalf("%d rows but %d origins", len(rendering.Rows), len(rendering.RowOrigins))
	}
	sf := fixtureText(t, "matrix.sysml")
	for i, origin := range rendering.RowOrigins {
		if !origin.Located() || sf.Text(origin.Span) == "" {
			t.Errorf("row %d (%s) has no source origin: %+v", i, rendering.Rows[i][0], origin)
		}
	}
}

func TestGridViewMatrixRefusesGraphForms(t *testing.T) {
	rendering := render(t, "matrix.sysml", "MatrixViews::allRelations")
	for _, form := range []Form{FormMermaid, FormDot, FormPlantUML} {
		_, err := rendering.Write(form)
		var wrong *WrongFormError
		if !errors.As(err, &wrong) || wrong.Kind != KindMatrix || wrong.Form != form ||
			!strings.Contains(err.Error(), "matrix rendering") {
			t.Errorf("Write(%s) error = %v, want matrix WrongFormError", form, err)
		}
	}
	if got := KindMatrix.SupportedForms(); strings.Join(formNames(got), ",") != "text,markdown,csv,tsv" {
		t.Errorf("matrix forms = %v", got)
	}
}

func TestMatrixPseudoViewRendersExposedRelationships(t *testing.T) {
	r, idx := loadFixture(t, "matrix.sysml")
	kind, target, ok := ParsePseudoView("#matrix:MatrixModel")
	if !ok || kind != KindMatrix || target != "MatrixModel" {
		t.Fatalf("ParsePseudoView = %q, %q, %t", kind, target, ok)
	}
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, target)}, kind, "#matrix")
	if err != nil {
		t.Fatalf("RenderExposed: %v", err)
	}
	if rendering.Kind != KindMatrix || len(rendering.Rows) == 0 || len(rendering.Notices) != 0 {
		t.Fatalf("#matrix rendering = %#v", rendering.Data())
	}
	if !slices.Contains(PseudoViewSpecs(), "#matrix") {
		t.Fatalf("pseudo view specs = %v", PseudoViewSpecs())
	}
}

func formNames(forms []Form) []string {
	names := make([]string, len(forms))
	for i, form := range forms {
		names[i] = string(form)
	}
	return names
}
