package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestW7GStateReportsOnlyTheExtraSubactions(t *testing.T) {
	const src = `package S {
		action def A;
		state def St {
			entry action e1 : A;
			entry action e2 : A;
			do action d1 : A;
			do action d2 : A;
			exit action x1 : A;
			exit action x2 : A;
		}
	}`
	// The subaction rules read the declaration only, so they report at the type
	// tier rather than behind it.
	diags := typeDiags(t, src)
	for _, c := range []string{"state-entry-action", "state-do-action", "state-exit-action"} {
		got := only(diags, c)
		if len(got) != 1 {
			t.Fatalf("expected one %s diagnostic, got %v", c, got)
		}
		if got[0].Severity != diag.SeverityError {
			t.Fatalf("%s is an error in the reference, got %v", c, got[0].Severity)
		}
	}
}

func TestW7GStateWithOneSubactionOfEachKindIsSilent(t *testing.T) {
	const src = `package S {
		action def A;
		state def St {
			entry action e : A;
			do action d : A;
			exit action x : A;
		}
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("one subaction of each kind is valid, got %v", diags)
	}
}

func TestW7GRequirementReportsOnlyTheExtraSubject(t *testing.T) {
	const src = `package R {
		part def V;
		requirement def Req {
			subject v1 : V;
			subject v2 : V;
		}
	}`
	diags := only(constraintDiags(t, src), "only-one-subject")
	if len(diags) != 1 || diags[0].Message != msgOnlyOneSubject {
		t.Fatalf("expected one subject diagnostic worded as the reference words it, got %v", diags)
	}
}

func TestW7GCaseReportsOnlyTheExtraSubject(t *testing.T) {
	const src = `package C {
		part def V;
		case def Cs {
			subject v1 : V;
			subject v2 : V;
		}
	}`
	if got := len(only(constraintDiags(t, src), "only-one-subject")); got != 1 {
		t.Fatalf("expected one subject diagnostic, got %d", got)
	}
}

func TestW7GCaseReportsOnlyTheExtraObjective(t *testing.T) {
	const src = `package C {
		case def Cs {
			objective o1;
			objective o2;
		}
	}`
	if got := len(only(constraintDiags(t, src), "only-one-objective")); got != 1 {
		t.Fatalf("expected one objective diagnostic, got %d", got)
	}
}

func TestW7GVerificationAndUseCaseReportTheExtraObjective(t *testing.T) {
	for _, src := range []string{
		`package C {
			verification def Vs {
				objective o1;
				objective o2;
			}
		}`,
		`package C {
			use case def Us {
				objective o1;
				objective o2;
			}
		}`,
		`package C {
			verification v {
				objective o1;
				objective o2;
			}
		}`,
		`package C {
			use case u {
				objective o1;
				objective o2;
			}
		}`,
	} {
		if got := len(only(constraintDiags(t, src), "only-one-objective")); got != 1 {
			t.Fatalf("expected one objective diagnostic for %q, got %d", src, got)
		}
	}
}

// An analysis case improves several objectives lexicographically
// (internal/exec/solve), so the cardinality rule does not judge it.
func TestW7GAnalysisCaseAdmitsSeveralObjectives(t *testing.T) {
	for _, src := range []string{
		`package C {
			analysis def As {
				objective o1;
				objective o2;
			}
		}`,
		`package C {
			analysis a {
				objective o1;
				objective o2;
			}
		}`,
	} {
		if diags := only(constraintDiags(t, src), "only-one-objective"); len(diags) != 0 {
			t.Fatalf("analysis case objectives were diagnosed for %q: %v", src, diags)
		}
	}
}

// An `objective : R;` is a member under no name, so it competes where a named
// one does — owned, inherited, and mixed with a named one.
func TestW7GAnonymousObjectivesCompete(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want int
	}{
		{`package P {
			requirement def R1;
			case def Anon {
				objective : R1;
				objective : R1;
			}
			case anonUse : Anon;
		}`, 2},
		{`package P {
			requirement def R1;
			case def Mixed {
				objective named : R1;
				objective : R1;
			}
			case mixedUse : Mixed;
		}`, 2},
		{`package P {
			requirement def R1;
			case def OneAnon {
				objective : R1;
			}
			case anonOk : OneAnon;
		}`, 0},
	} {
		if diags := only(constraintDiags(t, tc.src), "only-one-objective"); len(diags) != tc.want {
			t.Fatalf("expected %d objective diagnostic(s) for %q, got %v", tc.want, tc.src, diags)
		}
	}
}

// The first owned objective redefines the inherited one, so only the second is
// reported — the pinned reference is silent on the first.
func TestW7GOwnedObjectiveRedefinesTheInheritedOne(t *testing.T) {
	const src = `package C {
		case def Base { objective inherited; }
		case c : Base {
			objective one;
			objective two;
		}
	}`
	diags := only(constraintDiags(t, src), "only-one-objective")
	if len(diags) != 1 {
		t.Fatalf("expected only the second owned objective to be diagnosed, got %v", diags)
	}
	if got := src[diags[0].Span.Offset:]; len(got) < len("objective two;") || got[:len("objective two;")] != "objective two;" {
		t.Fatalf("diagnostic is not on the second objective: %q", got)
	}
}

func TestW7GCaseReportsInheritedObjectiveConflictOnOwner(t *testing.T) {
	const src = `package P {
		case def C {
			objective o1;
			objective o2;
		}
	case c1 : C;
	}`
	diags := only(constraintDiags(t, src), "only-one-objective")
	if len(diags) != 2 {
		t.Fatalf("expected the inherited conflict and its source declaration, got %v", diags)
	}
	if got := src[diags[1].Span.Offset:]; len(got) < len("case c1 : C;") ||
		got[:len("case c1 : C;")] != "case c1 : C;" {
		t.Fatalf("inherited objective diagnostic is not on the usage: %q", got)
	}
}

func TestW7GInheritedSubjectsCompeteUnlessAnOwnedOneRedefinesThem(t *testing.T) {
	const src = `package R {
		requirement def A { subject a; }
		requirement def B { subject b; }
		requirement def Both :> A, B;
		requirement def Mine :> A, B { subject m; }
		requirement def Clause :> A, B { subject c :>> a; }
		requirement def Extra :> A { subject e1; subject e2; }
	}`
	diags := only(constraintDiags(t, src), "only-one-subject")
	if len(diags) != 2 {
		t.Fatalf("expected the inherited pair and the second owned subject, got %v", diags)
	}
	for i, want := range []string{"requirement def Both :> A, B;", "subject e2;"} {
		if got := src[diags[i].Span.Offset:]; len(got) < len(want) || got[:len(want)] != want {
			t.Fatalf("diagnostic %d is at %q, want %q", i, got, want)
		}
	}
}

func TestW7GSubjectAfterAnotherParameterIsReported(t *testing.T) {
	const src = `package R {
		part def V;
		requirement def Req {
			in attribute n;
			subject v : V;
		}
	}`
	diags := only(constraintDiags(t, src), "subject-parameter-position")
	if len(diags) != 1 || diags[0].Message != msgSubjectParameterPosition {
		t.Fatalf("expected the subject-position diagnostic, got %v", diags)
	}
}

func TestW7GSubjectAsTheFirstParameterIsSilent(t *testing.T) {
	const src = `package R {
		part def V;
		requirement def Req {
			subject v : V;
			in attribute n;
		}
	}`
	if diags := only(constraintDiags(t, src), "subject-parameter-position"); len(diags) != 0 {
		t.Fatalf("a subject declared first conforms, got %v", diags)
	}
}

func TestW7GExpressionSubjectBeforeLaterInputIsSilent(t *testing.T) {
	const src = `package R {
		part vehicle;
		analysis a {
			subject = vehicle;
			in attribute scenario;
		}
	}`
	if diags := only(constraintDiags(t, src), "subject-parameter-position"); len(diags) != 0 {
		t.Fatalf("an expression subject before a later input is first, got %v", diags)
	}
}

func TestW7GLocalResultSuppressesInheritedParameterFallback(t *testing.T) {
	const src = `package R {
		part def V;
		requirement def Base {
			in attribute inherited;
			subject baseSubject : V;
		}
		requirement def Child :> Base {
			return attribute result;
		}
	}`
	if diags := only(constraintDiags(t, src), "subject-parameter-position"); len(diags) != 1 {
		t.Fatalf("a locally declared result must prevent inherited-parameter fallback, got %v", diags)
	}
}

// The reference reports the position rule on a requirement whose first parameter
// is not a subject even when it declares no subject at all (matched run).
func TestW7GParameterBeforeAnAbsentSubjectIsReported(t *testing.T) {
	const src = `package R {
		requirement def Req {
			in attribute n;
		}
	}`
	if got := len(only(constraintDiags(t, src), "subject-parameter-position")); got != 1 {
		t.Fatalf("expected the subject-position diagnostic on the declaration, got %d", got)
	}
}

func TestW7GRequirementWithNoParameterIsSilent(t *testing.T) {
	const src = `package R {
		requirement def Req {
			attribute n;
		}
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("a requirement with no parameter has an implicit subject, got %v", diags)
	}
}

// An owned objective beside an inherited one it does not redefine is reported
// on the owned objective (pilot checkAtMostOneRelationship, mixed ownership),
// wherever the inherited one comes from.
func TestW7GOwnedObjectiveBesideAnUnredefinedInheritedOne(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want []string
	}{
		{`package P {
			case def C { objective o1; objective o2; }
			case c : C { objective o3; objective o4; }
		}`, []string{"objective o2;", "objective o3;", "objective o4;"}},
		{`package P {
			case def C { objective o1; objective o2; }
			case def C1 :> C { objective o5; }
		}`, []string{"objective o2;", "objective o5;"}},
		{`package P {
			case def C { objective o1; objective o2; }
			case def C4 :> C { objective o7 :>> o1; }
		}`, []string{"objective o2;", "objective o7 :>> o1;"}},
		{`package P {
			case def C { objective o1; objective o2; }
			case def C3 :> C { objective o6 :>> o2; }
		}`, []string{"objective o2;"}},
		{`package P {
			case def B1 { objective b1; }
			case def B2 { objective b2; }
			case def D :> B1, B2;
			case def D2 :> B1, B2 { objective d; }
		}`, []string{"case def D :> B1, B2;"}},
	} {
		diags := only(constraintDiags(t, tc.src), "only-one-objective")
		if len(diags) != len(tc.want) {
			t.Fatalf("expected %d objective diagnostic(s) for %q, got %v", len(tc.want), tc.src, diags)
		}
		for i, want := range tc.want {
			if got := tc.src[diags[i].Span.Offset:]; len(got) < len(want) || got[:len(want)] != want {
				t.Fatalf("diagnostic %d is not on %q: %q", i, want, got)
			}
		}
	}
}
