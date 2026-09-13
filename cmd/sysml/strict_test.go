package main

import "testing"

// extensionModel uses OpenSysML notation no SysML v2 production admits.
const extensionModel = `package Mission {
    attribute def Alarm;
    state def Monitor {
        entry; then off;
        state off {
            defer Alarm;
        }
        state on;
        transition first off accept Alarm then on;
    }
}
`

// -strict answers a different question about the same file: the notation stays
// parsed, and the warnings it draws by default become the errors that decide the
// exit status.
func TestStrictConformanceDecidesTheExitStatus(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, extensionModel, "-validate"),
		0, "warning:", "is an OpenSysML extension with no SysML v2 production")

	strict := check(t, binary, extensionModel, "-validate", "-strict")
	wantReport(t, strict, 2, "error:", "is an OpenSysML extension with no SysML v2 production",
		"did not analyse cleanly")
	rejectReport(t, strict, "warning:")
}

// Every state-body notation outside StateBodyItem (SysML.xtext) is a
// nonstandard-notation warning by default and an error under -strict.
func TestStateBodyExtensionsAreReportedByDefault(t *testing.T) {
	binary := buildCLI(t)
	const corpus = "../pilot-reject/testdata/negative/extensions/"
	for _, tc := range []struct{ file, notation string }{
		{"x02-choice-pseudostate.sysml", "`choice <name>;`"},
		{"x03-junction-pseudostate.sysml", "`junction <name>;`"},
		{"x05-defer-member.sysml", "`defer <event>;`"},
		{"x06-history-member.sysml", "`history <name>;`"},
	} {
		want := tc.notation + " is an OpenSysML extension with no SysML v2 production"
		got := checkPaths(t, binary, "-validate", corpus+tc.file)
		wantReport(t, got, got.status, "warning: "+want)
		rejectReport(t, got, "error: "+want)

		strict := checkPaths(t, binary, "-validate", "-strict", corpus+tc.file)
		wantReport(t, strict, 2, "error: "+want)
		rejectReport(t, strict, "warning: "+want)
	}
}

// A model in standard notation is unaffected, so -strict is a check and not a
// second dialect.
func TestStrictConformanceLeavesStandardNotationAlone(t *testing.T) {
	binary := buildCLI(t)
	const standard = `package Mission {
    state def Monitor {
        entry; then off;
        state off;
        state on;
        transition first off accept Signal then on;
    }
    attribute def Signal;
}
`
	for _, args := range [][]string{{"-validate"}, {"-validate", "-strict"}} {
		wantReport(t, check(t, binary, standard, args...), 0, "no errors")
	}
}

// -strict judges notation, not the model: the unbound-parameter advisory on well-formed
// SysML v2 stays a warning in either mode, at a bare call and a chain head alike.
func TestStrictConformanceKeepsUnboundParameterAdvisory(t *testing.T) {
	binary := buildCLI(t)
	const unbound = `package Arity {
    private import ScalarValues::*;
    calc def F { in x : Real; in y : Real; return : Real = x + y; }
    attribute plain : Real = F(1.0);
    attribute head : Real = F(1.0).result;
}
`
	for _, args := range [][]string{{"-validate"}, {"-validate", "-strict"}} {
		got := check(t, binary, unbound, args...)
		wantReport(t, got, 0, "no errors",
			"4:30: warning: F leaves parameter y unbound, so the call cannot be evaluated",
			"5:29: warning: F leaves parameter y unbound, so the call cannot be evaluated")
		rejectReport(t, got, "error:")
	}
}

// A notation error does not stop a check, so the run succeeds — but a run that
// reported an error must not also report that the model has none.
func TestValidateDoesNotCallAReportedErrorNone(t *testing.T) {
	binary := buildCLI(t)
	const bare = "package Q {\n    part def A;\n}\npackage P {\n    import Q::*;\n}\n"

	got := check(t, binary, bare, "-validate")
	wantReport(t, got, 0, "error: import without a visibility indicator",
		"no error that stops a check")
	rejectReport(t, got, ": no errors")
}

func TestKeywordAsNameReportsOnceInEitherMode(t *testing.T) {
	binary := buildCLI(t)
	const keywordName = "package P {\n    part def part;\n}\n"

	for _, tc := range []struct {
		status int
		args   []string
	}{
		{0, []string{"-validate"}},
		{2, []string{"-validate", "-strict"}},
	} {
		got := check(t, binary, keywordName, tc.args...)
		wantReport(t, got, tc.status, "error: \"part\" is a reserved keyword, not a name the ID terminal admits")
		rejectReport(t, got, "warning:")
	}
}
