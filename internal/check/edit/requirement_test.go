package edit

import (
	"strings"
	"testing"
)

const satisfyFixture = `part def Vehicle;
part vehicle : Vehicle;
requirement spec { subject s : Vehicle; }
part holder {
    part owned : Vehicle;
}
`

func TestAddSatisfyWritesAndAnalyzes(t *testing.T) {
	m := loadContent(t, "satisfy.sysml", satisfyFixture)
	op := AddSatisfy("holder", "spec", "owned", true, true)
	res, err := Apply(m, []Operation{op})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := string(res.Content); !strings.Contains(got, "assert not satisfy spec by owned;") {
		t.Fatalf("satisfy usage not written:\n%s", got)
	}
	requireClean(t, loadContent(t, "satisfy.sysml", string(res.Content)))
}

func TestAddSatisfyRejectsNonRequirementTarget(t *testing.T) {
	m := loadContent(t, "satisfy-nonrequirement.sysml", satisfyFixture)
	e := addFailure(t, m, AddSatisfy("holder", "Vehicle", "owned", false, false), FailureResultInvalid)
	if !strings.Contains(strings.ToLower(e.Message), "requirement") {
		t.Fatalf("non-requirement satisfy target was rejected for an unexpected reason: %q", e.Message)
	}
}

func TestAddSatisfyWritesAtPackageLevel(t *testing.T) {
	const src = `part def Vehicle;
requirement def R { subject s : Vehicle; }
package Demo {
    requirement r : R { subject s : Vehicle; }
    part t : Vehicle;
}
`
	m := loadContent(t, "satisfy-package.sysml", src)
	res, err := Apply(m, []Operation{AddSatisfy("Demo", "r", "t", true, false)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := string(res.Content); !strings.Contains(got, "assert satisfy r by t;") {
		t.Fatalf("package-level satisfy not written:\n%s", got)
	}
	requireClean(t, loadContent(t, "satisfy-package.sysml", string(res.Content)))
}

func TestAddSatisfyRejectsEnumerationBody(t *testing.T) {
	m := loadContent(t, "satisfy-enum.sysml", "enum def E { one; }\n")
	addFailure(t, m, AddSatisfy("E", "r", "t", false, false), FailureIllegalKind)
}

func TestAddSatisfyRejectsKerML(t *testing.T) {
	m := loadContent(t, "satisfy.kerml", "package P;\n")
	addFailure(t, m, AddSatisfy("", "R", "", false, false), FailureIllegalKind)
}

func TestAddRequirementConstraintWritesAndAnalyzes(t *testing.T) {
	m := loadContent(t, "constraint.sysml", "part def Vehicle;\nrequirement def R { subject s : Vehicle; }\n")
	res, err := Apply(m, []Operation{
		AddRequirementConstraint("R", "require", "true", "isValid"),
		AddRequirementConstraint("R", "assume", "true", ""),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	for _, want := range []string{"require constraint isValid { true }", "assume constraint { true }"} {
		if !strings.Contains(got, want) {
			t.Fatalf("content lacks %q:\n%s", want, got)
		}
	}
	requireClean(t, loadContent(t, "constraint.sysml", got))
}

func TestAddRequirementConstraintRejectsQuotedDuplicateName(t *testing.T) {
	m := loadContent(t, "constraint-quoted-duplicate.sysml",
		"part def Vehicle;\nrequirement def R { subject s : Vehicle; require constraint isValid { true } }\n")
	addFailure(t, m, AddRequirementConstraint("R", "require", "true", "'isValid'"),
		FailureMemberNameTaken)
}

func TestAddRequirementConstraintRefusals(t *testing.T) {
	t.Run("outside requirement body", func(t *testing.T) {
		m := loadContent(t, "part.sysml", "part def P;\n")
		addFailure(t, m, AddRequirementConstraint("P", "require", "true", ""), FailureIllegalKind)
	})
	t.Run("invalid kind", func(t *testing.T) {
		m := loadContent(t, "requirement.sysml", "requirement def R;\n")
		addFailure(t, m, AddRequirementConstraint("R", "assert", "true", ""), FailureIllegalKind)
	})
	t.Run("empty expression", func(t *testing.T) {
		m := loadContent(t, "requirement.sysml", "requirement def R;\n")
		addFailure(t, m, AddRequirementConstraint("R", "require", "", ""), FailureInvalidValue)
	})
	t.Run("invalid expression", func(t *testing.T) {
		m := loadContent(t, "requirement.sysml", "requirement def R;\n")
		addFailure(t, m, AddRequirementConstraint("R", "require", "true &&", ""), FailureInvalidValue)
	})
	t.Run("KerML", func(t *testing.T) {
		m := loadContent(t, "constraint.kerml", "package P;\n")
		addFailure(t, m, AddRequirementConstraint("", "require", "true", ""), FailureIllegalKind)
	})
}
