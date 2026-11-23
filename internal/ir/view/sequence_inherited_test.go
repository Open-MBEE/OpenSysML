package view

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const inheritedEventModel = `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		event occurrence got;
		succession first sent then got;
	}
	part server {
		event occurrence a;
		event occurrence b;
		event occurrence c;
		event occurrence d;
	}
	part client : Client;
	part other : Client;
	message m1 of Ping from client.sent to server.a;
	message m2 of Ping from server.b to client.got;
	message m3 of Ping from other.sent to server.c;
	message m4 of Ping from server.d to other.got;
}`

// renderInheritedEvents renders the named members of the inherited-event model
// with its four messages as one sequence.
func renderInheritedEvents(t *testing.T, names ...string) *Rendering {
	t.Helper()
	r, idx := loadSources(t, []string{"inline.sysml"}, [][]byte{[]byte(inheritedEventModel)})
	var exposed []*symbols.Symbol
	for _, name := range append(names, "m1", "m2", "m3", "m4") {
		exposed = append(exposed, lookup(t, idx, "P::"+name))
	}
	rendering, err := r.RenderExposed(exposed, KindSequence, "#sequence")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return rendering
}

// sequenceEnds spells each edge of a sequence by the lifelines and events it runs
// between, a succession without a label.
func sequenceEnds(rendering *Rendering) []string {
	name := func(id string) string {
		for _, root := range rendering.Roots {
			if root.ID == id {
				return root.Name
			}
			for _, proxy := range root.Children {
				if proxy.ID == id {
					return root.Name + "." + proxy.Name
				}
			}
		}
		return id
	}
	var out []string
	for _, edge := range rendering.Edges {
		out = append(out, edge.Label+": "+name(edge.From)+" -> "+name(edge.To))
	}
	return out
}

// A message naming an event through a lifeline that inherits it is drawn on that
// lifeline, whether or not the definition declaring the event is shown too, and
// the order a shown definition states holds on each such lifeline on its own.
func TestSequenceRenderingDrawsInheritedEventsOnTheLifelineNamed(t *testing.T) {
	want := []string{
		"m1 of Ping: P::client.sent -> a",
		": P::client.sent -> P::client.got",
		"m2 of Ping: b -> P::client.got",
		"m3 of Ping: P::other.sent -> c",
		": P::other.sent -> P::other.got",
		"m4 of Ping: d -> P::other.got",
	}
	// Without the definition its succession is not among what the rendering reads.
	messages := []string{want[0], want[2], want[3], want[5]}
	if got := sequenceEnds(renderInheritedEvents(t, "server", "client", "other")); !slices.Equal(got, messages) {
		t.Errorf("usages alone:\n got %q\nwant %q", got, messages)
	}
	rendering := renderInheritedEvents(t, "Client", "server", "client", "other")
	got := sequenceEnds(rendering)
	// The events the definition and the server declare are lifelines of their own.
	if !slices.Equal(got, append(slices.Clone(want), ": sent -> got")) {
		t.Errorf("definition and usages:\n got %q\nwant %q then the definition's own succession", got, want)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
}

// With no lifeline inheriting it shown, the event stays on the definition's.
func TestSequenceRenderingKeepsAnEventOnItsDefinitionWhenNoUsageIsShown(t *testing.T) {
	want := []string{
		"m1 of Ping: sent -> a",
		"m2 of Ping: b -> got",
		"m3 of Ping: sent -> c",
		"m4 of Ping: d -> got",
		": sent -> got",
	}
	if got := sequenceEnds(renderInheritedEvents(t, "Client", "server")); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
