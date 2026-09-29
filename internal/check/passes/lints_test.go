package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// lintDiags analyzes src under mode with every pass and returns the findings
// carrying code, failing on any error: a lint must never be one.
func lintDiags(t *testing.T, src, code string, mode diag.ConformanceMode) []diag.Diagnostic {
	t.Helper()
	const name = "lint.sysml"
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	all := AnalyzeWithOptions(name, source.KindSysML, root, nil, idx, Options{Conformance: mode})
	var out []diag.Diagnostic
	for _, d := range all {
		if d.Code == code {
			if d.Severity != diag.SeverityWarning || d.Blocking() {
				t.Errorf("%s: severity %v, want a non-blocking warning", code, d.Severity)
			}
			if d.Source != lintSource {
				t.Errorf("%s: source %q, want %q", code, d.Source, lintSource)
			}
			out = append(out, d)
		}
	}
	return out
}

// wantLints asserts, in both conformance modes, one finding of code per entry
// of wants, the message of each containing every substring of its entry.
func wantLints(t *testing.T, src, code string, wants ...[]string) {
	t.Helper()
	for _, mode := range []diag.ConformanceMode{diag.ConformanceDefault, diag.ConformanceStrict} {
		got := lintDiags(t, src, code, mode)
		if len(got) != len(wants) {
			t.Fatalf("%s (%s): got %d findings %+v, want %d", code, mode, len(got), got, len(wants))
		}
		for i, want := range wants {
			for _, part := range want {
				if !strings.Contains(got[i].Message, part) {
					t.Errorf("%s (%s): message %q, want it to contain %q", code, mode, got[i].Message, part)
				}
			}
		}
	}
}

func TestUndeclaredSignalLint(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		wants []string
	}{
		{"when names nothing", `package P {
	state def Machine {
		entry; then idle;
		state idle; state busy;
		transition first idle when Strat then busy;
	}
	part sender { action a { send new Start() to sender; } }
	attribute def Start;
}`, []string{"`when Strat` names no declaration", "did you mean Start"}},
		{"defer names nothing", `package P {
	state def Machine {
		entry; then idle;
		state idle { defer Pnig; }
		state busy;
		transition first idle when Ping then busy;
	}
	attribute def Ping;
}`, []string{"`defer Pnig` names no declaration", "did you mean Ping"}},
		{"suggests a sent signal", `package P {
	state def Machine {
		entry; then idle;
		state idle; state busy;
		transition first idle when sigAA then busy;
	}
	part sender { attribute sigA; action a { send sigA to sender; } }
}`, []string{"`when sigAA`", "sigA"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantLints(t, tc.src, CodeUndeclaredSignal, tc.wants)
		})
	}
}

func TestUndeclaredSignalLintSilent(t *testing.T) {
	cases := map[string]string{
		"visible declaration": `package P {
	attribute def Go;
	state def Machine {
		entry; then idle;
		state idle { defer Go; }
		state busy;
		transition first idle when Go then busy;
	}
}`,
		"sent by constructor": `package P {
	part def Sender { action a { send new Kick() to self; } }
	private import Q::*;
	state def Machine {
		entry; then idle;
		state idle { defer Kick; }
		state busy;
		transition first idle when Kick then busy;
	}
}
package Q { attribute def Kick; }`,
		"sent by name": `package P {
	state def Machine {
		entry; then waiting;
		state waiting; state pathA; state pathB;
		transition first waiting when sigA then pathA;
		transition first waiting when sigB then pathB;
	}
	part def Sender { attribute sigA; attribute sigB; action a { send sigA to self; send sigB to self; } }
}`,
		"sent payload typed": `package P {
	private import Q::*;
	state def Machine {
		entry; then idle;
		state idle; state busy;
		transition first idle when Beep then busy;
	}
	part def Sender { action a { send { in :>> payload : Beep; } } }
}
package Q { attribute def Beep; }`,
		"change trigger": `package P {
	state def Machine {
		attribute ready : ScalarValues::Boolean;
		entry; then idle;
		state idle; state busy;
		transition first idle when ready then busy;
	}
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			wantLints(t, src, CodeUndeclaredSignal)
		})
	}
}

// A signal sent in another workspace document silences the lint: the union is
// read across the batch, not per document.
func TestUndeclaredSignalLintReadsOtherDocuments(t *testing.T) {
	machine := `package M {
	state def Machine {
		entry; then idle;
		state idle; state busy;
		transition first idle when Remote then busy;
	}
}`
	sender := `package S { attribute Remote; part def Sender { action a { send Remote to self; } } }`
	idx := newTestIndex()
	mroot := parser.New(source.New("m.sysml", []byte(machine))).ParseFile()
	sroot := parser.New(source.New("s.sysml", []byte(sender))).ParseFile()
	idx.AddDocument("m.sysml", mroot)
	idx.AddDocument("s.sysml", sroot)
	for _, d := range Analyze("m.sysml", mroot, nil, idx) {
		if d.Code == CodeUndeclaredSignal {
			t.Fatalf("a signal another document sends was reported: %+v", d)
		}
	}
}

// A send invoking a calculation sends its value, typed by the calculation's
// result; one invoking anything else sends a signal by the invoked name.
func TestUndeclaredSignalLintInvokedSend(t *testing.T) {
	machine := `package M {
	state def Machine {
		entry; then idle;
		state idle; state busy;
		transition first idle when Ping then busy;
		transition first busy when Reading then idle;
		transition first busy when Halt then idle;
	}
}`
	sender := `package S {
	item def Reading;
	attribute def Halt;
	calc def Ping { return r : Reading; }
	part def Sender { action a { send Ping() to self; send Halt() to self; } }
}`
	idx := newTestIndex()
	mroot := parser.New(source.New("m.sysml", []byte(machine))).ParseFile()
	sroot := parser.New(source.New("s.sysml", []byte(sender))).ParseFile()
	idx.AddDocument("m.sysml", mroot)
	idx.AddDocument("s.sysml", sroot)
	var got []string
	for _, d := range Analyze("m.sysml", mroot, nil, idx) {
		if d.Code == CodeUndeclaredSignal {
			got = append(got, d.Message)
		}
	}
	if len(got) != 1 || !strings.Contains(got[0], "`when Ping`") {
		t.Fatalf("got %q, want one finding, on `when Ping`", got)
	}
}

func TestWithoutLints(t *testing.T) {
	diags := []diag.Diagnostic{
		{Code: CodeUndeclaredSignal, Source: lintSource},
		{Code: CodePortTypeMismatch, Source: lintSource},
		{Code: CodeUndeclaredSignal, Source: "name-resolution"},
	}
	got := WithoutLints(diags, map[string]bool{CodeUndeclaredSignal: true})
	if len(got) != 2 || got[0].Code != CodePortTypeMismatch || got[1].Source != "name-resolution" {
		t.Fatalf("got %+v", got)
	}
	if got := WithoutLints(diags, nil); len(got) != len(diags) {
		t.Fatalf("nothing disabled dropped %+v", got)
	}
	for _, code := range LintCodes() {
		if !IsLintCode(code) {
			t.Errorf("%s is not a lint code", code)
		}
	}
	if IsLintCode("unresolved") {
		t.Error("unresolved is not a lint")
	}
}

const portDefs = `
	item def Power; item def Fuel;
	port def PowerOut { out item p : Power; }
	port def FuelIn { in item f : Fuel; }
	port def PowerIn { in item p : Power; }
	port def Base { out item p : Power; }
	port def Derived :> Base;
	port def Sibling :> Base;
	port def Wide { out item p : Power; out item q : Fuel; }
	port def Other { out item o : Power; }
	port def OtherIn { in item o : Power; }
	interface def Mount { end x : Other; end y : PowerIn; flow x.o to y.p; }
	part def Source { port power : PowerOut; port base : Base; port derived : Derived; port sibling : Sibling; port conj : ~PowerOut; port wide : Wide; port other : Other; }
	part def Sink { port fuel : FuelIn; port powerIn : PowerIn; }
	interface def Link { end a : PowerOut; end b : PowerIn; }
	interface def HalfLink { end a : PowerOut; end b; }
	connection def Pipe { end a : PowerOut; end b : FuelIn; }
	interface def SubMount :> Mount { end x; }
`

func TestPortTypeMismatchLint(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"connect", `connect a.power to b.fuel;`, []string{"connection connects port power : PowerOut to port fuel : FuelIn", "~PowerOut"}},
		{"named connection", `connection c connect a.power to b.fuel;`, []string{"connection c connects port power : PowerOut"}},
		{"interface", `interface i connect a.power to b.fuel;`, []string{"interface i connects port power : PowerOut to port fuel : FuelIn"}},
		{"flow", `flow from a.power to b.fuel;`, []string{"flow connects port power : PowerOut to port fuel : FuelIn"}},
		{"features by other names", `connect a.other to b.powerIn;`, []string{"port other : Other to port powerIn : PowerIn"}},
		{"connection typed by a port-typed connection definition", `connection : Pipe connect a.power to b.fuel;`, []string{"port power : PowerOut to port fuel : FuelIn"}},
		{"interface with one port-typed end", `interface : HalfLink connect a.power to b.fuel;`, []string{"port power : PowerOut to port fuel : FuelIn"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P {" + portDefs + "\tpart s { part a : Source; part b : Sink; " + tc.body + " }\n}"
			wantLints(t, src, CodePortTypeMismatch, tc.want)
		})
	}
}

func TestPortTypeMismatchLintSilent(t *testing.T) {
	cases := map[string]string{
		"conjugate features":                   `connect a.power to b.powerIn;`,
		"typed interface":                      `interface : Link connect a.power to b.powerIn;`,
		"conjugated port":                      `connect a.power to a.conj;`,
		"same definition":                      `connect a.power to c.power;`,
		"specialization":                       `connect a.base to a.derived;`,
		"common definition":                    `connect a.derived to a.sibling;`,
		"flow between conjugates":              `flow from a.power to b.powerIn;`,
		"flow of items":                        `flow of Power from a.power.p to b.powerIn.p;`,
		"non-port ends":                        `connect a to b;`,
		"one port's features all conjugate":    `connect a.wide to b.powerIn;`,
		"typed by an interface with port ends": `interface : Mount connect a.other to b.powerIn;`,
		"port ends inherited by redefinition":  `interface : SubMount connect a.other to b.powerIn;`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			src := "package P {" + portDefs + "\tpart s { part a : Source; part b : Sink; part c : Source; " + body + " }\n}"
			wantLints(t, src, CodePortTypeMismatch)
		})
	}
}

// A literal sends a value, typed by its scalar type as the runtime names it.
func TestUndeclaredSignalLintLiteralSend(t *testing.T) {
	machine := `package M {
	state def Machine {
		entry; then idle;
		state idle; state busy;
		transition first idle when String then busy;
		transition first busy when Integer then idle;
		transition first busy when Real then idle;
		transition first busy when Boolean then idle;
	}
}`
	count := func(sender string) int {
		idx := newTestIndex()
		mroot := parser.New(source.New("m.sysml", []byte(machine))).ParseFile()
		sroot := parser.New(source.New("s.sysml", []byte(sender))).ParseFile()
		idx.AddDocument("m.sysml", mroot)
		idx.AddDocument("s.sysml", sroot)
		n := 0
		for _, d := range Analyze("m.sysml", mroot, nil, idx) {
			if d.Code == CodeUndeclaredSignal {
				n++
			}
		}
		return n
	}
	if n := count(`package S { part def Sender { action a { send "go" to self; } } }`); n != 3 {
		t.Fatalf("a string send left %d finding(s), want 3", n)
	}
	if n := count(`package S { part def Sender { action a { send "go" to self; send 1 to self; send 2.5 to self; send true to self; } } }`); n != 0 {
		t.Fatalf("literal sends left %d finding(s), want none", n)
	}
	if n := count(`package S { private import ScalarValues::*; part def Sender { attribute r : Real = 1.5; action a { send 1 + 2 to self; send r * 2.0 to self; send not true to self; send "a" + "b" to self; } } }`); n != 0 {
		t.Fatalf("computed scalar sends left %d finding(s), want none", n)
	}
}
