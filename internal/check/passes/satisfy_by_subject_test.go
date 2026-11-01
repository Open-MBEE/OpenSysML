package passes

import (
	"strings"
	"testing"
)

func assertSatisfySubjectDiagnostics(t *testing.T, src, code, message string, members ...string) {
	t.Helper()
	diags := only(constraintDiags(t, src), code)
	if len(diags) != len(members) {
		t.Fatalf("got %d %s diagnostics, want %d: %v", len(diags), code, len(members), diags)
	}
	for i, member := range members {
		if diags[i].Message != message {
			t.Errorf("diagnostic %d message = %q, want %q", i, diags[i].Message, message)
		}
		offset := strings.Index(src, member)
		if offset < 0 {
			t.Fatalf("member %q not found in source", member)
		}
		if diags[i].Span.Offset != offset {
			t.Errorf("diagnostic %d offset = %d, want %d at %q", i, diags[i].Span.Offset, offset, member)
		}
	}
}

func TestSatisfyBySubjectThenBodySubjectReportsExtraBodySubject(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	requirement def SoftLanding { subject lander : Lander; }
	part slowLander : Lander;
	assert satisfy requirement grounded : SoftLanding by slowLander {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject,
		"subject lander : Lander;\n\t\trequire constraint")
}

func TestUntypedSatisfyBySubjectThenBodySubjectReportsExtraBodySubject(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	part slowLander : Lander;
	assert satisfy requirement grounded by slowLander {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject, "subject lander : Lander;")
}

func TestSatisfyBySubjectAndTwoBodySubjectsReportsBothBodySubjects(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	part slowLander : Lander;
	assert satisfy requirement grounded by slowLander {
		subject lander : Lander;
		subject other : Lander;
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject,
		"subject lander : Lander;", "subject other : Lander;")
}

func TestSatisfyWithoutByReportsSecondBodySubject(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	requirement def SoftLanding { subject lander : Lander; }
	assert satisfy requirement grounded : SoftLanding {
		subject lander : Lander;
		subject other : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject, "subject other : Lander;")
}

func TestSatisfyByOnlyHasOneSubject(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	requirement def SoftLanding { subject lander : Lander; }
	part slowLander : Lander;
	assert satisfy requirement grounded : SoftLanding by slowLander {
		require constraint { slowLander.verticalSpeed <= 1.5 }
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject)
}

func TestSatisfyBodyOnlyHasOneSubject(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	requirement def SoftLanding { subject lander : Lander; }
	assert satisfy requirement grounded : SoftLanding {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject)
}

func TestSatisfyByReferencingRequirementDoesNotReportDuplicateSubject(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	part slowLander : Lander;
	requirement touchdown { subject lander : Lander; require constraint { lander.verticalSpeed <= 1.5 } }
	satisfy touchdown by slowLander;
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject)
}

func TestSatisfyBySubjectPrecedesBodyInputParameter(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	part slowLander : Lander;
	assert satisfy requirement grounded by slowLander {
		in attribute limit : ScalarValues::Real = 1.5;
		require constraint { slowLander.verticalSpeed <= limit }
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject)
	assertSatisfySubjectDiagnostics(t, src, "subject-parameter-position", msgSubjectParameterPosition)
}

func TestSatisfyBySubjectDoesNotMakeRequirementSubjectNameAvailable(t *testing.T) {
	const src = `package P {
	part def X { attribute v : ScalarValues::Real; }
	requirement def R { subject s : X; }
	part x : X;
	assert satisfy requirement g : R by x { require constraint { s.v <= 1 } }
}`
	assertSatisfySubjectDiagnostics(t, src, "only-one-subject", msgOnlyOneSubject)
}

func TestSatisfyWithoutBySubjectAfterInputParameterReportsPosition(t *testing.T) {
	const src = `package P {
	part def Lander { attribute verticalSpeed : ScalarValues::Real; }
	assert satisfy requirement grounded {
		in attribute limit : ScalarValues::Real = 1.5;
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= limit }
	}
}`
	assertSatisfySubjectDiagnostics(t, src, "subject-parameter-position",
		msgSubjectParameterPosition, "subject lander : Lander;")
}
