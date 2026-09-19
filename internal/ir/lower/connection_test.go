package lower

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestLowerConnectionsFromActionBody(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := peerPorts(graph.Connections, "outPort"); len(got) != 1 || got[0] != "inPort" {
		t.Errorf("peerPorts(outPort) = %v, want [inPort]", got)
	}
	// A connection joins its ends without a direction, so routing works either way.
	if got := peerPorts(graph.Connections, "inPort"); len(got) != 1 || got[0] != "outPort" {
		t.Errorf("peerPorts(inPort) = %v, want [outPort]", got)
	}
}

// An n-ary connection joins every end it declares, and every end is a peer of
// every other one (SysML v2 §7.13.2): lowering must not truncate to a pair.
func TestLowerNaryConnectionKeepsEveryEnd(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p1;
			port p2;
			port p3;
			port p4;
			connection link connect (p1, p2, p3, p4);
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := graph.Connections[0].Ends; len(got) != 4 {
		t.Fatalf("lowered ends = %v, want four", got)
	}
	for _, port := range []string{"p1", "p2", "p3", "p4"} {
		peers := peerPorts(graph.Connections, port)
		if len(peers) != 3 {
			t.Errorf("peerPorts(%s) = %v, want the other three ends", port, peers)
		}
		if slices.Contains(peers, port) {
			t.Errorf("peerPorts(%s) = %v, an end is not its own peer", port, peers)
		}
	}
}

// An end that declares its own name attaches to the feature it
// reference-subsets, so that is what routing must join.
func TestLowerConnectionEndsThatDeclareTheirOwnName(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port outPort;
			port inPort;
			connection : Link connect
				source references outPort to
				target references inPort;
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := peerPorts(graph.Connections, "outPort"); len(got) != 1 || got[0] != "inPort" {
		t.Errorf("peerPorts(outPort) = %v, want [inPort]", got)
	}
}

func TestLowerSendRecordsViaForm(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p;
			first start;
			action viaSend { send 1 via p; }
			action toSend { send 2 to other; }
			action routedSend { send 3 via p to receiver; }
			done;
			succession first start then viaSend;
			succession first viaSend then toSend;
			succession first toSend then routedSend;
			succession first routedSend then done;
		}
	`)
	var sends []Send
	for _, body := range graph.Bodies {
		for _, stmt := range body {
			if send, ok := stmt.(Send); ok {
				sends = append(sends, send)
			}
		}
	}
	if len(sends) != 3 {
		t.Fatalf("lowered %d sends, want 3", len(sends))
	}
	var via, addressed, routed *Send
	for i := range sends {
		send := &sends[i]
		switch {
		case send.IsVia && send.Receiver != "":
			routed = send
		case send.IsVia:
			via = send
		default:
			addressed = send
		}
	}
	if via == nil || via.Target != "p" {
		t.Errorf("`send 1 via p` lowered to %+v, want IsVia with Target p", via)
	}
	if addressed == nil || addressed.IsVia || addressed.Target != "other" {
		t.Errorf("`send 2 to other` lowered to %+v, want no IsVia with Target other", addressed)
	}
	if routed == nil || routed.Target != "p" || !routed.TargetPath ||
		routed.Receiver != "receiver" || routed.ReceiverPath {
		t.Errorf("`send 3 via p to receiver` lowered to %+v", routed)
	}
}

func TestLowerSendRoutedReceiverKeepsFeaturePath(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p;
			part outer { action receiver; }
			action sender { send 1 via p to outer.receiver; }
			first start;
			done;
			succession first start then sender;
			succession first sender then done;
		}
	`)
	var sends []Send
	for _, body := range graph.Bodies {
		for _, stmt := range body {
			if send, ok := stmt.(Send); ok {
				sends = append(sends, send)
			}
		}
	}
	if len(sends) != 1 {
		t.Fatalf("lowered %d sends, want 1", len(sends))
	}
	if got := sends[0]; got.Receiver != "outer.receiver" || !got.ReceiverPath {
		t.Errorf("routed receiver lowered to %+v, want feature path", got)
	}
}

// An addressed target keeps every segment it was written with: dropping the
// owner would let the runtime address any same-named port.
func TestLowerSendKeepsAddressedPath(t *testing.T) {
	tests := []struct {
		target string
		want   string
		path   bool
	}{
		{"alpha.inPort", "alpha.inPort", true},
		{"P::Driver", "P::Driver", false},
		{"reader", "reader", false},
	}
	for _, tc := range tests {
		graph := actionGraphFor(t, `
			action a {
				first start;
				action toSend { send 2 to `+tc.target+`; }
				done;
				succession first start then toSend;
				succession first toSend then done;
			}
		`)
		var sends []Send
		for _, body := range graph.Bodies {
			for _, stmt := range body {
				if send, ok := stmt.(Send); ok {
					sends = append(sends, send)
				}
			}
		}
		if len(sends) != 1 {
			t.Fatalf("`send 2 to %s` lowered %d sends, want 1", tc.target, len(sends))
		}
		if sends[0].Target != tc.want || sends[0].TargetPath != tc.path {
			t.Errorf("`send 2 to %s` lowered target %q (path %v), want %q (path %v)",
				tc.target, sends[0].Target, sends[0].TargetPath, tc.want, tc.path)
		}
	}
}

// A `via` target names a port of the sender, which routing matches against the
// lowered connector ends, so both must be rendered the same way.
func TestLowerSendViaQualifiedPortMatchesConnectionEnds(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port inPort;
			part outer { port p; }
			connect outer.p to inPort;
			first start;
			action viaSend { send 1 via outer::p; }
			done;
			succession first start then viaSend;
			succession first viaSend then done;
		}
	`)
	var via Send
	for _, body := range graph.Bodies {
		for _, stmt := range body {
			if send, ok := stmt.(Send); ok && send.IsVia {
				via = send
			}
		}
	}
	if via.Target != "outer.p" {
		t.Fatalf("`send 1 via outer::p` lowered target %q, want outer.p", via.Target)
	}
	if got := peerPorts(graph.Connections, via.Target); len(got) != 1 || got[0] != "inPort" {
		t.Errorf("peerPorts(%q) = %v, want [inPort]", via.Target, got)
	}
}

func TestLowerAcceptRecordsViaPort(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port inPort;
			first start;
			action r accept msg : Integer via inPort;
			done;
			succession first start then r;
			succession first r then done;
		}
	`)
	if len(graph.Accepts) != 1 {
		t.Fatalf("Accepts = %v, want one accept", graph.Accepts)
	}
	for _, accept := range graph.Accepts {
		if accept.ViaPort != "inPort" {
			t.Errorf("Accept = %+v, want ViaPort inPort", accept)
		}
	}
}

func TestPeerPortsIgnoresUnconnectedAndSelf(t *testing.T) {
	conns := []Connection{{Ends: []string{"a", "b"}}, {Ends: []string{"a", "a"}}}
	if got := peerPorts(conns, "lonely"); got != nil {
		t.Errorf("peerPorts(lonely) = %v, want nothing", got)
	}
	// A port joined to itself reaches nobody but itself, which is not a peer.
	if got := peerPorts(conns, "a"); len(got) != 1 || got[0] != "b" {
		t.Errorf("peerPorts(a) = %v, want [b]", got)
	}
	if got := peerPorts(nil, "a"); got != nil {
		t.Errorf("peerPorts with no connections = %v, want nothing", got)
	}
}

func TestPeerPortsSpansMultiEndAndMultipleConnections(t *testing.T) {
	conns := []Connection{
		{Ends: []string{"hub", "left", "right"}},
		{Ends: []string{"hub", "extra"}},
		{Ends: []string{"hub", "left"}},
	}
	got := peerPorts(conns, "hub")
	want := []string{"left", "right", "extra"}
	if len(got) != len(want) {
		t.Fatalf("peerPorts(hub) = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("peerPorts(hub) = %v, want %v", got, want)
		}
	}
}

// The anonymous inline form declares no connection name, but its ends must
// still reach routing.
func TestLowerAnonymousNaryConnectionKeepsEveryEnd(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p1;
			port p2;
			port p3;
			connect (p1, p2, p3);
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := graph.Connections[0].Ends; len(got) != 3 {
		t.Fatalf("lowered ends = %v, want three", got)
	}
	for _, port := range []string{"p1", "p2", "p3"} {
		if peers := peerPorts(graph.Connections, port); len(peers) != 2 {
			t.Errorf("peerPorts(%s) = %v, want the other two ends", port, peers)
		}
	}
}

// A `variation interface` declares no ends of its own: what joins ends are the
// connections its variants declare, and each is lowered tagged with the
// variation and variant it came from so routing can honor the selection.
func TestLowerVariantConnectionsCarryTheirVariation(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p1;
			port p2;
			port p3;
			variation interface link {
				variant interface direct connect p1 to p2;
				variant interface indirect connect p1 to p3;
			}
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 2 {
		t.Fatalf("Connections = %v, want the two variants' connections", graph.Connections)
	}
	for i, want := range []Connection{
		{Ends: []string{"p1", "p2"}, Variation: "link", Variant: "direct"},
		{Ends: []string{"p1", "p3"}, Variation: "link", Variant: "indirect"},
	} {
		got := graph.Connections[i]
		if got.Variation != want.Variation || got.Variant != want.Variant {
			t.Errorf("connection %d = %+v, want variation %s variant %s",
				i, got, want.Variation, want.Variant)
		}
		if len(got.Ends) != 2 || got.Ends[0] != want.Ends[0] || got.Ends[1] != want.Ends[1] {
			t.Errorf("connection %d ends = %v, want %v", i, got.Ends, want.Ends)
		}
	}
}

// A connection declared outside a variation belongs to no variant, so nothing
// about it is conditional on a selection.
func TestLowerPlainConnectionCarriesNoVariant(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if conn := graph.Connections[0]; conn.Variation != "" || conn.Variant != "" {
		t.Errorf("connection = %+v, want no variation and no variant", conn)
	}
}

// An end reached through a chain attaches to the port the chain names, keeping
// every segment: `sensor.out` joins that port, not another named `out`.
func TestLowerConnectionEndFollowsAFeatureChain(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			part sensor { port out; }
			port inPort;
			connect sensor.out to inPort;
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := peerPorts(graph.Connections, "sensor.out"); len(got) != 1 || got[0] != "inPort" {
		t.Errorf("peerPorts(sensor.out) = %v, want [inPort]", got)
	}
	if got := peerPorts(graph.Connections, "out"); got != nil {
		t.Errorf("peerPorts(out) = %v, want nothing: the end names sensor.out", got)
	}
}

// A connection lowered from a behavior's own body is owned by that behavior, so
// routing it needs no object.
func TestLowerBehaviorConnectionIsOwnedByTheBehavior(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			done;
			succession first start then done;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := graph.Connections[0].Owner; got != OwnerBehavior {
		t.Errorf("Owner = %v, want OwnerBehavior", got)
	}
}

// The connections of the type of an object performing a behavior are lowered
// tagged as the object's own, so a variant of a variation the object binds is
// realized for the object that bound it rather than for the behavior.
func TestLowerObjectConnectionsAreOwnedByTheObject(t *testing.T) {
	def := usageOrDefFor(t, `
		part def Router {
			port outPort;
			port inPort;
			port bypass;
			connect outPort to inPort;
			variation interface link {
				variant interface direct connect outPort to inPort;
				variant interface indirect connect outPort to bypass;
			}
		}
	`)
	conns := ToObjectConnections(def, nil)
	if len(conns) != 3 {
		t.Fatalf("ToObjectConnections = %+v, want the plain connection and both variants", conns)
	}
	for _, conn := range conns {
		if conn.Owner != OwnerObject {
			t.Errorf("connection %+v: Owner = %v, want OwnerObject", conn, conn.Owner)
		}
	}
	if conns[1].Variation != "link" || conns[1].Variant != "direct" {
		t.Errorf("connection = %+v, want variation link variant direct", conns[1])
	}
	if got := peerPorts(conns[2:], "outPort"); len(got) != 1 || got[0] != "bypass" {
		t.Errorf("peerPorts(outPort) over the indirect variant = %v, want [bypass]", got)
	}
}

// usageOrDefFor parses one declaration and returns its node, for lowering that
// starts from a type rather than from a behavior graph.
func usageOrDefFor(t *testing.T, src string) ast.Node {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	for _, member := range root.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		switch decl := membership.Member.(type) {
		case *ast.Definition:
			return decl
		case *ast.Usage:
			return decl
		}
	}
	t.Fatalf("no definition or usage in %q", src)
	return nil
}

// peerPorts returns the ends connected to port, in declaration order and
// without duplicates. A port is never its own peer.
func peerPorts(conns []Connection, port string) []string {
	if port == "" {
		return nil
	}
	var peers []string
	seen := map[string]bool{port: true}
	for _, conn := range conns {
		if !slices.Contains(conn.Ends, port) {
			continue
		}
		for _, end := range conn.Ends {
			if seen[end] {
				continue
			}
			seen[end] = true
			peers = append(peers, end)
		}
	}
	return peers
}
