package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
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
	turtle, err := export.Convert("calls.kerml", []byte(namedInvocations), export.FormatSysML, export.FormatTurtle)
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
	back, err := export.Convert("calls.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the graph alone: %v", err)
	}
	if string(back) != namedInvocations {
		t.Errorf("invocations were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", namedInvocations, back)
	}
	again, err := export.Convert("calls.kerml", back, export.FormatSysML, export.FormatTurtle)
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
	dangling := strings.ReplaceAll(stripped, "sysml:function elmt:Calls__twice ;", "sysml:function elmt:Calls__nowhere ;")
	_, err := export.Convert("calls.ttl", []byte(dangling), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatal("an invocation of an undefined function was written")
	}
	if !strings.Contains(err.Error(), `no element with id "Calls__nowhere"`) {
		t.Errorf("the refusal does not name the missing function: %v", err)
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
	_, err := export.Convert("calls.ttl", []byte(nameless), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error is %T, want *export.UnsupportedError: %v", err, err)
	}
	if !strings.Contains(err.Error(), "an invocation names the function it invokes") {
		t.Errorf("the refusal does not say what is missing: %v", err)
	}
}
