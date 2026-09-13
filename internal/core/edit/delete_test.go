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
