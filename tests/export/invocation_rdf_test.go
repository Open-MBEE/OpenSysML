package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

const namedInvocations = `package Calls {
    private import ScalarValues::*;
    function twice {
        in x : Integer;
        return : Integer = x + x;
    }
    function pick {
        in a : Integer;
        in b : Integer;
        return : Integer = a;
    }
    class Sensor {
        feature reading : Integer;
        feature condition : Boolean;
        feature signal : Sensor;
    }
    feature s : Sensor;
    feature doubled : Integer = twice(s.reading);
    feature chosen : Integer = pick(1, s.reading);
    feature named : Integer = pick(a = 2, b = 3);
    feature chained : Boolean = not s.signal.condition();
    feature piped : Integer = s.reading->twice();
    feature made : Sensor = new Sensor();
}
`

// invocationTurtle converts the invocation model to Turtle without its source text.
func invocationTurtle(t *testing.T) []byte {
	t.Helper()
	turtle, err := convert.Convert("calls.kerml", []byte(namedInvocations), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return withoutSourceText(t, turtle)
}

// An invocation is written from sysml:function, its receiver and its arguments
// alone: by name, positionally and by argument name, through `->`, as a
// constructor, and as the feature chain it applies to.
func TestNamedFunctionInvocationsComeBackFromTheGraphAlone(t *testing.T) {
	stripped := invocationTurtle(t)
	back, err := convert.Convert("calls.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the graph alone: %v", err)
	}
	if string(back) != namedInvocations {
		t.Errorf("invocations were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", namedInvocations, back)
	}
	again, err := convert.Convert("calls.kerml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(withoutSourceText(t, again)) != string(stripped) {
		t.Errorf("the rebuilt notation states another graph:\n%s", firstLineDifference(stripped, withoutSourceText(t, again)))
	}
}

// An invocation whose function the graph does not define is refused, never
// written under a guessed name.
func TestInvocationOfAnUndefinedFunctionIsRefused(t *testing.T) {
	stripped := string(invocationTurtle(t))
	if !strings.Contains(stripped, "sysml:function elmt:Calls__twice ;") {
		t.Fatalf("the graph does not link the invoked function:\n%s", stripped)
	}
	// Both statements of the callee, collapsed and the Membership's member, move together.
	dangling := strings.ReplaceAll(stripped, "sysml:function elmt:Calls__twice ;", "sysml:function elmt:Calls__nowhere ;")
	dangling = strings.ReplaceAll(dangling, "_pfunction\" ;\n    sysml:memberElement elmt:Calls__twice ;", "_pfunction\" ;\n    sysml:memberElement elmt:Calls__nowhere ;")
	_, err := convert.Convert("calls.ttl", []byte(dangling), convert.FormatTurtle, convert.FormatSysML)
	if err == nil {
		t.Fatal("an invocation of an undefined function was written")
	}
	if !strings.Contains(err.Error(), `no element with id "Calls__nowhere"`) {
		t.Errorf("the refusal does not name the missing function: %v", err)
	}
}

// The callee is stated twice, by the collapsed sysml:function and by the owned
// Membership the pilot writes; when the two disagree the graph is refused, not guessed at.
func TestInvocationCalleeDisagreementIsRefused(t *testing.T) {
	stripped := string(invocationTurtle(t))
	const membership = "expr:Calls__doubled_pvalue_pfunction\n    a sysml:Membership ;\n    sysml:elementId \"Calls__doubled_pvalue_pfunction\" ;\n    sysml:memberElement elmt:Calls__twice ;"
	if !strings.Contains(stripped, membership) {
		t.Fatalf("the graph does not own the callee membership:\n%s", stripped)
	}
	disagreeing := strings.Replace(stripped, membership, strings.Replace(membership, "elmt:Calls__twice", "elmt:Calls__Sensor", 1), 1)
	_, err := convert.Convert("calls.ttl", []byte(disagreeing), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error is %T, want *export.UnsupportedError: %v", err, err)
	}
	if !strings.Contains(err.Error(), "Calls__doubled_pvalue_pfunction") {
		t.Errorf("the refusal does not name the disagreeing membership: %v", err)
	}
}

// The pilot's shape alone, an owned Membership whose member is the function
// and no collapsed sysml:function, names the callee.
func TestInvocationCalleeMembershipAloneIsRead(t *testing.T) {
	stripped := string(invocationTurtle(t))
	normative := withoutTriples(t, []byte(stripped), "sysml:function")
	back, err := convert.Convert("calls.ttl", normative, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("from the membership alone: %v", err)
	}
	if string(back) != namedInvocations {
		t.Errorf("invocations were not rebuilt from the memberships:\n--- want ---\n%s--- got ---\n%s", namedInvocations, back)
	}
}

// An invocation that names no function and applies to no feature chain has
// nothing to write ahead of its arguments, so it is refused.
func TestInvocationWithoutAFunctionIsRefused(t *testing.T) {
	stripped := string(invocationTurtle(t))
	if !strings.Contains(stripped, "sysml:function elmt:Calls__twice ;\n") {
		t.Fatalf("the graph does not link the invoked function:\n%s", stripped)
	}
	nameless := strings.Replace(stripped, "    sysml:function elmt:Calls__twice ;\n", "", 1)
	nameless = strings.Replace(nameless, "    sysml:elementId \"Calls__doubled_pvalue_pfunction\" ;\n    sysml:memberElement elmt:Calls__twice ;\n", "    sysml:elementId \"Calls__doubled_pvalue_pfunction\" ;\n", 1)
	_, err := convert.Convert("calls.ttl", []byte(nameless), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error is %T, want *export.UnsupportedError: %v", err, err)
	}
	if !strings.Contains(err.Error(), "an invocation names the function it invokes") {
		t.Errorf("the refusal does not say what is missing: %v", err)
	}
}
