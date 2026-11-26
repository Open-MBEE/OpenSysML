package view

import (
	"slices"
	"strings"
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

// renderSequenceOf renders the named members of package P of a model as one sequence.
func renderSequenceOf(t *testing.T, model string, names ...string) *Rendering {
	t.Helper()
	r, idx := loadSources(t, []string{"inline.sysml"}, [][]byte{[]byte(model)})
	var exposed []*symbols.Symbol
	for _, name := range names {
		exposed = append(exposed, lookup(t, idx, "P::"+name))
	}
	rendering, err := r.RenderExposed(exposed, KindSequence, "#sequence")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return rendering
}

// A succession naming its ends through two usages orders their two events: it is
// one arrow between the lifelines named, not an order within each.
func TestSequenceRenderingDrawsASuccessionBetweenTheLifelinesItNames(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		event occurrence got;
	}
	part server {
		event occurrence a;
		event occurrence b;
	}
	part client : Client;
	part other : Client;
	message m1 of Ping from other.got to server.a;
	message m2 of Ping from client.sent to server.b;
	succession handover first client.sent then other.got;
}`, "server", "client", "other", "m1", "m2", "handover")
	want := []string{
		"m2 of Ping: P::client.sent -> b",
		"handover: P::client.sent -> P::other.got",
		"m1 of Ping: P::other.got -> a",
	}
	if got := sequenceEnds(rendering); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// A chain of inherited orders reaches a lifeline's events whichever link is
// stated first, so the messages at its two ends are ordered by it.
func TestSequenceRenderingFollowsAnInheritedChainStatedOutOfOrder(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence first_;
		event occurrence x;
		event occurrence y;
		event occurrence last_;
		succession first x then y;
		succession first first_ then x;
		succession first y then last_;
	}
	part server {
		event occurrence a;
		event occurrence b;
	}
	part client : Client;
	message late of Ping from client.last_ to server.a;
	message early of Ping from client.first_ to server.b;
}`, "Client", "server", "client", "late", "early")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"early of Ping", "late of Ping"}) {
		t.Errorf("messages = %v, want early before late through the inherited chain", got)
	}
	var inherited []string
	for _, end := range sequenceEnds(rendering) {
		if len(end) > 2 && end[:2] == ": " && end[2] == 'P' {
			inherited = append(inherited, end)
		}
	}
	want := []string{
		": P::client.first_ -> P::client.x",
		": P::client.x -> P::client.y",
		": P::client.y -> P::client.last_",
	}
	if !slices.Equal(inherited, want) {
		t.Errorf("inherited successions = %q, want %q", inherited, want)
	}
}

// An inherited order reaches the event a lifeline redefines, not the one redefined.
func TestSequenceRenderingReadsAnInheritedOrderOverARedefinedEvent(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		event occurrence got;
		succession first sent then got;
	}
	part def Scene {
		part server {
			event occurrence a;
			event occurrence b;
		}
		part client : Client {
			event occurrence answered :>> got;
		}
		message reply of Ping from server.a to client.answered;
		message ask of Ping from client.sent to server.b;
	}
}`, "Client", "Scene")
	want := []string{
		"ask of Ping: client.sent -> server.b",
		": client.sent -> client.answered",
		"reply of Ping: server.a -> client.answered",
		": sent -> got",
	}
	if got := sequenceEnds(rendering); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// An inherited order holds on a lifeline whose events only successions name, so
// the messages those successions lead from and to are ordered through it.
func TestSequenceRenderingReadsAnInheritedOrderBetweenEventsOnlySuccessionsName(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		event occurrence got;
		succession first got then sent;
	}
	part def Scene {
		part server {
			event occurrence a;
			event occurrence b;
		}
		part peer {
			event occurrence c;
			event occurrence d;
		}
		part client : Client;
		message late of Ping from server.b to peer.d;
		message early of Ping from server.a to peer.c;
		succession first peer.c then client.got;
		succession first client.sent then server.b;
	}
}`, "Client", "Scene")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"early of Ping", "late of Ping"}) {
		t.Errorf("messages = %v, want early before late through client's inherited order", got)
	}
	if got := sequenceEnds(rendering); !slices.Contains(got, ": client.got -> client.sent") {
		t.Errorf("edges = %q, want the inherited order drawn on client", got)
	}
}

// An order one usage states over the events it inherits is that usage's alone:
// it is drawn on its lifeline and does not hold on another typed alike.
func TestSequenceRenderingKeepsAUsagesOwnOrderOffItsSiblings(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		event occurrence got;
	}
	part def Scene {
		part server {
			event occurrence a;
			event occurrence b;
		}
		part client : Client;
		part other : Client {
			succession first got then sent;
		}
		message reply of Ping from server.a to client.got;
		message ask of Ping from client.sent to server.b;
		succession first client.sent then other.got;
	}
}`, "Scene")
	want := []string{
		"reply of Ping: server.a -> client.got",
		"ask of Ping: client.sent -> server.b",
		": client.sent -> other.got",
		": other.got -> other.sent",
	}
	if got := sequenceEnds(rendering); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// An order a part states between occurrences it neither declares nor inherits
// stays on their own lifelines.
func TestSequenceRenderingKeepsAnOrderOfForeignEventsOnTheirLifelines(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	occurrence opened;
	occurrence closed;
	part door {
		succession first opened then closed;
	}
}`, "opened", "closed", "door")
	if got := sequenceEnds(rendering); !slices.Equal(got, []string{": P::opened -> P::closed"}) {
		t.Errorf("got %q, want the order between the two occurrences' own lifelines", got)
	}
}

// An order a subtype states over events it inherits holds on a lifeline typed by
// the subtype, and not on one typed by the base alone.
func TestSequenceRenderingReadsASubtypesOrderOnItsLifelines(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		event occurrence got;
	}
	part def Careful :> Client {
		succession first got then sent;
	}
	part def Scene {
		part server {
			event occurrence a;
			event occurrence b;
			event occurrence c;
			event occurrence d;
		}
		part careful : Careful;
		part plain : Client;
		message ask of Ping from careful.sent to server.a;
		message reply of Ping from server.b to careful.got;
		message plainAsk of Ping from plain.sent to server.c;
		message plainReply of Ping from server.d to plain.got;
	}
}`, "Careful", "Scene")
	want := []string{
		"reply of Ping: server.b -> careful.got",
		": careful.got -> careful.sent",
		"ask of Ping: careful.sent -> server.a",
		"plainAsk of Ping: plain.sent -> server.c",
		"plainReply of Ping: server.d -> plain.got",
		// The subtype shown is a lifeline of its own, holding the order it states.
		": P::Careful.got -> P::Careful.sent",
	}
	if got := sequenceEnds(rendering); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// An inherited order reaches the event an intermediate type redefines, where the
// messages of a lifeline typed by it attach.
func TestSequenceRenderingReadsAnInheritedOrderOverAnEventATypeRedefines(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		event occurrence got;
		succession first sent then got;
	}
	part def Careful :> Client {
		event occurrence answered :>> got;
	}
	part def Scene {
		part server {
			event occurrence a;
			event occurrence b;
		}
		part careful : Careful;
		message reply of Ping from server.a to careful.answered;
		message ask of Ping from careful.sent to server.b;
	}
}`, "Client", "Scene")
	want := []string{
		"ask of Ping: careful.sent -> server.b",
		": careful.sent -> careful.answered",
		"reply of Ping: server.a -> careful.answered",
		": sent -> got",
	}
	if got := sequenceEnds(rendering); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// An inherited chain of `then` runs through an event its definition leaves
// unnamed, which no member list holds, and still orders the messages at its ends.
func TestSequenceRenderingFollowsAnInheritedChainThroughAnUnnamedEvent(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		then event occurrence;
		then event occurrence got;
	}
	part def Scene {
		part server {
			event occurrence a;
			event occurrence b;
		}
		part client : Client;
		message reply of Ping from server.a to client.got;
		message ask of Ping from client.sent to server.b;
	}
}`, "Client", "Scene")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"ask of Ping", "reply of Ping"}) {
		t.Errorf("messages = %v, want ask before reply through the unnamed event; edges %q", got, sequenceEnds(rendering))
	}
}

// An event one supertype redefines privately and another still supplies is the
// event a message names, on the lifeline it names it through.
func TestSequenceRenderingKeepsAnEventAnotherBranchRedefines(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
	}
	part def Quiet :> Client {
		private event occurrence hushed :>> sent;
	}
	part def Both :> Quiet, Client;
	part def Scene {
		part server {
			event occurrence a;
		}
		part both : Both;
		message ask of Ping from both.sent to server.a;
	}
}`, "Client", "Scene")
	if got := sequenceEnds(rendering); !slices.Contains(got, "ask of Ping: both.sent -> server.a") {
		t.Errorf("edges = %q, want the message on the both lifeline; notices %q", got, rendering.Notices)
	}
}

// A message naming an event of an action definition through an action typed by
// it is drawn on the usage's lifeline; the definition, reached by nothing, is not shown.
func TestSequenceRenderingDrawsAnActionsInheritedEventOnTheUsage(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	action def Run {
		event occurrence sent;
	}
	action run : Run;
	part server {
		event occurrence got;
	}
	message ping of Ping from run.sent to server.got;
}`, "Run", "run", "server", "ping")
	if got := sequenceEnds(rendering); !slices.Equal(got, []string{"ping of Ping: P::run.sent -> got"}) {
		t.Errorf("got %q, want the message on the run lifeline", got)
	}
	if got := lifelineNames(rendering); slices.Contains(got, "P::Run") {
		t.Errorf("lifelines = %v, want the definition no message reaches left out", got)
	}
}

// A private event is not inherited (KerML 8.3.3.1 inheritableMemberships), so an
// order from one, named or not, stays on the definition and off the lifelines typed by it.
func TestSequenceRenderingKeepsAPrivateEventOffTheLifelinesSpecializingItsOwner(t *testing.T) {
	for name, private := range map[string]string{
		"unnamed": "private event occurrence;\n\t\tthen event occurrence got;",
		"named":   "private event occurrence hidden;\n\t\tevent occurrence got;\n\t\tsuccession first hidden then got;",
	} {
		t.Run(name, func(t *testing.T) {
			rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		`+private+`
	}
	part def Scene {
		part server {
			event occurrence a;
		}
		part client : Client;
		message reply of Ping from server.a to client.got;
	}
}`, "Client", "Scene")
			got := sequenceEnds(rendering)
			if !slices.Contains(got, "reply of Ping: server.a -> client.got") {
				t.Errorf("edges = %q, want the message on the client lifeline", got)
			}
			for _, edge := range got {
				if strings.HasPrefix(edge, ": client.") {
					t.Errorf("edges = %q, want no order from the private event on the client lifeline", got)
				}
			}
			if proxies := len(lifelineNamed(t, rendering, "client").Children); proxies != 1 {
				t.Errorf("client lifeline holds %d events, want got alone; edges %q", proxies, got)
			}
		})
	}
}

// lifelineNamed is the lifeline of a sequence with the given name.
func lifelineNamed(t *testing.T, rendering *Rendering, name string) *Node {
	t.Helper()
	for _, root := range rendering.Roots {
		if root.Name == name {
			return root
		}
	}
	t.Fatalf("no lifeline %q among %d", name, len(rendering.Roots))
	return nil
}

// An inherited chain of `then` through a private event still orders the messages
// at its ends, though neither the event nor the links at it are drawn on the lifeline.
func TestSequenceRenderingOrdersMessagesThroughAPrivateEventItDoesNotDraw(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		event occurrence sent;
		then private event occurrence;
		then event occurrence got;
	}
	part def Scene {
		part server {
			event occurrence a;
			event occurrence b;
		}
		part client : Client;
		message reply of Ping from server.a to client.got;
		message ask of Ping from client.sent to server.b;
	}
}`, "Client", "Scene")
	if got := messageLabels(rendering); !slices.Equal(got, []string{"ask of Ping", "reply of Ping"}) {
		t.Errorf("messages = %v, want ask before reply through the private event; edges %q", got, sequenceEnds(rendering))
	}
	if proxies := len(lifelineNamed(t, rendering, "client").Children); proxies != 2 {
		t.Errorf("client lifeline holds %d events, want sent and got; edges %q", proxies, sequenceEnds(rendering))
	}
}

// An order a usage states between events it holds only by reference subsetting is
// read on a lifeline specializing both that usage and the one referenced.
func TestSequenceRenderingReadsAnOrderOverEventsHeldByReferenceSubsetting(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part server {
		event occurrence a;
		event occurrence b;
	}
	part source {
		event occurrence sent;
		event occurrence got;
	}
	ref part observer ::> source {
		succession first sent then got;
	}
	part client :> observer, source;
	message ask of Ping from client.sent to server.a;
	message reply of Ping from server.b to client.got;
}`, "server", "observer", "client", "ask", "reply")
	want := []string{
		"ask of Ping: P::client.sent -> a",
		": P::client.sent -> P::client.got",
		"reply of Ping: b -> P::client.got",
	}
	if got := sequenceEnds(rendering); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// A lifeline that specializes only the usage referenced does not read the order the
// referencing usage states.
func TestSequenceRenderingKeepsAReferencingUsagesOrderOffTheUsageReferenced(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
		item def Ping;
		part server {
			event occurrence a;
			event occurrence b;
		}
		part source {
			event occurrence sent;
			event occurrence got;
		}
		ref part observer ::> source {
			succession first sent then got;
		}
		part client :> source;
		message ask of Ping from client.sent to server.a;
		message reply of Ping from server.b to client.got;
	}`, "server", "observer", "client", "ask", "reply")
	want := []string{
		"ask of Ping: P::client.sent -> a",
		"reply of Ping: b -> P::client.got",
	}
	if got := sequenceEnds(rendering); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// A public alias is a membership of its own, inherited whatever the visibility of the
// event it names, so a message naming a private event through one ends on the lifeline.
func TestSequenceRenderingDrawsAPrivateEventNamedThroughAnInheritedAlias(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		private event occurrence hidden;
		public alias shown for hidden;
	}
	part def Scene {
		part server {
			event occurrence a;
		}
		part client : Client;
		message reply of Ping from server.a to client.shown;
	}
}`, "Client", "Scene")
	if got := sequenceEnds(rendering); !slices.Contains(got, "reply of Ping: server.a -> client.hidden") {
		t.Errorf("edges = %q, want the message to end on the client lifeline", got)
	}
}

// A private alias is no more inherited than the private event it names, so the
// order from that event stays off the lifeline.
func TestSequenceRenderingKeepsAPrivateEventWithAPrivateAliasOffTheLifeline(t *testing.T) {
	rendering := renderSequenceOf(t, `package P {
	item def Ping;
	part def Client {
		private event occurrence hidden;
		private alias shown for hidden;
		event occurrence got;
		succession first hidden then got;
	}
	part def Scene {
		part server {
			event occurrence a;
		}
		part client : Client;
		message reply of Ping from server.a to client.got;
	}
}`, "Client", "Scene")
	got := sequenceEnds(rendering)
	if slices.Contains(got, ": client.hidden -> client.got") {
		t.Errorf("edges = %q, want no order from the private event on the client lifeline", got)
	}
	if proxies := len(lifelineNamed(t, rendering, "client").Children); proxies != 1 {
		t.Errorf("client lifeline holds %d events, want got alone; edges %q", proxies, got)
	}
}
