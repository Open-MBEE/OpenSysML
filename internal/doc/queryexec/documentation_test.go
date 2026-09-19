package queryexec

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// documentedBody declares requirements with short names and doc bodies the way
// a specification model does, plus one with neither and one with two bodies.
const documentedBody = `
requirement def <'HLR-R001'> CrewSafety {
	doc /* The mission shall safely return
	     * all three crew members to Earth. */
}
requirement def <'HLR-R002'> SoftLanding {
	doc /* The mission shall achieve a soft landing on the lunar surface. */
}
requirement def Unlabeled;
requirement def <'HLR-R004'> TwoBodies {
	doc Summary /* Short form. */
	doc Detail /* Long form. */
}
part spec {
	requirement r1 : CrewSafety;
	requirement r2 : SoftLanding;
	requirement r3 : Unlabeled;
	requirement r4 : TwoBodies;
}
`

func documentedRows(t *testing.T, query string) *RowSet {
	t.Helper()
	fixture := loadExecutionFixture(t, documentedBody+query)
	result, err := fixture.execute(t, "Q", Bindings{
		"root": {ElementValue(fixture.symbol(t, "spec"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return result
}

// cellStrings reads every string value of the named column, one slice per row.
func cellStrings(t *testing.T, result *RowSet, column string) [][]string {
	t.Helper()
	position := -1
	for i, c := range result.Columns() {
		if c.Name() == column {
			position = i
		}
	}
	if position < 0 {
		t.Fatalf("no column %s in %v", column, result.Columns())
	}
	var out [][]string
	for _, row := range result.Rows() {
		values := row.Cells()[position].Values()
		strs := make([]string, 0, len(values))
		for _, value := range values {
			s, ok := value.String()
			if !ok {
				t.Fatalf("%s cell = %+v, want strings", column, values)
			}
			strs = append(strs, s)
		}
		out = append(out, strs)
	}
	return out
}

const requirementDefinitions = `
RelatedElements(
	source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "RequirementUsage"),
	relationshipKind = "typing", direction = "outgoing", maxDepth = 1
)`

func TestExecuteProjectsShortNameAndDocumentation(t *testing.T) {
	result := documentedRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = `+requirementDefinitions+`,
		properties = ("shortName", "declaredShortName", "name", "documentation")
	)
}`)
	if got := cellStrings(t, result, "shortName"); !slices.EqualFunc(got,
		[][]string{{"HLR-R001"}, {"HLR-R002"}, {}, {"HLR-R004"}}, slices.Equal) {
		t.Fatalf("shortName cells = %q", got)
	}
	if got := cellStrings(t, result, "declaredShortName"); !slices.EqualFunc(got,
		[][]string{{"HLR-R001"}, {"HLR-R002"}, {}, {"HLR-R004"}}, slices.Equal) {
		t.Fatalf("declaredShortName cells = %q", got)
	}
	want := [][]string{
		{"The mission shall safely return\nall three crew members to Earth."},
		{"The mission shall achieve a soft landing on the lunar surface."},
		{},
		{"Short form.", "Long form."},
	}
	if got := cellStrings(t, result, "documentation"); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("documentation cells = %q, want %q", got, want)
	}
}

func TestExecuteOrdersByShortName(t *testing.T) {
	result := documentedRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = OrderBy(
			source = `+requirementDefinitions+`,
			property = "shortName", direction = "descending",
			missing = "last", multiple = "error"
		),
		properties = ("name")
	)
}`)
	if got := projectedNames(t, result); !slices.Equal(got, []string{"TwoBodies", "SoftLanding", "CrewSafety", "Unlabeled"}) {
		t.Fatalf("ordered names = %v", got)
	}
}

func TestExecuteFiltersOnShortNameAndDocumentation(t *testing.T) {
	byShortName := documentedRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = `+requirementDefinitions+`,
			feature = "shortName", operator = "startsWith", value = "HLR-R00"
		),
		properties = ("name")
	)
}`)
	if got := projectedNames(t, byShortName); !slices.Equal(got, []string{"CrewSafety", "SoftLanding", "TwoBodies"}) {
		t.Fatalf("filtered by short name = %v", got)
	}

	byDocumentation := documentedRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = `+requirementDefinitions+`,
			feature = "documentation", operator = "contains", value = "lunar"
		),
		properties = ("name")
	)
}`)
	if got := projectedNames(t, byDocumentation); !slices.Equal(got, []string{"SoftLanding"}) {
		t.Fatalf("filtered by documentation = %v", got)
	}

	anyBody := documentedRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = `+requirementDefinitions+`,
			feature = "documentation", operator = "=", value = "Long form."
		),
		properties = ("name")
	)
}`)
	if got := projectedNames(t, anyBody); !slices.Equal(got, []string{"TwoBodies"}) {
		t.Fatalf("filtered by a later body = %v", got)
	}
}

func TestExecuteComputesElementShortNameAndDocumentation(t *testing.T) {
	result := documentedRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = `+requirementDefinitions+`,
			feature = "name", operator = "!=", value = "TwoBodies"
		),
		properties = ("name"),
		columns = (
			Column(name = "label", expression = (Element::shortName ?? "-") + ": " + Element::name),
			Column(name = "declared", expression = Element::declaredShortName ?? "none"),
			Column(name = "text", expression = Element::documentation ?? "undocumented")
		)
	)
}`)
	if got := cellStrings(t, result, "label"); !slices.EqualFunc(got,
		[][]string{{"HLR-R001: CrewSafety"}, {"HLR-R002: SoftLanding"}, {"-: Unlabeled"}}, slices.Equal) {
		t.Fatalf("label cells = %q", got)
	}
	if got := cellStrings(t, result, "declared"); !slices.EqualFunc(got,
		[][]string{{"HLR-R001"}, {"HLR-R002"}, {"none"}}, slices.Equal) {
		t.Fatalf("declared cells = %q", got)
	}
	want := [][]string{
		{"The mission shall safely return\nall three crew members to Earth."},
		{"The mission shall achieve a soft landing on the lunar surface."},
		{"undocumented"},
	}
	if got := cellStrings(t, result, "text"); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("text cells = %q, want %q", got, want)
	}
}

// A `perform` usage declares no short name of its own: it carries the one of
// the action it reference-subsets, everywhere the property is read.
func TestExecuteShortNameFollowsAReferenceSubsetting(t *testing.T) {
	const performing = `
action def Provide;
action <pp> providePower : Provide;
action <ss> steer : Provide;
part spec {
	perform providePower;
	perform action turn references steer;
}
`
	fixture := loadExecutionFixture(t, performing+`
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "ActionUsage"),
			feature = "shortName", operator = "startsWith", value = "p"
		),
		properties = ("shortName", "name", "declaredShortName"),
		columns = (Column(name = "label", expression = Element::shortName + "/" + Element::name))
	)
}`)
	result, err := fixture.execute(t, "Q", Bindings{
		"root": {ElementValue(fixture.symbol(t, "spec"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := cellStrings(t, result, "shortName"); !slices.EqualFunc(got, [][]string{{"pp"}}, slices.Equal) {
		t.Fatalf("shortName cells = %q, want pp", got)
	}
	if got := cellStrings(t, result, "name"); !slices.EqualFunc(got, [][]string{{"providePower"}}, slices.Equal) {
		t.Fatalf("name cells = %q", got)
	}
	if got := cellStrings(t, result, "declaredShortName"); !slices.EqualFunc(got, [][]string{nil}, slices.Equal) {
		t.Fatalf("declaredShortName cells = %q, want absent", got)
	}
	if got := cellStrings(t, result, "label"); !slices.EqualFunc(got, [][]string{{"pp/providePower"}}, slices.Equal) {
		t.Fatalf("label cells = %q", got)
	}
}

func TestExecuteShortNameIsNotDerivedForANamedFeature(t *testing.T) {
	fixture := loadExecutionFixture(t, `
action def Provide;
action <pp> providePower : Provide;
action <ss> steer : Provide;
part spec {
	perform providePower;
	perform action turn references steer;
}
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "ActionUsage"),
		properties = ("name", "shortName")
	)
}`)
	result, err := fixture.execute(t, "Q", Bindings{
		"root": {ElementValue(fixture.symbol(t, "spec"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := cellStrings(t, result, "name"); !slices.EqualFunc(got, [][]string{{"providePower"}, {"turn"}}, slices.Equal) {
		t.Fatalf("name cells = %q", got)
	}
	if got := cellStrings(t, result, "shortName"); !slices.EqualFunc(got, [][]string{{"pp"}, nil}, slices.Equal) {
		t.Fatalf("shortName cells = %q, want pp for the unnamed perform and none for turn", got)
	}
}

// A computed column is one value per row, so an element with two doc bodies
// fails the column the way every multi-valued feature does.
func TestExecuteComputedDocumentationReportsSeveralBodies(t *testing.T) {
	fixture := loadExecutionFixture(t, documentedBody+`
calc def Q :> Query {
	in root : Element;
	Project(
		source = `+requirementDefinitions+`,
		properties = ("name"),
		columns = (Column(name = "text", expression = Element::documentation ?? "undocumented"))
	)
}`)
	_, err := fixture.execute(t, "Q", Bindings{
		"root": {ElementValue(fixture.symbol(t, "spec"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorColumnCardinality {
		t.Fatalf("error = %v, want %v", err, ErrorColumnCardinality)
	}
	if executionError.Property != "text" || !strings.HasSuffix(executionError.Target, "TwoBodies") {
		t.Fatalf("error = %+v, want the text column of TwoBodies", executionError)
	}
}
