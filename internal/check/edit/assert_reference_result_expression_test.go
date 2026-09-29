package edit

import (
	"strings"
	"testing"
)

func TestAddReferenceAssertions(t *testing.T) {
	source := "package P {\n" +
		"    constraint def C;\n" +
		"    part def Holder {\n" +
		"        constraint c : C;\n" +
		"    }\n" +
		"    part host : Holder;\n" +
		"    part def Host {\n" +
		"        constraint c : C;\n" +
		"    }\n" +
		"}\n"
	tests := []struct {
		name, kind, reference, want string
	}{
		{"assert", "assert", "c", "assert c;"},
		{"assert not", "assert not", "c", "assert not c;"},
		{"dotted reference", "assert", "host.c", "assert host.c;"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "assert-reference.sysml", source)
			result := applyOne(t, model, AddMember("P::Host", test.kind, test.reference))
			if !strings.Contains(string(result.Content), test.want) {
				t.Fatalf("notation lacks %q:\n%s", test.want, result.Content)
			}
			requireClean(t, loadContent(t, "assert-reference.sysml", string(result.Content)))
		})
	}
}

func TestReferenceAssertionsAreAnonymous(t *testing.T) {
	model := loadContent(t, "assert-reference.sysml", `package P {
    constraint def C;
    part def Host {
        constraint c : C;
    }
}
`)
	op := AddMember("P::Host", "assert", "c")
	result := applyOne(t, model, op)
	requireClean(t, loadContent(t, "assert-reference.sysml", string(result.Content)))

	result, err := Apply(model, []Operation{op, op})
	if err != nil {
		t.Fatalf("identical anonymous assertions: %v", err)
	}
	if got := strings.Count(string(result.Content), "assert c;"); got != 2 {
		t.Fatalf("assertion count = %d, want 2:\n%s", got, result.Content)
	}
	requireClean(t, loadContent(t, "assert-reference.sysml", string(result.Content)))
}

func TestReferenceAssertionsValidateReferenceAndRejectMemberOptions(t *testing.T) {
	model := loadContent(t, "assert-reference.sysml", `package P {
    constraint def C;
    part def Host { constraint c : C; attribute a; }
}
`)
	for _, test := range []struct {
		name string
		op   Operation
		want Failure
	}{
		{"empty reference", AddMember("P::Host", "assert", ""), FailureInvalidName},
		{"malformed reference", AddMember("P::Host", "assert", "c +"), FailureInvalidName},
		{"type", func() Operation { op := AddMember("P::Host", "assert", "c"); op.Type = "C"; return op }(), FailureIllegalKind},
		{"value", func() Operation { op := AddMember("P::Host", "assert", "c"); op.Value = "true"; return op }(), FailureIllegalKind},
		{"multiplicity", func() Operation { op := AddMember("P::Host", "assert", "c"); op.Multiplicity = "[1]"; return op }(), FailureIllegalKind},
		{"default", func() Operation { op := AddMember("P::Host", "assert", "c"); op.IsDefault = true; return op }(), FailureIllegalKind},
		{"abstract", func() Operation { op := AddMember("P::Host", "assert", "c"); op.IsAbstract = true; return op }(), FailureIllegalKind},
		{"direction", func() Operation { op := AddMember("P::Host", "assert", "c"); op.Direction = "in"; return op }(), FailureIllegalKind},
		{"redefines", func() Operation {
			op := AddMember("P::Host", "assert", "c")
			op.Redefines = []string{"other"}
			return op
		}(), FailureIllegalKind},
		{"body expression", func() Operation { op := AddMember("P::Host", "assert", "c"); op.BodyExpression = "true"; return op }(), FailureIllegalKind},
	} {
		t.Run(test.name, func(t *testing.T) {
			failure := addFailure(t, model, test.op, test.want)
			if failure.Message == "" {
				t.Fatal("refusal message is empty")
			}
		})
	}
}

func TestReferenceAssertionsRequireConstraintReferencesAfterAnalysis(t *testing.T) {
	for _, test := range []struct {
		name, reference string
	}{
		{"attribute", "a"},
		{"unresolved", "missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "assert-reference.sysml", `package P {
    part def Host { attribute a; }
}
`)
			failure := addFailure(t, model, AddMember("P::Host", "assert", test.reference), FailureResultInvalid)
			if len(failure.Diagnostics) == 0 {
				t.Fatalf("post-edit refusal has no analysis diagnostics: %+v", failure)
			}
		})
	}
}

func TestAddCalculationAndCaseResultExpressions(t *testing.T) {
	source := `package P {
    calc def CalcType;
    case def CaseType;
    analysis def AnalysisType;
    verification def VerificationType;
    use case def UseCaseType;
}
`
	tests := []struct {
		kind, name, typ string
	}{
		{"calc def", "CalcDef", ""},
		{"calc", "calcUsage", "CalcType"},
		{"case def", "CaseDef", ""},
		{"case", "caseUsage", "CaseType"},
		{"analysis def", "AnalysisDef", ""},
		{"analysis", "analysisUsage", "AnalysisType"},
		{"verification def", "VerificationDef", ""},
		{"verification", "verificationUsage", "VerificationType"},
		{"use case def", "UseCaseDef", ""},
		{"use case", "useCaseUsage", "UseCaseType"},
	}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			model := loadContent(t, "result-expression.sysml", source)
			op := AddMember("P", test.kind, test.name)
			op.Type = test.typ
			op.BodyExpression = "1"
			result := applyOne(t, model, op)
			want := test.kind + " " + test.name
			if test.typ != "" {
				want += " : " + test.typ
			}
			want += " { 1 }"
			if !strings.Contains(string(result.Content), want) {
				t.Fatalf("notation lacks %q:\n%s", want, result.Content)
			}
			requireClean(t, loadContent(t, "result-expression.sysml", string(result.Content)))
		})
	}
}

func TestAddResultExpressionUsageFormAndMultilineBody(t *testing.T) {
	model := loadContent(t, "result-expression.sysml", `package P {
    calc def C;
    part def Host { attribute x; }
}
`)
	op := AddMember("P::Host", "calc", "c")
	op.Type = "C"
	op.BodyExpression = "x * 2"
	result := applyOne(t, model, op)
	if !strings.Contains(string(result.Content), "calc c : C { x * 2 }") {
		t.Fatalf("typed calculation usage missing:\n%s", result.Content)
	}

	model = loadContent(t, "result-expression.sysml", "package P {\n    calc def C;\n}\n")
	op = AddMember("P", "calc def", "D")
	op.BodyExpression = "1 * 2\n  + 1"
	result = applyOne(t, model, op)
	if !strings.Contains(string(result.Content), "calc def D {\n        1 * 2\n          + 1\n    }") {
		t.Fatalf("multiline result expression indentation is wrong:\n%s", result.Content)
	}
}

func TestMembersAddedAfterResultExpressionPrecedeIt(t *testing.T) {
	t.Run("calculation input", func(t *testing.T) {
		model := loadContent(t, "result-expression.sysml", "package P { calc def D { x * 2 } }\n")
		op := AddMember("P::D", "ref", "x")
		op.Type, op.Direction = "ScalarValues::Real", "in"
		result := applyOne(t, model, op)
		got := string(result.Content)
		if strings.Index(got, "in ref x : ScalarValues::Real;") > strings.Index(got, "x * 2") {
			t.Fatalf("input follows result expression:\n%s", got)
		}
	})
	t.Run("case subject", func(t *testing.T) {
		model := loadContent(t, "result-expression.sysml", "package P { case def C { 1 } }\n")
		result := applyOne(t, model, AddMember("P::C", "subject", "s"))
		got := string(result.Content)
		if strings.Index(got, "subject s;") > strings.Index(got, "1") {
			t.Fatalf("subject follows result expression:\n%s", got)
		}
	})
}

func TestRequirementBodyKindsRefuseResultExpressionsSpecifically(t *testing.T) {
	model := loadContent(t, "requirement-body.sysml", "package P;\n")
	for _, kind := range []string{
		"requirement def", "requirement", "concern def", "concern",
		"viewpoint def", "viewpoint", "objective",
	} {
		t.Run(kind, func(t *testing.T) {
			op := AddMember("P", kind, "R")
			op.BodyExpression = "1"
			failure := addFailure(t, model, op, FailureIllegalKind)
			if !strings.Contains(failure.Message, "has no result expression") ||
				!strings.Contains(failure.Message, "require constraint") {
				t.Fatalf("body-expression refusal = %q", failure.Message)
			}
		})
	}
}
