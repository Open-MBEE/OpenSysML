package view

import (
	"slices"
	"strings"
	"testing"
)

// The occurrences an interaction declares are its lifelines, flat and in
// declaration order, and each message runs between the lifelines its events
// belong to.
func TestSequenceRenderingDrawsLifelinesAndMessages(t *testing.T) {
	rendering := render(t, "sequence.sysml", "SequenceViews::pubSubView")
	if rendering.Kind != KindSequence {
		t.Fatalf("kind = %q, want %q", rendering.Kind, KindSequence)
	}
	if got := lifelineNames(rendering); !slices.Equal(got, []string{"producer", "server", "consumer"}) {
		t.Errorf("lifelines = %v, want the parts the interaction declares in order", got)
	}
	for _, node := range rendering.Roots {
		if len(node.Children) != 0 {
			t.Errorf("lifeline %s holds %d children; a sequence diagram nests nothing", node.Name, len(node.Children))
		}
		if !node.Origin.Located() {
			t.Errorf("lifeline %s carries no origin", node.Name)
		}
	}
	// The successions between the events state that the subscription precedes the
	// publication, against the order the messages are declared in.
	if got := messageLabels(rendering); !slices.Equal(got, []string{"subscribe_message", "publish_message", "deliver_message"}) {
		t.Errorf("messages = %v, want one per message usage in the stated order", got)
	}
	for _, edge := range rendering.Edges {
		if edge.Kind != EdgeFlow {
			t.Errorf("message %q is an edge of kind %v, want a flow", edge.Label, edge.Kind)
		}
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none: the interaction is shown whole", rendering.Notices)
	}
}

// The short name of the standard view definition states the same kind; an
// anonymous message is labeled with the payload it carries rather than left
// blank; and an action the interaction performs is behavior rather than a
// participant, so it is no lifeline and its own flows are reported.
func TestSequenceRenderingFromTheShortViewName(t *testing.T) {
	rendering := render(t, "sequence-vehicle.sysml", "VehicleSequenceViews::startVehicleView")
	if rendering.Kind != KindSequence {
		t.Fatalf("kind = %q, want %q", rendering.Kind, KindSequence)
	}
	if got := lifelineNames(rendering); !slices.Equal(got, []string{"driver", "vehicle"}) {
		t.Errorf("lifelines = %v, want the parts only: the action performed is not a participant", got)
	}
	labels := messageLabels(rendering)
	for _, want := range []string{"of ignitionCmd : IgnitionCmd", "of es : EngineStatus"} {
		if !slices.Contains(labels, want) {
			t.Errorf("messages = %v, want an anonymous message labeled %q", labels, want)
		}
	}
	// The flows written inside the performed action are its action flow; they are
	// reported rather than drawn on a lifeline of their own.
	for _, edge := range rendering.Edges {
		if edge.From == edge.To {
			t.Errorf("message %q is drawn as a self-message; the action's own flows are not messages", edge.Label)
		}
	}
	if len(rendering.Notices) != 1 ||
		!strings.Contains(rendering.Notices[0], "the flows in perform action startVehicle are its action flow") {
		t.Errorf("notices = %v, want the performed action's flows reported", rendering.Notices)
	}
}

// A message end naming a feature nested in a lifeline is drawn on that lifeline,
// and everything a sequence rendering cannot show is reported.
func TestSequenceRenderingReportsWhatItCannotShow(t *testing.T) {
	rendering := render(t, "sequence-notices.sysml", "NoticeViews::handshakeView")
	if got := lifelineNames(rendering); !slices.Equal(got, []string{"caller", "callee"}) {
		t.Errorf("lifelines = %v, want the parts only", got)
	}
	if got := messageLabels(rendering); !slices.Equal(got, []string{"echo", "of Ping"}) {
		t.Errorf("messages = %v, want the self-message and the port-to-port flow", got)
	}
	ping, echo := edgeLabeled(t, rendering, "of Ping"), edgeLabeled(t, rendering, "echo")
	if ping.From != rendering.Roots[0].ID || ping.To != rendering.Roots[1].ID {
		t.Errorf("the flow between the ports is drawn %s => %s, want caller => callee", ping.From, ping.To)
	}
	if echo.From != echo.To {
		t.Errorf("the message between two events of caller is drawn %s => %s, want a self-message",
			echo.From, echo.To)
	}
	wants := []string{
		"attribute def Kit::Level has no place in a sequence rendering",
		"a connect in Talk::Handshake states no direction",
		"message pending states no source and target",
		"message adrift attaches to monitor.tapped, which is on no lifeline the rendering shows",
	}
	if len(rendering.Notices) != len(wants) {
		t.Fatalf("notices = %v, want %d", rendering.Notices, len(wants))
	}
	for i, want := range wants {
		if !strings.Contains(rendering.Notices[i], want) {
			t.Errorf("notice %d = %q, want it to say %q", i, rendering.Notices[i], want)
		}
	}
}

// A succession between the events of two messages orders those messages, even
// against the order they are declared in.
func TestSequenceRenderingHonoursStatedOrder(t *testing.T) {
	rendering := render(t, "sequence-order.sysml", "OrderingViews::relayView")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"early", "late"}) {
		t.Errorf("messages = %v, want the succession to put early first", got)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none: the order is stated and honoured", rendering.Notices)
	}
}

// Successions that contradict each other cannot be ordered: the cycle is
// reported, naming the successions, and declaration order stands.
func TestSequenceRenderingReportsASuccessionCycle(t *testing.T) {
	rendering := render(t, "sequence-order.sysml", "OrderingViews::deadlockView")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"ask", "reply"}) {
		t.Errorf("messages = %v, want declaration order", got)
	}
	if len(rendering.Notices) != 1 {
		t.Fatalf("notices = %v, want one about the cycle", rendering.Notices)
	}
	for _, want := range []string{"cycle", "answering", "heard to sent", "declaration order"} {
		if !strings.Contains(rendering.Notices[0], want) {
			t.Errorf("notice %q does not mention %q", rendering.Notices[0], want)
		}
	}
}

// An empty sequence rendering says why as a participant, since a sequence
// diagram carries no free text.
func TestEmptySequenceRenderingSaysWhy(t *testing.T) {
	rendering := render(t, "errors.sysml", "ErrorViews::emptySequenceView")
	if rendering.Kind != KindSequence || !rendering.Empty() {
		t.Fatalf("rendering = %s, empty = %v, want an empty sequence rendering", rendering.Kind, rendering.Empty())
	}
	if !strings.Contains(rendering.Text(), "the view exposes nothing") {
		t.Errorf("text does not say why it is empty:\n%s", rendering.Text())
	}
	mermaid := rendering.Mermaid()
	if !strings.Contains(mermaid, "sequenceDiagram") || !strings.Contains(mermaid, "participant empty as ") {
		t.Errorf("the empty sequence diagram holds no participant carrying the reason:\n%s", mermaid)
	}
}

// The Mermaid form is a sequence diagram declaring every participant before the
// first message, which is what the grammar asks for.
func TestSequenceMermaidDeclaresParticipantsFirst(t *testing.T) {
	mermaid := render(t, "sequence.sysml", "SequenceViews::pubSubView").Mermaid()
	lines := strings.Split(strings.TrimSpace(mermaid), "\n")
	if lines[0] != "%% SequenceViews::pubSubView — sequence rendering (view def SequenceView)" {
		t.Errorf("first line = %q, want the header comment", lines[0])
	}
	if lines[1] != "sequenceDiagram" {
		t.Errorf("second line = %q, want sequenceDiagram", lines[1])
	}
	var messages int
	for _, line := range lines[2:] {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "participant "):
			if messages > 0 {
				t.Errorf("participant declared after a message: %q", line)
			}
		case strings.Contains(line, "->>"):
			messages++
			if !strings.Contains(line, ": ") {
				t.Errorf("message carries no label: %q", line)
			}
		default:
			t.Errorf("unexpected line in the sequence diagram: %q", line)
		}
	}
	if messages != 3 {
		t.Errorf("%d messages in the diagram, want 3", messages)
	}
}

// lifelineNames are the names of a rendering's lifelines, in order.
func lifelineNames(rendering *Rendering) []string {
	out := make([]string, 0, len(rendering.Roots))
	for _, node := range rendering.Roots {
		out = append(out, node.Name)
	}
	return out
}

// edgeLabeled is the one message carrying a label, so a check names the message
// it is about rather than an index into the order.
func edgeLabeled(t *testing.T, rendering *Rendering, label string) Edge {
	t.Helper()
	for _, edge := range rendering.Edges {
		if edge.Label == label {
			return edge
		}
	}
	t.Fatalf("no message labeled %q among %v", label, messageLabels(rendering))
	return Edge{}
}

// messageLabels are the labels of a rendering's messages, in order.
func messageLabels(rendering *Rendering) []string {
	out := make([]string, 0, len(rendering.Edges))
	for _, edge := range rendering.Edges {
		out = append(out, edge.Label)
	}
	return out
}
