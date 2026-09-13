package edit

import (
	"strings"
	"testing"
)

func TestDeleteReferencedRefusesAndCascadeRemovesReferrers(t *testing.T) {
	src := "package P {\n    part def Base;\n    part x : Base;\n}\n"
	m := loadContent(t, "delete.sysml", src)
	e := addFailure(t, m, Delete("P::Base", false), FailureDeleteReferenced)
	if !strings.Contains(strings.Join(e.Referring, ","), "P") {
		t.Fatalf("referrers = %v, want P", e.Referring)
	}
	res, err := Apply(m, []Operation{Delete("P::Base", true)})
	if err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	if strings.Contains(string(res.Content), "Base") || strings.Contains(string(res.Content), "part x") {
		t.Fatalf("cascade left target or referrer:\n%s", res.Content)
	}
}

// A cascade removes referrers of referrers too, so the model left behind
// declares nothing that has lost its target.
func TestDeleteCascadeIsTransitive(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part x : Base;\n" +
		"    part y : Base;\n" +
		"    connection c connect x to y;\n" +
		"    part z = c;\n" +
		"    part def Keep;\n" +
		"    part k : Keep;\n" +
		"}\n"
	m := loadContent(t, "delete.sysml", src)
	requireClean(t, m)

	e := addFailure(t, m, Delete("P::Base", false), FailureDeleteReferenced)
	want := []string{"P::x", "P::y", "P::c", "P::z"}
	if strings.Join(e.Referring, ",") != strings.Join(want, ",") {
		t.Fatalf("referrers = %v, want %v", e.Referring, want)
	}

	res, err := Apply(m, []Operation{Delete("P::Base", true)})
	if err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	got := string(res.Content)
	if got != "package P {\n    part def Keep;\n    part k : Keep;\n}\n" {
		t.Fatalf("cascade left dangling declarations:\n%s", got)
	}
	requireClean(t, loadContent(t, "delete.sysml", got))
}

// Deleting a declaration takes its members with it, so what refers to a member
// is a referrer too, and a referrer nested in the target is not spliced twice.
func TestDeleteCascadeFollowsMembersOfTheTarget(t *testing.T) {
	src := "package P {\n" +
		"    part def Base {\n" +
		"        part def Inner;\n" +
		"        part self : Base;\n" +
		"    }\n" +
		"    part b : Base;\n" +
		"    part def Other {\n" +
		"        part i : Base::Inner;\n" +
		"    }\n" +
		"    part def Keep;\n" +
		"}\n"
	m := loadContent(t, "delete.sysml", src)
	requireClean(t, m)

	e := addFailure(t, m, Delete("P::Base", false), FailureDeleteReferenced)
	want := []string{"P::Other::i", "P::b"}
	if strings.Join(e.Referring, ",") != strings.Join(want, ",") {
		t.Fatalf("referrers = %v, want %v", e.Referring, want)
	}

	res, err := Apply(m, []Operation{Delete("P::Base", true)})
	if err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	got := string(res.Content)
	if got != "package P {\n    part def Other {\n    }\n    part def Keep;\n}\n" {
		t.Fatalf("cascade result:\n%s", got)
	}
	requireClean(t, loadContent(t, "delete.sysml", got))
}

// Referrers are told apart by declaration, not by qualified name, so a cascade
// removes every one of several same-named referrers rather than the first only.
func TestDeleteCascadeRemovesEverySameNamedReferrer(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part f : Base;\n" +
		"    part f : Base;\n" +
		"    part def Keep;\n" +
		"}\n"
	m := loadContent(t, "delete.sysml", src)
	requireClean(t, m)

	e := addFailure(t, m, Delete("P::Base", false), FailureDeleteReferenced)
	if want := []string{"P::f", "P::f"}; strings.Join(e.Referring, ",") != strings.Join(want, ",") {
		t.Fatalf("referrers = %v, want %v", e.Referring, want)
	}

	res, err := Apply(m, []Operation{Delete("P::Base", true)})
	if err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	got := string(res.Content)
	if got != "package P {\n    part def Keep;\n}\n" {
		t.Fatalf("cascade left a same-named referrer:\n%s", got)
	}
	requireClean(t, loadContent(t, "delete.sysml", got))
}

// An import, a filter or an anonymous declaration referring to the target is
// the referrer itself, not the namespace it is written in: a cascade removes
// that one declaration and leaves the namespace's other members.
func TestDeleteCascadeRemovesTheReferringDeclarationOnly(t *testing.T) {
	tests := []struct {
		name, src, referrer, want string
	}{
		{
			name: "import in a package",
			src: "package P {\n    part def Base;\n}\n" +
				"package Q {\n    private import P::Base;\n    part keep;\n}\n",
			referrer: "import P::Base in Q",
			want:     "package P {\n}\npackage Q {\n    part keep;\n}\n",
		},
		{
			name:     "import at the root",
			src:      "package P {\n    part def Base;\n}\nprivate import P::Base;\npart keep;\n",
			referrer: "import P::Base in delete.sysml",
			want:     "package P {\n}\npart keep;\n",
		},
		{
			name: "namespace import",
			src: "package P {\n    part def Base;\n}\n" +
				"package Q {\n    import P::Base::*;\n    part keep;\n}\n",
			referrer: "import P::Base::* in Q",
			want:     "package P {\n}\npackage Q {\n    part keep;\n}\n",
		},
		{
			name: "filter",
			src: "package P {\n    metadata def Base;\n" +
				"    package Q {\n        filter @Base;\n        part def Keep;\n    }\n}\n",
			referrer: "the filter in P::Q",
			want:     "package P {\n    package Q {\n        part def Keep;\n    }\n}\n",
		},
		{
			name:     "anonymous usage",
			src:      "package P {\n    part def Base;\n    part : Base;\n    part def Keep;\n}\n",
			referrer: "part : Base in P",
			want:     "package P {\n    part def Keep;\n}\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "delete.sysml", tc.src)
			requireClean(t, m)
			e := addFailure(t, m, Delete("P::Base", false), FailureDeleteReferenced)
			if strings.Join(e.Referring, ",") != tc.referrer {
				t.Fatalf("referrers = %v, want %q", e.Referring, tc.referrer)
			}
			res, err := Apply(m, []Operation{Delete("P::Base", true)})
			if err != nil {
				t.Fatalf("cascade delete: %v", err)
			}
			if got := string(res.Content); got != tc.want {
				t.Fatalf("cascade result:\n%s\nwant:\n%s", got, tc.want)
			}
			requireClean(t, loadContent(t, "delete.sysml", string(res.Content)))
		})
	}
}

// A declaration another workspace document refers to is not deleted, with or
// without cascade: an edit rewrites one document, so the reference could not
// follow. The refusal names each referrer with its document.
func TestDeleteRefusesWhenAnotherDocumentRefers(t *testing.T) {
	m := loadWorkspace(t, "p.sysml",
		"package P {\n    part def Base {\n        part def Inner;\n    }\n    part def Keep;\n}\n",
		map[string]string{
			"q.sysml": "package Q {\n    private import P::Base;\n    part b : P::Base;\n    part : P::Base::Inner;\n}\n",
			"r.sysml": "package R {\n    part k : P::Keep;\n}\n",
		})
	requireClean(t, m)
	want := []string{"Q::b (q.sysml)", "an anonymous part in Q (q.sysml)", "import P::Base in Q (q.sysml)"}
	for _, cascade := range []bool{false, true} {
		e := addFailure(t, m, Delete("P::Base", cascade), FailureReferencedElsewhere)
		if strings.Join(e.Referring, ",") != strings.Join(want, ",") {
			t.Fatalf("cascade=%v: referrers = %v, want %v", cascade, e.Referring, want)
		}
	}
	if _, err := Apply(m, []Operation{Delete("P::Keep", true)}); err == nil {
		t.Fatal("deleting P::Keep succeeded although r.sysml refers to it")
	}
	if _, err := Apply(m, []Operation{Delete("P::Base::Inner", true)}); err == nil {
		t.Fatal("deleting P::Base::Inner succeeded although q.sysml refers to it")
	}
}

// A local cascade proceeds when the other documents refer to something else.
func TestDeleteCascadesWhenOtherDocumentsReferElsewhere(t *testing.T) {
	m := loadWorkspace(t, "p.sysml",
		"package P {\n    part def Base;\n    part b : Base;\n    part def Keep;\n}\n",
		map[string]string{"q.sysml": "package Q {\n    part k : P::Keep;\n}\n"})
	requireClean(t, m)
	res, err := Apply(m, []Operation{Delete("P::Base", true)})
	if err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	if got := string(res.Content); got != "package P {\n    part def Keep;\n}\n" {
		t.Fatalf("cascade result:\n%s", got)
	}
}

func TestDeleteOnlyMemberRootAndNeighborTrivia(t *testing.T) {
	tests := []struct {
		name, src, target, want string
	}{
		{
			name:   "only member",
			src:    "package P {\n    part def Only;\n}\n",
			target: "P::Only",
			want:   "package P {\n}\n",
		},
		{
			name:   "root declaration",
			src:    "// keep\npart def Keep;\n\n// remove\npart def Gone;\n",
			target: "Gone",
			want:   "// keep\npart def Keep;\n",
		},
		{
			name:   "blank line stops leading comment scan",
			src:    "// keep this\n\n// remove this\npart def Gone;\n",
			target: "Gone",
			want:   "// keep this\n",
		},
		{
			name:   "neighbor comment and blank line",
			src:    "package P {\n    // keep\n    part def Keep;\n\n    // remove\n    part def Gone;\n\n    // neighbor\n    part def Next;\n}\n",
			target: "P::Gone",
			want:   "package P {\n    // keep\n    part def Keep;\n\n    // neighbor\n    part def Next;\n}\n",
		},
		{
			name:   "trailing line comment goes with the declaration",
			src:    "package P {\n    part def Keep;\n    part def Gone; // gone too\n    part def Next; // stays\n}\n",
			target: "P::Gone",
			want:   "package P {\n    part def Keep;\n    part def Next; // stays\n}\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "delete.sysml", tc.src)
			res, err := Apply(m, []Operation{Delete(tc.target, false)})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if got := string(res.Content); got != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMixedBatchAppliesInRequestOrder(t *testing.T) {
	src := "package P {\n    attribute a = 1;\n    part def Gone;\n}\n"
	m := loadContent(t, "mixed.sysml", src)
	res, err := Apply(m, []Operation{
		{
			Kind:       OpAddMember,
			Owner:      "P",
			MemberKind: "attribute",
			MemberName: "b",
			Value:      "2",
		},
		SetValue("P::a", "3"),
		Delete("P::Gone", false),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	if !strings.Contains(got, "attribute a = 3;") ||
		!strings.Contains(got, "attribute b = 2;") ||
		strings.Contains(got, "Gone") {
		t.Fatalf("mixed batch result:\n%s", got)
	}
}
