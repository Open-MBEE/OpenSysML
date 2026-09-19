package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// sendDiags is every diagnostic the full registry reports for src, filtered to
// the codes the send-action rule and the invocation rule it activates report.
func sendDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range analyzeAll(t, "send.sysml", src) {
		switch d.Code {
		case CodeSendPayloadMissing, CodeSendToPort, CodeSendReceiverNotOccurrence,
			CodeSendSenderNotOccurrence, "invocation-not-behavior":
			out = append(out, d)
		}
	}
	return out
}

// sendPrelude declares the payload types, ports and receivers the fixtures
// below send between.
const sendPrelude = `
	attribute def Sig;
	item def Msg;
	action def Ping;
	port def PD;
	part def Receiver { port p : PD; part inner : Receiver; }
`

// assertOneSendDiag checks that got is the one diagnostic code at severity,
// whose span reads at, and whose message mentions every want.
func assertOneSendDiag(t *testing.T, src string, got []diag.Diagnostic, code string, severity diag.Severity, at string, wants ...string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("expected one %s diagnostic, got %+v", code, got)
	}
	d := got[0]
	if d.Code != code || d.Severity != severity {
		t.Errorf("got code %q severity %v, want %s at severity %v", d.Code, d.Severity, code, severity)
	}
	if spanned := strings.TrimSpace(src[d.Span.Offset:d.Span.End()]); spanned != at {
		t.Errorf("reported at %q, want %q: %q", spanned, at, d.Message)
	}
	for _, want := range wants {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q does not mention %q", d.Message, want)
		}
	}
}

// The payload of a send is an expression the type checker sees: instantiating
// a definition that is not a behavior is reported where the reference does.
func TestSendPayloadInvokingNonBehaviorIsReported(t *testing.T) {
	cases := map[string]struct{ src, at string }{
		"attribute def": {at: "Sig", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver;
				action s send Sig() to r;
			} }`},
		"item def": {at: "Msg", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver;
				action s send Msg() via r.p;
			} }`},
		"bare send in an action usage": {at: "Sig", src: `package P {` + sendPrelude + `
			part def V { part r : Receiver; action a {
				send Sig() to r;
			} } }`},
		"state entry action": {at: "Sig", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver;
				entry send Sig() to r;
			} }`},
		"transition effect": {at: "Sig", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver; state a; state b;
				transition first a do send Sig() to r then b;
			} }`},
		"body payload binding": {at: "Sig", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver;
				action s send to r { in :>> payload = Sig(); }
			} }`},
		"nested action body": {at: "Sig", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver; action outer { action inner {
				send Sig() to r;
			} } } }`},
		"succession body": {at: "Sig", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver; action a; action b;
				then b { send Sig() to r; }
			} }`},
		"body payload binding of a bare send in a definition": {at: "Sig", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver;
				send to r { in :>> payload = Sig(); }
			} }`},
		"body payload binding in a branch": {at: "Sig", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver;
				if true { send to r { in :>> payload = Sig(); } }
			} }`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertOneSendDiag(t, tc.src, sendDiags(t, tc.src), "invocation-not-behavior", diag.SeverityError, tc.at, "Must invoke a behavior")
		})
	}
}

// Sending `to` a port is reported as a warning naming the port, whether the
// port is referenced directly, through a feature chain, or after a `via` port.
func TestSendToPortWarns(t *testing.T) {
	cases := map[string]struct{ src, at string }{
		"own port": {at: "q", src: `package P {` + sendPrelude + `
			part def V { port q : PD; action a {
				send Ping() to q;
			} } }`},
		"chained port": {at: "r.p", src: `package P {` + sendPrelude + `
			part def V { part r : Receiver; action a {
				send Ping() to r.p;
			} } }`},
		"deeply chained port": {at: "r.inner.p", src: `package P {` + sendPrelude + `
			part def V { part r : Receiver; action a {
				send Ping() to r.inner.p;
			} } }`},
		"via then to a port": {at: "r.p", src: `package P {` + sendPrelude + `
			part def V { port q : PD; part r : Receiver; action a {
				send Ping() via q to r.p;
			} } }`},
		"transition effect": {at: "q", src: `package P {` + sendPrelude + `
			state def M { port q : PD; state a; state b;
				transition first a do send Ping() to q then b;
			} }`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertOneSendDiag(t, tc.src, sendDiags(t, tc.src), CodeSendToPort, diag.SeverityWarning, tc.at, "'via'", "'to'")
		})
	}
}

// A `to` or `via` argument binds an Occurrence parameter of the send, so a
// feature whose types are disjoint from Occurrence is reported, as the
// reference does for the implicit binding, at the argument.
func TestSendArgumentNotAnOccurrenceWarns(t *testing.T) {
	cases := map[string]struct{ src, code, at, want string }{
		"attribute receiver": {code: CodeSendReceiverNotOccurrence, at: "a", want: "Real", src: `package P {` + sendPrelude + `
			part def V { attribute a : ScalarValues::Real; action s {
				send Ping() to a;
			} } }`},
		"chained attribute receiver": {code: CodeSendReceiverNotOccurrence, at: "r.mass", want: "Real", src: `package P {` + sendPrelude + `
			part def Weighed { attribute mass : ScalarValues::Real; }
			part def V { part r : Weighed; action s {
				send Ping() to r.mass;
			} } }`},
		"attribute sender": {code: CodeSendSenderNotOccurrence, at: "a", want: "Real", src: `package P {` + sendPrelude + `
			part def V { attribute a : ScalarValues::Real; part r : Receiver; action s {
				send Ping() via a to r;
			} } }`},
		"inherited attribute receiver": {code: CodeSendReceiverNotOccurrence, at: "a", want: "Real", src: `package P {` + sendPrelude + `
			part def Base { attribute a : ScalarValues::Real; }
			part def V :> Base { action s {
				send Ping() to a;
			} } }`},
		"untyped redefinition of an attribute receiver": {code: CodeSendReceiverNotOccurrence, at: "a", want: "Real", src: `package P {` + sendPrelude + `
			part def Base { attribute a : ScalarValues::Real; }
			part def V :> Base { attribute :>> a; action s {
				send Ping() to a;
			} } }`},
		"renamed redefinition of an attribute sender": {code: CodeSendSenderNotOccurrence, at: "b", want: "Real", src: `package P {` + sendPrelude + `
			part def Base { attribute a : ScalarValues::Real; }
			part def V :> Base { attribute b :>> a; part r : Receiver; action s {
				send Ping() via b to r;
			} } }`},
		"subsetting an attribute receiver": {code: CodeSendReceiverNotOccurrence, at: "b", want: "Real", src: `package P {` + sendPrelude + `
			part def V { attribute a : ScalarValues::Real; attribute b :> a; action s {
				send Ping() to b;
			} } }`},
		"untyped attribute receiver": {code: CodeSendReceiverNotOccurrence, at: "a", want: "DataValue", src: `package P {` + sendPrelude + `
			part def V { attribute a; action s {
				send Ping() to a;
			} } }`},
		"user attribute def receiver": {code: CodeSendReceiverNotOccurrence, at: "sig", want: "Sig", src: `package P {` + sendPrelude + `
			part def V { attribute sig : Sig; action s {
				send Ping() to sig;
			} } }`},
		"integer literal receiver": {code: CodeSendReceiverNotOccurrence, at: "42", want: "Natural", src: `package P {` + sendPrelude + `
			part def V { action s {
				send Ping() to 42;
			} } }`},
		"string literal receiver": {code: CodeSendReceiverNotOccurrence, at: `"console"`, want: "String", src: `package P {` + sendPrelude + `
			part def V { action s {
				send Ping() to "console";
			} } }`},
		"arithmetic receiver": {code: CodeSendReceiverNotOccurrence, at: "1 + 2", want: "Natural", src: `package P {` + sendPrelude + `
			part def V { action s {
				send Ping() to 1 + 2;
			} } }`},
		"comparison receiver": {code: CodeSendReceiverNotOccurrence, at: "a > 1", want: "Boolean", src: `package P {` + sendPrelude + `
			part def V { attribute a : ScalarValues::Real; action s {
				send Ping() to a > 1;
			} } }`},
		"constructed attribute receiver": {code: CodeSendReceiverNotOccurrence, at: "new Sig()", want: "Sig", src: `package P {` + sendPrelude + `
			part def V { action s {
				send Ping() to new Sig();
			} } }`},
		"scalar calculation result receiver": {code: CodeSendReceiverNotOccurrence, at: "mass()", want: "Real", src: `package P {` + sendPrelude + `
			calc def mass { return : ScalarValues::Real; }
			part def V { action s {
				send Ping() to mass();
			} } }`},
		"literal sender": {code: CodeSendSenderNotOccurrence, at: "true", want: "Boolean", src: `package P {` + sendPrelude + `
			part def V { part r : Receiver; action s {
				send Ping() via true to r;
			} } }`},
		"constructed attribute sender": {code: CodeSendSenderNotOccurrence, at: "new Sig()", want: "Sig", src: `package P {` + sendPrelude + `
			part def V { part r : Receiver; action s {
				send Ping() via new Sig() to r;
			} } }`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertOneSendDiag(t, tc.src, sendDiags(t, tc.src), tc.code, diag.SeverityWarning, tc.at, "occurrence", tc.want)
		})
	}
}

// A send that is a state subaction or a transition effect must carry a
// payload, whether written bare or as the one statement of an action node.
func TestSendSubactionWithoutPayloadIsReported(t *testing.T) {
	cases := map[string]struct{ src, at string }{
		"entry": {at: "send to r;", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver;
				entry send to r;
			} }`},
		"do action node": {at: "send to r;", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver;
				do action f send to r;
			} }`},
		"exit with a body": {at: "send to r { doc /* nothing bound */ }", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver;
				exit send to r { doc /* nothing bound */ }
			} }`},
		"transition effect": {at: "send to r", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver; state a; state b;
				transition first a do send to r then b;
			} }`},
		"transition effect action node": {at: "send to r", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver; state a; state b;
				transition first a do action g send to r then b;
			} }`},
		"nested state entry": {at: "send to r;", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver; state outer { state inner {
				entry send to r;
			} } } }`},
		"exhibited state": {at: "send to r;", src: `package P {` + sendPrelude + `
			part def V { part r : Receiver; exhibit state m {
				entry send to r;
			} } }`},
		"body binds an input that is not the payload": {at: "send to r { in delay = 3; }", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver;
				entry send to r { in delay = 3; }
			} }`},
		"body binds only the sender": {at: "send to r { in :>> sender = r; }", src: `package P {` + sendPrelude + `
			state def M { part r : Receiver;
				entry send to r { in :>> sender = r; }
			} }`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertOneSendDiag(t, tc.src, sendDiags(t, tc.src), CodeSendPayloadMissing, diag.SeverityError, tc.at, "payload", "send new Msg() to receiver")
		})
	}
}

// Every well-formed send shape stays silent: behaviors and constructors as the
// payload, `via` a port, `to` an occurrence directly or through a chain, a
// payload bound in the body, and a payload-less send outside a state.
func TestSendWellFormedShapesAreSilent(t *testing.T) {
	cases := map[string]string{
		"behavior via port": `package P {` + sendPrelude + `
			part def V { port q : PD; action a { send Ping() via q; } } }`,
		"constructor via port": `package P {` + sendPrelude + `
			part def V { port q : PD; action a { send new Sig() via q; } } }`,
		"constructor with named arguments": `package P {` + sendPrelude + `
			item def Frame { attribute count : ScalarValues::Integer; }
			part def V { port q : PD; action a { send new Frame(count = 3) via q; } } }`,
		"item to part": `package P {` + sendPrelude + `
			part def V { part r : Receiver; item m : Msg; action a { send m to r; } } }`,
		"to a chained occurrence": `package P {` + sendPrelude + `
			part def V { part r : Receiver; action a { send Ping() to r.inner; } } }`,
		"via port to part": `package P {` + sendPrelude + `
			part def V { port q : PD; part r : Receiver; action a { send Ping() via q to r; } } }`,
		"body binds the payload": `package P {` + sendPrelude + `
			part def V { part r : Receiver; action a { send to r { in :>> payload = new Sig(); } } } }`,
		"body binds the payload of a bare send in a definition": `package P {` + sendPrelude + `
			action def A { part r : Receiver; send to r { in :>> payload = new Sig(); } } }`,
		"body binds the payload in a branch": `package P {` + sendPrelude + `
			action def A { part r : Receiver; if true { send to r { in :>> payload = new Sig(); } } } }`,
		"body binds the payload in a succession body": `package P {` + sendPrelude + `
			action def A { part r : Receiver; action a; action b;
				then b { send to r { in :>> payload = new Sig(); } } } }`,
		"body binds the payload by position": `package P {` + sendPrelude + `
			state def M { part r : Receiver; entry send { in msg = new Sig(); } } }`,
		"body declares the payload parameter beside an argument": `package P {` + sendPrelude + `
			part def V { port q : PD; item m : Msg; action a send m via q { in m : Msg; } } }`,
		"state entry with payload": `package P {` + sendPrelude + `
			state def M { part r : Receiver; entry send Ping() to r; } }`,
		"transition effect with a bound payload": `package P {` + sendPrelude + `
			state def M { part r : Receiver; state a; state b;
				transition first a do send to r { in :>> payload = new Sig(); } then b; } }`,
		"transition effect action with several statements": `package P {` + sendPrelude + `
			state def M { part r : Receiver; state a; state b;
				transition first a do action g { send to r; send to r; } then b; } }`,
		"braced subaction with one payload-less send": `package P {` + sendPrelude + `
			state def M { part r : Receiver; state a; state b;
				entry action e { send to r; }
				do action d { send to r; }
				exit action x { send to r; }
				transition first a do action g { send to r; } then b; } }`,
		"payload-less send in an action def": `package P {` + sendPrelude + `
			action def A { part r : Receiver; action s send to r; } }`,
		"action parameter as payload": `package P {` + sendPrelude + `
			action def A { in item m : Msg; part r : Receiver; action s send m to r; } }`,
		"to an item": `package P {` + sendPrelude + `
			part def V { item box : Msg; action a { send Ping() to box; } } }`,
		"to an untyped part": `package P {` + sendPrelude + `
			part def V { part r; action a { send Ping() to r; } } }`,
		"to an untyped reference": `package P {` + sendPrelude + `
			part def V { ref r; action a { send Ping() to r; } } }`,
		"to a bare member": `package P {` + sendPrelude + `
			part def V { target; action a { send Ping() to target; } } }`,
		"to a bare member of an action def": `package P {` + sendPrelude + `
			action def A { target; action s send Ping() to target; } }`,
		"to an annotated bare member": `package P {` + sendPrelude + `
			metadata def Tag;
			part def V { #Tag target; action a { send Ping() to target; } } }`,
		"via a bare member": `package P {` + sendPrelude + `
			part def V { sender; part r : Receiver; action a { send Ping() via sender to r; } } }`,
		"to an inherited part": `package P {` + sendPrelude + `
			part def Base { part r : Receiver; }
			part def V :> Base { action a { send Ping() to r; } } }`,
		"to a redefined part": `package P {` + sendPrelude + `
			part def Base { part r; }
			part def V :> Base { part :>> r : Receiver; action a { send Ping() to r; } } }`,
		"to an untyped redefinition of a part": `package P {` + sendPrelude + `
			part def Base { part r : Receiver; }
			part def V :> Base { part :>> r; action a { send Ping() to r; } } }`,
		"to an untyped redefinition of a reference": `package P {` + sendPrelude + `
			part def Base { ref r; }
			part def V :> Base { ref :>> r; action a { send Ping() to r; } } }`,
		"via an untyped redefinition of a port": `package P {` + sendPrelude + `
			part def Base { port q : PD; }
			part def V :> Base { port :>> q; action a { send Ping() via q; } } }`,
		"via an inherited port": `package P {` + sendPrelude + `
			part def Base { port q : PD; }
			part def V :> Base { action a { send Ping() via q; } } }`,
		"via a chained port": `package P {` + sendPrelude + `
			part def V { part r : Receiver; action a { send Ping() via r.inner.p; } } }`,
		"to an action": `package P {` + sendPrelude + `
			part def V { action target : Ping; action a { send Ping() to target; } } }`,
		"to a definition is the feature-reference rule's": `package P {` + sendPrelude + `
			part def V { action a { send Ping() to Sig; } } }`,
		"to a constructed item": `package P {` + sendPrelude + `
			part def V { action a { send Ping() to new Msg(); } } }`,
		"to a constructed part": `package P {` + sendPrelude + `
			part def V { action a { send Ping() to new Receiver(); } } }`,
		"via a constructed port": `package P {` + sendPrelude + `
			part def V { part r : Receiver; action a { send Ping() via new PD() to r; } } }`,
		"to a constructed package is the instantiation rule's": `package P {` + sendPrelude + `
			package Q; part def V { action a { send Ping() to new Q(); } } }`,
		"via a constructed package is the instantiation rule's": `package P {` + sendPrelude + `
			package Q; part def V { part r : Receiver; action a { send Ping() via new Q() to r; } } }`,
		"to an occurrence-valued calculation": `package P {` + sendPrelude + `
			calc def pick { return : Receiver; }
			part def V { action a { send Ping() to pick(); } } }`,
		"to a conditional the checker cannot type": `package P {` + sendPrelude + `
			part def V { part r : Receiver; part s : Receiver; attribute hot : ScalarValues::Boolean;
				action a { send Ping() to if hot ? r else s; } } }`,
		"to self": `package P {` + sendPrelude + `
			part def V { action a { send Ping() to self; } } }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if got := sendDiags(t, src); len(got) != 0 {
				t.Errorf("expected no send diagnostics, got %+v", got)
			}
		})
	}
}

// A constraint body carries action nodes: a send written in an asserted, assumed
// or required constraint's body is checked like any other; a well-formed one is silent.
func TestSendInConstraintBodiesIsChecked(t *testing.T) {
	hosts := map[string]func(send string) string{
		"asserted constraint": func(send string) string {
			return `package P {` + sendPrelude + `
				part def V { port q : PD; assert constraint c { ` + send + ` true } } }`
		},
		"assumed constraint": func(send string) string {
			return `package P {` + sendPrelude + `
				requirement def R { port q : PD; assume constraint { ` + send + ` true } } }`
		},
		"required constraint": func(send string) string {
			return `package P {` + sendPrelude + `
				requirement def R { port q : PD; require constraint { ` + send + ` true } } }`
		},
		"nested assertion": func(send string) string {
			return `package P {` + sendPrelude + `
				part def V { port q : PD; constraint c { assert constraint { ` + send + ` true } true } } }`
		},
	}
	for name, host := range hosts {
		t.Run(name, func(t *testing.T) {
			bad := host(`send Sig() to q;`)
			got := sendDiags(t, bad)
			if len(got) != 2 {
				t.Fatalf("expected the payload and receiver diagnostics, got %+v", got)
			}
			codes := map[string]bool{}
			for _, d := range got {
				codes[d.Code] = true
			}
			if !codes["invocation-not-behavior"] || !codes[CodeSendToPort] {
				t.Errorf("got codes %v, want invocation-not-behavior and %s", codes, CodeSendToPort)
			}
			if got := sendDiags(t, host(`send Ping() via q;`)); len(got) != 0 {
				t.Errorf("expected no send diagnostics on the valid send, got %+v", got)
			}
		})
	}
}

// The payload parameter a send body redefines resolves wherever the send is
// written: on an action node, bare in a definition, in a branch or a succession.
func TestSendBodyPayloadResolvesInEveryHost(t *testing.T) {
	cases := map[string]string{
		"action node": `package P {` + sendPrelude + `
			action def A { part r : Receiver; action s send to r { in :>> payload = new Sig(); } } }`,
		"bare in a definition": `package P {` + sendPrelude + `
			action def A { part r : Receiver; send to r { in :>> payload = new Sig(); } } }`,
		"in a branch": `package P {` + sendPrelude + `
			action def A { part r : Receiver; if true { send to r { in :>> payload = new Sig(); } } } }`,
		"in a succession body": `package P {` + sendPrelude + `
			action def A { part r : Receiver; action a; action b;
				then b { send to r { in :>> payload = new Sig(); } } } }`,
		"state entry": `package P {` + sendPrelude + `
			state def M { part r : Receiver; entry send to r { in :>> payload = new Sig(); } } }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			for _, d := range analyzeAll(t, "send.sysml", src) {
				if d.Severity == diag.SeverityError {
					t.Errorf("unexpected error: %s %s", d.Code, d.Message)
				}
			}
		})
	}
}

// A payload the body binds is typed by the send rule itself, so it is still
// reported when an unrelated unresolved name elsewhere in the document gates
// the document-wide type pass — once, wherever the send is written.
func TestSendBodyPayloadCheckedWhenTypePassIsGated(t *testing.T) {
	const unrelated = ` part def Broken : Nowhere; `
	cases := map[string]string{
		"action node": `package P {` + sendPrelude + unrelated + `
			action def A { part r : Receiver; action s send to r { in :>> payload = Sig(); } } }`,
		"bare in a definition": `package P {` + sendPrelude + unrelated + `
			action def A { part r : Receiver; send to r { in :>> payload = Sig(); } } }`,
		"state entry": `package P {` + sendPrelude + unrelated + `
			state def M { part r : Receiver; entry send to r { in :>> payload = Sig(); } } }`,
		"transition effect": `package P {` + sendPrelude + unrelated + `
			state def M { part r : Receiver; state a; state b;
				transition first a do send to r { in :>> payload = Sig(); } then b; } }`,
		"positional payload": `package P {` + sendPrelude + unrelated + `
			state def M { part r : Receiver; entry send { in msg = Sig(); } } }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			var unresolved []diag.Diagnostic
			for _, d := range analyzeAll(t, "send.sysml", src) {
				if d.Code == "unresolved" {
					unresolved = append(unresolved, d)
				}
			}
			if len(unresolved) != 1 || !strings.Contains(unresolved[0].Message, "Nowhere") {
				t.Fatalf("expected one unresolved reference to Nowhere, got %+v", unresolved)
			}
			assertOneSendDiag(t, src, sendDiags(t, src), "invocation-not-behavior", diag.SeverityError, "Sig", "Must invoke a behavior")
		})
	}
}

// A send whose arguments fail to resolve is left to the name-resolution tier:
// the rule does not pile a second diagnostic on an unresolved receiver, nor on
// an unresolved payload the body binds.
func TestSendGatesOnUnresolvedArguments(t *testing.T) {
	cases := map[string]struct{ src, unresolved string }{
		"receiver": {unresolved: "nowhere", src: `package P {` + sendPrelude + `
			action def A { action s send Ping() to nowhere; } }`},
		"body-bound payload": {unresolved: "Missing", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver; action s send to r { in :>> payload = Missing(); } } }`},
		"body-bound payload of a bare send": {unresolved: "Missing", src: `package P {` + sendPrelude + `
			action def A { part r : Receiver; send to r { in :>> payload = Missing(); } } }`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var unresolved []diag.Diagnostic
			for _, d := range analyzeAll(t, "send.sysml", tc.src) {
				if d.Code == "unresolved" {
					unresolved = append(unresolved, d)
				}
			}
			if len(unresolved) != 1 || !strings.Contains(unresolved[0].Message, tc.unresolved) {
				t.Fatalf("expected one unresolved reference to %s, got %+v", tc.unresolved, unresolved)
			}
			if got := sendDiags(t, tc.src); len(got) != 0 {
				t.Errorf("expected the send rule to stay silent on an unresolved argument, got %+v", got)
			}
		})
	}
}
