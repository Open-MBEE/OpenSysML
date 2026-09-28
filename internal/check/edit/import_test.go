package edit

import (
	"strings"
	"testing"
)

func TestAddImportForms(t *testing.T) {
	cases := []struct {
		name       string
		visibility string
		target     string
		recursive  bool
		all        bool
		filters    []string
		want       string
	}{
		{"namespace default", "", "ScalarValues::*", false, false, nil, "private import ScalarValues::*;"},
		{"membership public", "public", "A::B", false, false, nil, "public import A::B;"},
		{"membership recursive", "protected", "A", true, false, nil, "protected import A::**;"},
		{"nested membership recursive", "private", "A::B", true, false, nil, "private import A::B::**;"},
		{"namespace recursive", "private", "A::*", true, false, nil, "private import A::*::**;"},
		{"all", "private", "A::*", false, true, nil, "private import all A::*;"},
		{"one filter", "private", "A::*", false, false, []string{"@A::Safety"}, "private import A::*[@A::Safety];"},
		{"two filters", "private", "A::*", false, false, []string{"@A::Safety", "@A::Approved"}, "private import A::*[@A::Safety][@A::Approved];"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := loadContent(t, "import.sysml",
				"package A {\n    part def B;\n    metadata def Safety;\n    metadata def Approved;\n}\npackage P {\n}\n")
			requireClean(t, model)
			result := applyOne(t, model, AddImport("P", tc.visibility, tc.target, tc.recursive, tc.all, tc.filters))
			if !strings.Contains(string(result.Content), tc.want) {
				t.Fatalf("want %q in:\n%s", tc.want, result.Content)
			}
			requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
		})
	}
}

// Each form that resolves must write exactly the notation and re-load clean.
func TestAddImportResolvingForms(t *testing.T) {
	model := loadContent(t, "import.sysml", "package P {\n}\n")
	requireClean(t, model)
	result, err := Apply(model, []Operation{
		AddImport("P", "", "ScalarValues::*", false, false, nil),
		AddImport("P", "public", "ScalarValues::Real", false, false, nil),
		AddImport("P", "protected", "ScalarValues", true, false, nil),
		AddImport("P", "private", "SI::*", true, false, nil),
		AddImport("P", "private", "ISQ::*", false, true, nil),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "package P {\n" +
		"    private import ScalarValues::*;\n" +
		"    public import ScalarValues::Real;\n" +
		"    protected import ScalarValues::**;\n" +
		"    private import SI::*::**;\n" +
		"    private import all ISQ::*;\n" +
		"}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportKerML(t *testing.T) {
	model := loadContent(t, "import.kerml", "package A {\n    class B;\n}\nclassifier C {\n}\n")
	requireClean(t, model)
	result, err := Apply(model, []Operation{
		AddImport("", "private", "A::*", false, false, nil),
		AddImport("C", "private", "A::B", false, false, nil),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "private import A::*;\npackage A {\n    class B;\n}\nclassifier C {\n    private import A::B;\n}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.kerml", string(result.Content)))
}

func TestAddImportFilterOnMetadata(t *testing.T) {
	model := loadContent(t, "import.sysml",
		"package Q {\n    metadata def M;\n}\npackage P {\n    part a;\n}\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("P", "private", "Q::*", false, false, []string{"@Q::M"}))
	if !strings.Contains(string(result.Content), "private import Q::*[@Q::M];") {
		t.Fatalf("filtered import missing:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportFilterOnKerMLMetaclass(t *testing.T) {
	model := loadContent(t, "import.kerml",
		"package Q {\n    metaclass M;\n}\npackage P {\n    feature a;\n}\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("P", "private", "Q::*", false, false, []string{"@Q::M"}))
	if !strings.Contains(string(result.Content), "private import Q::*[@Q::M];") {
		t.Fatalf("filtered import missing:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "import.kerml", string(result.Content)))
}

func TestAddImportAfterLastImport(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  string
	}{
		{
			"4-space indent",
			"package P {\n    private import ScalarValues::*;\n    part a;\n}\n",
			"package P {\n    private import ScalarValues::*;\n    private import SI::*;\n    part a;\n}\n",
		},
		{
			"tab indent",
			"package P {\n\tprivate import ScalarValues::*;\n\tpart a;\n}\n",
			"package P {\n\tprivate import ScalarValues::*;\n\tprivate import SI::*;\n\tpart a;\n}\n",
		},
		{
			"import with a trailing comment",
			"package P {\n    private import ScalarValues::*; // library\n    part a;\n}\n",
			"package P {\n    private import ScalarValues::*; // library\n    private import SI::*;\n    part a;\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := loadContent(t, "import.sysml", tc.model)
			requireClean(t, model)
			result := applyOne(t, model, AddImport("P", "", "SI::*", false, false, nil))
			if got := string(result.Content); got != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
			requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
		})
	}
}

func TestAddImportInlineAfterImportSharingLine(t *testing.T) {
	model := loadContent(t, "import.sysml",
		"package P { private import ScalarValues::*; part a; }\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("P", "", "SI::*", false, false, nil))
	if !strings.Contains(string(result.Content),
		"private import ScalarValues::*; private import SI::*;") {
		t.Fatalf("import not inline after the last import:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportBeforeFirstMember(t *testing.T) {
	model := loadContent(t, "import.sysml",
		"package P {\n    // the parts\n    part a;\n}\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("P", "", "ScalarValues::*", false, false, nil))
	want := "package P {\n    private import ScalarValues::*;\n    // the parts\n    part a;\n}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportIntoSingleLineBody(t *testing.T) {
	model := loadContent(t, "import.sysml", "package P { part a; }\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("P", "", "ScalarValues::*", false, false, nil))
	if !strings.Contains(string(result.Content),
		"package P { private import ScalarValues::*; part a; }") {
		t.Fatalf("import not inline in a single-line body:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportIntoEmptyBody(t *testing.T) {
	model := loadContent(t, "import.sysml", "package P { }\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("P", "", "ScalarValues::*", false, false, nil))
	want := "package P {\n    private import ScalarValues::*;\n}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportsThenMemberFromSingleLineBody(t *testing.T) {
	model := loadContent(t, "import.sysml", "package ToasterDemo { }\n")
	requireClean(t, model)
	result, err := Apply(model, []Operation{
		AddImport("ToasterDemo", "", "ScalarValues::*", false, false, nil),
		AddImport("ToasterDemo", "", "SI::*", false, false, nil),
		AddImport("ToasterDemo", "", "ISQ::*", false, false, nil),
		AddImport("ToasterDemo", "", "MeasurementReferences::*", false, false, nil),
		{Kind: OpAddMember, Owner: "ToasterDemo", MemberKind: "attribute", MemberName: "efficiency", Type: "DimensionOneValue"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "package ToasterDemo {\n" +
		"    private import ScalarValues::*;\n" +
		"    private import SI::*;\n" +
		"    private import ISQ::*;\n" +
		"    private import MeasurementReferences::*;\n" +
		"    attribute efficiency : DimensionOneValue;\n" +
		"}\n"
	got := string(result.Content)
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", got))
}

func TestAddImportIntoBodylessOwner(t *testing.T) {
	model := loadContent(t, "import.sysml", "part def D;\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("D", "", "ScalarValues::*", false, false, nil))
	want := "part def D {\n    private import ScalarValues::*;\n}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportAtRootBeforeAHeader(t *testing.T) {
	model := loadContent(t, "import.sysml",
		"// the document header\npackage P {\n    part a;\n}\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("", "", "ScalarValues::*", false, false, nil))
	want := "// the document header\nprivate import ScalarValues::*;\npackage P {\n    part a;\n}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportIntoEmptyDocument(t *testing.T) {
	model := loadContent(t, "import.sysml", "")
	result := applyOne(t, model, AddImport("", "", "ScalarValues::*", false, false, nil))
	if got := string(result.Content); got != "private import ScalarValues::*;\n" {
		t.Fatalf("content = %q", got)
	}
	requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
}

func TestAddImportNestedOwners(t *testing.T) {
	cases := []struct {
		name  string
		model string
		owner string
		want  string
	}{
		{
			"part def",
			"part def D {\n    part a;\n}\n",
			"D",
			"part def D {\n    private import ScalarValues::*;\n    part a;\n}\n",
		},
		{
			"action",
			"action def A {\n    action run;\n}\n",
			"A",
			"action def A {\n    private import ScalarValues::*;\n    action run;\n}\n",
		},
		{
			"state",
			"state def S {\n    state idle;\n}\n",
			"S",
			"state def S {\n    private import ScalarValues::*;\n    state idle;\n}\n",
		},
		{
			"requirement",
			"requirement def R {\n    subject s;\n}\n",
			"R",
			"requirement def R {\n    private import ScalarValues::*;\n    subject s;\n}\n",
		},
		{
			"metadata def",
			"metadata def M {\n    ref feature a;\n}\n",
			"M",
			"metadata def M {\n    private import ScalarValues::*;\n    ref feature a;\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := loadContent(t, "import.sysml", tc.model)
			requireClean(t, model)
			result := applyOne(t, model, AddImport(tc.owner, "", "ScalarValues::*", false, false, nil))
			if got := string(result.Content); got != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
			requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
		})
	}
}

// A calculation's result expression stays last: its imports are written before
// its other members.
func TestAddImportIntoCalculationKeepsResultLast(t *testing.T) {
	model := loadContent(t, "import.sysml",
		"calc def C {\n    in attribute x : ScalarValues::Real;\n    x * 2\n}\n")
	requireClean(t, model)
	result := applyOne(t, model, AddImport("C", "", "SI::*", false, false, nil))
	got := string(result.Content)
	want := "calc def C {\n    private import SI::*;\n    in attribute x : ScalarValues::Real;\n    x * 2\n}\n"
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", got))
}

func TestAddImportsInOneApplyKeepOrder(t *testing.T) {
	model := loadContent(t, "import.sysml", "package P {\n}\n")
	requireClean(t, model)
	result, err := Apply(model, []Operation{
		AddImport("P", "", "ScalarValues::*", false, false, nil),
		AddImport("P", "", "SI::*", false, false, nil),
		AddImport("P", "", "ISQ::*", false, false, nil),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "package P {\n    private import ScalarValues::*;\n    private import SI::*;\n    private import ISQ::*;\n}\n"
	got := string(result.Content)
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "import.sysml", got))
}

func TestAddImportRefusals(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		op      Operation
		failure Failure
	}{
		{
			"unknown owner", "package P {\n}\n",
			AddImport("Nope", "", "ScalarValues::*", false, false, nil),
			FailureOwnerUnknown,
		},
		{
			"non-namespace owner", "package P {\n    alias a for ScalarValues::Real;\n}\n",
			AddImport("P::a", "", "ScalarValues::*", false, false, nil),
			FailureOwnerNotNamespace,
		},
		{
			"enum def owner", "enum def E {\n    enum a;\n}\n",
			AddImport("E", "", "ScalarValues::*", false, false, nil),
			FailureIllegalKind,
		},
		{
			"bad visibility", "package P {\n}\n",
			AddImport("P", "friend", "ScalarValues::*", false, false, nil),
			FailureInvalidValue,
		},
		{
			"empty target", "package P {\n}\n",
			AddImport("P", "", "", false, false, nil),
			FailureInvalidName,
		},
		{
			"dangling scope", "package P {\n}\n",
			AddImport("P", "", "A::", false, false, nil),
			FailureInvalidName,
		},
		{
			"spaced target", "package P {\n}\n",
			AddImport("P", "", "A B", false, false, nil),
			FailureInvalidName,
		},
		{
			"recursive suffix in target", "package P {\n}\n",
			AddImport("P", "", "A::**", false, false, nil),
			FailureInvalidName,
		},
		{
			"injected member", "package P {\n}\n",
			AddImport("P", "", "A;part x", false, false, nil),
			FailureInvalidName,
		},
		{
			"invalid filter expression", "package P {\n}\n",
			AddImport("P", "", "ScalarValues::*", false, false, []string{"@M then x"}),
			FailureInvalidValue,
		},
		{
			"unresolved namespace", "package P {\n}\n",
			AddImport("P", "", "Nope::*", false, false, nil),
			FailureResultInvalid,
		},
		{
			"unresolved member", "package P {\n}\n",
			AddImport("P", "", "ScalarValues::Nope", false, false, nil),
			FailureResultInvalid,
		},
		{
			"public at root", "package P {\n}\n",
			AddImport("", "public", "ScalarValues::*", false, false, nil),
			FailureResultInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := loadContent(t, "import.sysml", tc.model)
			requireClean(t, model)
			addFailure(t, model, tc.op, tc.failure)
		})
	}
}

func TestAddImportRefusesADuplicate(t *testing.T) {
	cases := []struct {
		name  string
		model string
		op    Operation
		want  bool
	}{
		{
			"identical",
			"package P {\n    private import ScalarValues::*;\n    part a;\n}\n",
			AddImport("P", "private", "ScalarValues::*", false, false, nil),
			true,
		},
		{
			"default visibility is private",
			"package P {\n    private import ScalarValues::*;\n}\n",
			AddImport("P", "", "ScalarValues::*", false, false, nil),
			true,
		},
		{
			"different visibility is not a duplicate",
			"package P {\n    public import ScalarValues::*;\n    part a;\n}\n",
			AddImport("P", "private", "ScalarValues::*", false, false, nil),
			false,
		},
		{
			"quoted name is the same import",
			"package P {\n    private import 'ScalarValues'::*;\n    part a;\n}\n",
			AddImport("P", "private", "ScalarValues::*", false, false, nil),
			true,
		},
		{
			"filter whitespace does not matter",
			"package Q {\n    metadata def M;\n}\npackage P {\n    private import Q::*[@Q::M];\n    part a;\n}\n",
			AddImport("P", "private", "Q::*", false, false, []string{"@Q::M"}),
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := loadContent(t, "import.sysml", tc.model)
			requireClean(t, model)
			if tc.want {
				addFailure(t, model, tc.op, FailureMemberNameTaken)
				return
			}
			result := applyOne(t, model, tc.op)
			requireClean(t, loadContent(t, "import.sysml", string(result.Content)))
		})
	}
}
