package passes

import "testing"

const messageTypingPrelude = `
item def Note;
part def Sender { event occurrence emit; }
part def Receiver { event occurrence receive; }
abstract flow def Memo;
part def Exchange {
	part sender : Sender;
	part receiver : Receiver;
`

// A message is a flow usage (SysML v2 §8.4.12.2), so `message m : T` is judged
// by validateFlowUsageType like `flow f : T`: an item definition is not an
// interaction, in every spelling of the declaration.
func TestMessageTypedByItemDefRejected(t *testing.T) {
	for _, decl := range []string{
		"message m : Note;",
		"message m : Note from sender.emit to receiver.receive;",
		"message : Note from sender.emit to receiver.receive;",
		"message m : Note of Note from sender.emit to receiver.receive;",
		"message m : $::Note from sender.emit to receiver.receive;",
		"flow f : Note;",
	} {
		src := messageTypingPrelude + decl + " }"
		diags := typeDiags(t, src)
		if len(diags) != 1 {
			t.Fatalf("%s: expected one type diagnostic, got %v", decl, diags)
		}
		if diags[0].Code != "usage-typing" ||
			diags[0].Message != "A flow usage must be typed by flow definitions." {
			t.Errorf("%s: got %s %q", decl, diags[0].Code, diags[0].Message)
		}
	}
}

// A message typed by a flow definition, or naming its payload type with `of`
// instead of a typing, draws nothing from the rule.
func TestMessageTypedByFlowDefOrPayloadOnlyAccepted(t *testing.T) {
	for _, decl := range []string{
		"message m : Memo from sender.emit to receiver.receive;",
		"message m of Note from sender.emit to receiver.receive;",
		"message m of note : Note from sender.emit to receiver.receive;",
		"message sender.emit to receiver.receive;",
	} {
		src := messageTypingPrelude + decl + " }"
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", decl, diags)
		}
	}
}
