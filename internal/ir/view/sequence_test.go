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
	// The events a message ends at are proxies on their lifelines (§8.2.3.9
	// sq-proxy), each named as the lifeline's body declares it.
	wantProxies := [][]string{{"publish_source_event"}, {"subscribe_target_event", "publish_target_event", "deliver_source_event"},
		{"subscribe_source_event", "deliver_target_event"}}
	for i, node := range rendering.Roots {
		if !node.Origin.Located() {
			t.Errorf("lifeline %s carries no origin", node.Name)
		}
		var proxies []string
		for _, child := range node.Children {
			proxies = append(proxies, child.Name)
			if child.Kind != "event occurrence" || !child.Origin.Located() {
				t.Errorf("proxy %s on %s is a %q at %v, want an event occurrence with its origin", child.Name, node.Name, child.Kind, child.Origin)
			}
		}
		if !slices.Equal(proxies, wantProxies[i]) {
			t.Errorf("lifeline %s holds the proxies %v, want %v", node.Name, proxies, wantProxies[i])
		}
	}
	// The successions between the events state that the subscription precedes the
	// publication, against the order the messages are declared in.
	if got := messageLabels(rendering); !slices.Equal(got, []string{"subscribe_message of Subscribe", "publish_message of Publish", "deliver_message of Deliver"}) {
		t.Errorf("messages = %v, want one per message usage in the stated order", got)
	}
	participants := rendering.participants()
	for _, edge := range rendering.Edges {
		if edge.Kind != EdgeMessage && edge.Kind != EdgeSuccession {
			t.Errorf("%q is an edge of kind %v, want a message or a succession", edge.Label, edge.Kind)
		}
		if participants[edge.From] == "" || participants[edge.To] == "" {
			t.Errorf("%q runs %s -> %s, which is not between the lifelines' proxies", edge.Label, edge.From, edge.To)
		}
	}
	// The `then` chains of server and consumer are dashed successions between
	// their proxies, each after the row of the event it leads from.
	wantRows := []string{"subscribe_message of Subscribe", "succession server", "succession consumer", "publish_message of Publish", "succession server", "deliver_message of Deliver"}
	var rows []string
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeSuccession {
			if edge.From == edge.To || participants[edge.From] != participants[edge.To] {
				t.Errorf("succession %s -> %s, want one between two proxies of one lifeline", edge.From, edge.To)
			}
			rows = append(rows, "succession "+lifelineName(rendering, participants[edge.From]))
			continue
		}
		rows = append(rows, edge.Label)
	}
	if !slices.Equal(rows, wantRows) {
		t.Errorf("rows = %v, want %v", rows, wantRows)
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
	if got := messageLabels(rendering); !slices.Equal(got, []string{"of Ping", "echo of Ping"}) {
		t.Errorf("messages = %v, want the port-to-port flow and the self-message in declaration order", got)
	}
	ping, echo := edgeLabeled(t, rendering, "of Ping"), edgeLabeled(t, rendering, "echo of Ping")
	caller, callee := rendering.Roots[0], rendering.Roots[1]
	if ping.From != caller.ID || ping.To != callee.ID {
		t.Errorf("the flow between the ports is drawn %s => %s, want caller => callee", ping.From, ping.To)
	}
	// The ports a message ends at are drawn on their lifelines' borders (§8.2.3.9 sq-port).
	if len(caller.Ports) != 1 || len(callee.Ports) != 1 || ping.FromPort != caller.Ports[0].ID || ping.ToPort != callee.Ports[0].ID {
		t.Errorf("the flow between the ports ends at %q and %q; caller has the ports %v, callee %v", ping.FromPort, ping.ToPort, caller.Ports, callee.Ports)
	}
	if caller.Ports[0].Name != "outbox" || caller.Ports[0].Direction != PortUndirected {
		t.Errorf("caller's port = %+v, want outbox, undirected", caller.Ports[0])
	}
	participants := rendering.participants()
	if echo.From == echo.To || participants[echo.From] != caller.ID || participants[echo.To] != caller.ID {
		t.Errorf("the message between two events of caller is drawn %s => %s, want between two proxies of caller",
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
	if got := messageLabels(rendering); !slices.Equal(got, []string{"early of Tick", "late of Tick"}) {
		t.Errorf("messages = %v, want the succession to put early first", got)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none: the order is stated and honoured", rendering.Notices)
	}
}

// A send and a receive on one lifeline order their two messages whichever
// lifelines receive them: q receives a, sends b, sends c, so the rows are a, b, c.
func TestSequenceRenderingOrdersMessagesByTheirLifelines(t *testing.T) {
	for _, view := range []string{"LifelineViews::relayView", "LifelineViews::relayReversedView"} {
		rendering := render(t, "sequence-lifelines.sysml", view)
		if got := messageLabels(rendering); !slices.Equal(got, []string{"a of A", "b of B", "c of C"}) {
			t.Errorf("%s: messages = %v, want a, b, c as the successions on q state", view, got)
		}
		if len(rendering.Notices) != 0 {
			t.Errorf("%s: notices = %v, want none", view, rendering.Notices)
		}
	}
}

// A case's subject exposed on its own is drawn as a part exposed on its own is —
// by the events it holds — and its messages with it, not reported as misplaced.
func TestSequenceRenderingDrawsASubjectLifeline(t *testing.T) {
	rendering := render(t, "sequence-lifelines.sysml", "LifelineViews::servedView")
	var lifelines []string
	for _, root := range rendering.Roots {
		lifelines = append(lifelines, root.Name)
	}
	if !slices.Equal(lifelines, []string{"askSent", "replyTaken", "askTaken", "replySent"}) {
		t.Errorf("lifelines = %v, want the subject's events before the part's", lifelines)
	}
	if got := messageLabels(rendering); !slices.Equal(got, []string{"ask of A", "reply of B"}) {
		t.Errorf("messages = %v, want ask then reply", got)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none: the subject is drawn, not reported", rendering.Notices)
	}
}

// An order carried through an event no message attaches to still orders the
// messages on either side of it.
func TestSequenceRenderingFollowsOrdersThroughUnmessagedEvents(t *testing.T) {
	rendering := render(t, "sequence-lifelines.sysml", "LifelineViews::settledView")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"ask of A", "reply of B"}) {
		t.Errorf("messages = %v, want ask before reply through the settle event", got)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
}

// The successions through events no message attaches to are rows between the
// messages they order, in the events' order, not after them.
func TestSequenceRenderingPlacesSuccessionsBetweenTheirMessages(t *testing.T) {
	rendering := render(t, "sequence-lifelines.sysml", "LifelineViews::stagedView")
	server := rendering.Roots[1]
	names := map[string]string{}
	for _, proxy := range server.Children {
		names[proxy.ID] = proxy.Name
	}
	var rows []string
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeSuccession {
			rows = append(rows, names[edge.From]+" then "+names[edge.To])
			continue
		}
		rows = append(rows, edge.Label)
	}
	want := []string{"ask of A", "askTaken then stageOne", "stageOne then stageTwo", "stageTwo then replySent", "reply of B"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
}

// A chain of successions declared last link first, after a message's event and
// before no message, is drawn in the events' order: each row holds its events.
func TestSequenceRenderingOrdersAChainOfSuccessionsByTheirEvents(t *testing.T) {
	rendering := render(t, "sequence-lifelines.sysml", "LifelineViews::chainedView")
	names := map[string]string{}
	for _, root := range rendering.Roots {
		for _, proxy := range root.Children {
			names[proxy.ID] = proxy.Name
		}
	}
	var rows []string
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeSuccession {
			rows = append(rows, names[edge.From]+" then "+names[edge.To])
			continue
		}
		rows = append(rows, edge.Label)
	}
	want := []string{"ask of A", "askTaken then second", "second then third"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
}

// Messages nothing orders keep declaration order.
func TestSequenceRenderingKeepsDeclarationOrderWhereNothingIsStated(t *testing.T) {
	rendering := render(t, "sequence-lifelines.sysml", "LifelineViews::unorderedView")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"pong of B", "ping of A"}) {
		t.Errorf("messages = %v, want declaration order", got)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
}

// An order against a message's own direction, or rows ordered both ways, is a
// cycle too: reported naming the succession, and declaration order stands.
func TestSequenceRenderingReportsOrdersNoRowCanSatisfy(t *testing.T) {
	for _, tc := range []struct {
		view     string
		messages []string
		named    string
	}{
		{"LifelineViews::reversedView", []string{"back of B", "m of A"}, "received to sent"},
		{"LifelineViews::crossedView", []string{"two of B", "one of A"}, "crossing"},
	} {
		rendering := render(t, "sequence-lifelines.sysml", tc.view)
		if got := messageLabels(rendering); !slices.Equal(got, tc.messages) {
			t.Errorf("%s: messages = %v, want declaration order %v", tc.view, got, tc.messages)
		}
		if len(rendering.Notices) != 1 {
			t.Fatalf("%s: notices = %v, want one about the cycle", tc.view, rendering.Notices)
		}
		for _, want := range []string{"cycle", tc.named, "declaration order"} {
			if !strings.Contains(rendering.Notices[0], want) {
				t.Errorf("%s: notice %q does not mention %q", tc.view, rendering.Notices[0], want)
			}
		}
	}
}

// A message attached to a lifeline itself has no event of its own to order: two
// running opposite ways are no cycle, and the other messages' orders still hold.
func TestSequenceRenderingLeavesUntimedMessageEndsUnordered(t *testing.T) {
	rendering := render(t, "sequence-lifelines.sysml", "LifelineViews::reciprocalView")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"forward of A", "backward of B", "early of A", "late of C"}) {
		t.Errorf("messages = %v, want the lifeline-attached pair in declaration order, then early before late", got)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
}

// Successions that contradict each other cannot be ordered: the cycle is
// reported, naming the successions, and declaration order stands.
func TestSequenceRenderingReportsASuccessionCycle(t *testing.T) {
	rendering := render(t, "sequence-order.sysml", "OrderingViews::deadlockView")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"ask of Tick", "reply of Tick"}) {
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
	mermaid := render(t, "sequence.sysml", "SequenceViews::pubSubView").MermaidWith(fullNotation)
	if strings.HasPrefix(mermaid, "---\n") {
		end := strings.Index(mermaid[4:], "\n---\n")
		if end < 0 {
			t.Fatal("Mermaid frontmatter is not closed")
		}
		mermaid = mermaid[4+end+len("\n---\n"):]
	}
	lines := strings.Split(strings.TrimSpace(mermaid), "\n")
	if lines[0] != "%% SequenceViews::pubSubView — sequence rendering (view def SequenceView)" {
		t.Errorf("first line = %q, want the header comment", lines[0])
	}
	if lines[1] != "sequenceDiagram" {
		t.Errorf("second line = %q, want sequenceDiagram", lines[1])
	}
	var messages, successions int
	for _, line := range lines[2:] {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "participant "):
			if messages > 0 {
				t.Errorf("participant declared after a message: %q", line)
			}
		case strings.Contains(line, "--)"):
			successions++
		case strings.Contains(line, "-)"):
			messages++
			if !strings.Contains(line, ": ") {
				t.Errorf("message carries no label: %q", line)
			}
		default:
			t.Errorf("unexpected line in the sequence diagram: %q", line)
		}
	}
	if messages != 3 || successions != 3 {
		t.Errorf("%d messages and %d successions in the diagram, want 3 of each", messages, successions)
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

// messageLabels are the labels of a rendering's messages, in order: its message
// and flow edges, not the successions drawn between them.
func messageLabels(rendering *Rendering) []string {
	out := make([]string, 0, len(rendering.Edges))
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeMessage || edge.Kind == EdgeFlow {
			out = append(out, edge.Label)
		}
	}
	return out
}

// lifelineName is the name of the lifeline with the given ID.
func lifelineName(rendering *Rendering, id string) string {
	for _, node := range rendering.Roots {
		if node.ID == id {
			return node.Name
		}
	}
	return ""
}
