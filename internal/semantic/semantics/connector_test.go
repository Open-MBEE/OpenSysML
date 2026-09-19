package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// connector returns the single unnamed connector usage declared in scope: a
// `connection : X connect ...` names nothing of its own.
func connector(t *testing.T, scope *symbols.Scope) *symbols.Symbol {
	t.Helper()
	var found *symbols.Symbol
	for _, s := range scope.AllMembers() {
		if s.Name != "" || !connectorLike(s) {
			continue
		}
		if found != nil {
			t.Fatalf("more than one unnamed connector in scope")
		}
		found = s
	}
	if found == nil {
		t.Fatalf("no unnamed connector in scope")
	}
	return found
}

// TestImplicitEndRedefinitionOfConnectionUsage covers SysML v2 7.13.2: an end
// a connection usage names in its connect clause redefines the end at the same
// position of the connection definition that types it, and so takes its type.
func TestImplicitEndRedefinitionOfConnectionUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def TireBead;
		part def Rim;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : Rim;
		}
		part w {
			part t { part bead : TireBead; }
			part wheel { part rim : Rim; }
			connection : PressureSeat connect
				bead references t.bead to
				rim references wheel.rim;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "PressureSeat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	for _, name := range []string{"bead", "rim"} {
		end := nested(t, conn.Scope, name)
		supers := m.DirectSupertypes(end)
		if len(supers) != 1 || supers[0] != nested(t, seat.Scope, name) {
			t.Fatalf("DirectSupertypes(%s) = %v, want [PressureSeat::%s]", name, supers, name)
		}
	}
	// The redefined end supplies the type.
	if !m.Conforms(nested(t, conn.Scope, "bead"), nested(t, p.Scope, "TireBead")) {
		t.Fatalf("the connection's bead end does not conform to TireBead")
	}
}

// TestImplicitEndRedefinitionIsPositional covers that the match is by position,
// not by name: a renamed end redefines the end at its own position.
func TestImplicitEndRedefinitionIsPositional(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def TireBead;
		part def Rim;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : Rim;
		}
		part w {
			part t { part b; }
			part wheel { part r; }
			connection : PressureSeat connect
				outer references t.b to
				inner references wheel.r;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "PressureSeat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	if supers := m.DirectSupertypes(nested(t, conn.Scope, "outer")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "bead") {
		t.Fatalf("DirectSupertypes(outer) = %v, want [PressureSeat::bead]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, conn.Scope, "inner")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "rim") {
		t.Fatalf("DirectSupertypes(inner) = %v, want [PressureSeat::rim]", supers)
	}
}

// TestImplicitEndRedefinitionCountsUnnamedEnds covers that positions are
// counted over the whole connect clause: an end that only names what it
// attaches to still occupies its position, so a following end that declares a
// name of its own redefines the end at its own position, not the first one.
func TestImplicitEndRedefinitionCountsUnnamedEnds(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def TireBead;
		part def Rim;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : Rim;
		}
		part w {
			part wheel { part r : Rim; }
			part b : TireBead;
			connection : PressureSeat connect b to seatRim references wheel.r;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "PressureSeat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	if supers := m.DirectSupertypes(nested(t, conn.Scope, "seatRim")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "rim") {
		t.Fatalf("DirectSupertypes(seatRim) = %v, want [PressureSeat::rim]", supers)
	}
}

// TestImplicitEndRedefinitionOfInterfaceUsage covers SysML v2 7.14.2, where the
// ends are ports bound with `::>` rather than `references`.
func TestImplicitEndRedefinitionOfInterfaceUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def FuelOutPort;
		port def FuelInPort;
		interface def FuelInterface {
			end supplierPort : FuelOutPort;
			end consumerPort : FuelInPort;
		}
		part vehicle {
			part tankAssy { port fuelTankPort : FuelOutPort; }
			part eng { port engineFuelPort : FuelInPort; }
			interface : FuelInterface connect
				supplierPort ::> tankAssy.fuelTankPort to
				consumerPort ::> eng.engineFuelPort;
		}
	}`)
	p := sym(t, root, "P")
	def := nested(t, p.Scope, "FuelInterface")
	iface := connector(t, nested(t, p.Scope, "vehicle").Scope)

	for _, name := range []string{"supplierPort", "consumerPort"} {
		supers := m.DirectSupertypes(nested(t, iface.Scope, name))
		if len(supers) != 1 || supers[0] != nested(t, def.Scope, name) {
			t.Fatalf("DirectSupertypes(%s) = %v, want [FuelInterface::%s]", name, supers, name)
		}
	}
}

// TestExplicitEndRedefinitionAddsToPositional covers that an end declaring `:>>`
// redefines what it names and, as every end does, the general end at its position.
func TestExplicitEndRedefinitionAddsToPositional(t *testing.T) {
	m, root := buildModel(t, `package P {
		connection def Seat { end [1] part bead; end [1] part rim; }
		part w {
			part t { part b; }
			part wheel { part r; }
			connection : Seat connect
				second :>> Seat::rim references wheel.r to
				first :>> Seat::bead references t.b;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "Seat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	bead, rim := nested(t, seat.Scope, "bead"), nested(t, seat.Scope, "rim")
	if supers := m.DirectSupertypes(nested(t, conn.Scope, "second")); len(supers) != 2 ||
		supers[0] != rim || supers[1] != bead {
		t.Fatalf("DirectSupertypes(second) = %v, want [Seat::rim Seat::bead]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, conn.Scope, "first")); len(supers) != 2 ||
		supers[0] != bead || supers[1] != rim {
		t.Fatalf("DirectSupertypes(first) = %v, want [Seat::bead Seat::rim]", supers)
	}
	if n := m.ConnectorEndCount(conn); n != 2 {
		t.Fatalf("ConnectorEndCount = %d, want 2", n)
	}
}

// TestConnectorEndRedefinitionNegativeCases covers the boundaries of the rule:
// an end beyond the last position of the general connector redefines nothing,
// an untyped connector has nothing to redefine, and an end of a connect clause
// that names an existing feature is a reference, not a declaration.
func TestConnectorEndRedefinitionNegativeCases(t *testing.T) {
	t.Run("more ends than the general connector", func(t *testing.T) {
		m, root := buildModel(t, `package P {
			connection def Seat { end [1] part bead; }
			part w {
				part a; part b;
				connection : Seat connect e1 references a to e2 references b;
			}
		}`)
		p := sym(t, root, "P")
		conn := connector(t, nested(t, p.Scope, "w").Scope)
		if supers := m.DirectSupertypes(nested(t, conn.Scope, "e2")); len(supers) != 0 {
			t.Fatalf("DirectSupertypes(e2) = %v, want none", supers)
		}
		general, unmatched := m.UnmatchedConnectorEnds(conn)
		if general == nil || general.Name != "Seat" || len(unmatched) != 1 || unmatched[0].Name != "e2" {
			t.Fatalf("UnmatchedConnectorEnds = (%v, %v), want (Seat, [e2])", general, unmatched)
		}
	})

	t.Run("untyped connector", func(t *testing.T) {
		m, root := buildModel(t, `package P {
			part w {
				part a; part b;
				connection connect e1 references a to e2 references b;
			}
		}`)
		p := sym(t, root, "P")
		conn := connector(t, nested(t, p.Scope, "w").Scope)
		if general, unmatched := m.UnmatchedConnectorEnds(conn); general != nil || unmatched != nil {
			t.Fatalf("UnmatchedConnectorEnds = (%v, %v), want none", general, unmatched)
		}
	})

	t.Run("an end naming an existing feature declares nothing", func(t *testing.T) {
		_, root := buildModel(t, `package P {
			connection def Seat { end [1] part bead; end [1] part rim; }
			part w {
				part a; part b;
				connection : Seat connect a to b;
			}
		}`)
		p := sym(t, root, "P")
		conn := connector(t, nested(t, p.Scope, "w").Scope)
		if _, ok := conn.Scope.LookupLocal("a"); ok {
			t.Fatalf("connect a to b must not declare an end named a")
		}
	})
}

// TestImplicitEndRedefinitionOfAssociationUsage covers that the rule is stated
// over associations (SysML v2 7.13.2, KerML 7.4.5): a connection usage typed by
// an association redefines the association's ends by position too.
func TestImplicitEndRedefinitionOfAssociationUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Owner;
		part def Owned;
		assoc Ownership {
			end owner : Owner;
			end owned : Owned;
		}
		part w {
			part o : Owner;
			part d : Owned;
			connection : Ownership connect
				theOwner references o to
				theOwned references d;
		}
	}`)
	p := sym(t, root, "P")
	assoc := nested(t, p.Scope, "Ownership")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	for _, c := range []struct{ end, redefined string }{
		{"theOwner", "owner"},
		{"theOwned", "owned"},
	} {
		supers := m.DirectSupertypes(nested(t, conn.Scope, c.end))
		if len(supers) != 1 || supers[0] != nested(t, assoc.Scope, c.redefined) {
			t.Fatalf("DirectSupertypes(%s) = %v, want [Ownership::%s]", c.end, supers, c.redefined)
		}
	}
}

// TestConnectorEndReferenceIsNotASiblingEnd covers that the feature an end
// attaches to is a feature of the connector's owner, not of the connector, so a
// name it shares with a sibling end names the owner's feature. The model is
// queried before any document walk, since the walk's memo would otherwise
// supply the answer.
func TestConnectorEndReferenceIsNotASiblingEnd(t *testing.T) {
	const name = "t.sysml"
	src := `package P {
		part def TireBead;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : TireBead;
		}
		part wheelAssy {
			part outer : TireBead;
			part bead : TireBead;
			connection : PressureSeat connect
				bead references outer to
				rim references bead;
		}
	}`
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)

	pkg := sym(t, idx.DocumentRoot(name), "P")
	wheel := nested(t, pkg.Scope, "wheelAssy")
	conn := connector(t, wheel.Scope)
	got := m.ReferencedFeature(nested(t, conn.Scope, "rim"))
	if got != nested(t, wheel.Scope, "bead") {
		t.Fatalf("ReferencedFeature(rim) = %v (kind %v), want wheelAssy::bead", got, got.Kind)
	}
}

// TestImplicitEndRedefinitionCountsUnnamedBodyEnds covers that an `end` feature
// of a connector's body occupies its position even when it declares no name, so
// the ends after it match the general's ends at the right position.
func TestImplicitEndRedefinitionCountsUnnamedBodyEnds(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A;
		part def B;
		connection def Base {
			end [1] part one : A;
			end [1] part two : B;
		}
		connection def Sub specializes Base {
			end [1] : A;
			end [1] part later : B;
		}
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	sub := nested(t, p.Scope, "Sub")

	supers := m.DirectSupertypes(nested(t, sub.Scope, "later"))
	if want := nested(t, base.Scope, "two"); len(supers) == 0 || supers[len(supers)-1] != want {
		t.Fatalf("DirectSupertypes(later) = %v, want the last to be Base::two", supers)
	}
}

// A connector usage is recognized whether it names a definition or not, and
// whether it names itself or not: what makes it one is the connect clause
// (SysML v2 §7.13.2, §8.3.13).
func TestIsConnectorUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def Pt;
		part def A { port p : Pt; }
		connection def Link { end source : Pt; end target : Pt; }
		part w {
			part a : A;
			part b : A;
			attribute plain;
			part nested { port q : Pt; }
			connection typed : Link connect a.p to b.p;
			interface untyped connect a.p to b.p;
			connect a.p to b.p;
		}
	}`)
	w := nested(t, sym(t, root, "P").Scope, "w")
	for _, name := range []string{"typed", "untyped"} {
		if !m.IsConnectorUsage(nested(t, w.Scope, name)) {
			t.Errorf("IsConnectorUsage(%s) = false, want true", name)
		}
	}
	if !m.IsConnectorUsage(connector(t, w.Scope)) {
		t.Error("IsConnectorUsage(anonymous connect) = false, want true")
	}
	for _, name := range []string{"a", "plain", "nested"} {
		if m.IsConnectorUsage(nested(t, w.Scope, name)) {
			t.Errorf("IsConnectorUsage(%s) = true, want false: it connects nothing", name)
		}
	}
}

// The attachments of a connector are the features its connect clause names, in
// the order written, each under the name its end is known by — the end's own
// name when it declares one, otherwise the definition's end at that position.
func TestConnectorEndAttachments(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def Pt;
		part def A { port p : Pt; part inner { port q : Pt; } }
		connection def Link { end source : Pt; end target : Pt; }
		part w {
			part a : A;
			part b : A;
			connection typed : Link connect a.p to b.inner.q;
			connection named : Link connect
				source references a.p to
				target references b.p;
			connection tri connect (a, b, a.inner);
		}
	}`)
	w := nested(t, sym(t, root, "P").Scope, "w")

	for _, name := range []string{"typed", "named"} {
		got := m.ConnectorEndAttachments(nested(t, w.Scope, name))
		if len(got) != 2 {
			t.Fatalf("%s: %d attachments, want two", name, len(got))
		}
		for i, wantEnd := range []string{"source", "target"} {
			if got[i].Name != wantEnd {
				t.Errorf("%s: attachment %d is named %q, want %q", name, i, got[i].Name, wantEnd)
			}
			if got[i].Attachment == nil {
				t.Errorf("%s: attachment %d attaches to nothing", name, i)
			}
		}
	}

	// Every end of an n-ary connector is kept, in declaration order.
	if got := m.ConnectorEndAttachments(nested(t, w.Scope, "tri")); len(got) != 3 {
		t.Fatalf("tri: %d attachments, want three", len(got))
	}
}

// TestEndsInheritedThroughSeveralGenerals covers a connector that specializes
// two connectors: its ends redefine the end at their position in each general,
// and an inherited end that another inherited end redefines counts once.
func TestEndsInheritedThroughSeveralGenerals(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A;
		part def B;
		connection def Base {
			end [1] part a : A;
			end [1] part b : B;
		}
		connection def Refined :> Base {
			end [1] part a2 : A :>> Base::a;
			end [1] part b2 : B :>> Base::b;
		}
		connection def Both :> Refined, Base;
		part w {
			part x : A;
			part y : B;
			connection : Both connect x to y;
		}
	}`)
	p := sym(t, root, "P")
	both := nested(t, p.Scope, "Both")
	refined := nested(t, p.Scope, "Refined")
	if ends := m.endsOf(both); len(ends) != 2 ||
		ends[0] != nested(t, refined.Scope, "a2") || ends[1] != nested(t, refined.Scope, "b2") {
		t.Fatalf("endsOf(Both) = %v, want [Refined::a2 Refined::b2]", ends)
	}
	conn := connector(t, nested(t, p.Scope, "w").Scope)
	atts := m.ConnectorEndAttachments(conn)
	if len(atts) != 2 || atts[0].Name != "a2" || atts[1].Name != "b2" {
		t.Fatalf("ConnectorEndAttachments = %+v, want ends named a2 and b2", atts)
	}
}

// An unnamed `from`/`to` end inherited along two paths of a diamond is one
// effective end, so the leaf has two ends, not four; a leaf restating the pair
// redefines both by position.
func TestEndsInheritedThroughDiamondCountOnce(t *testing.T) {
	m, root := buildModel(t, `package P {
		feature x;
		feature y;
		connector base : Links::BinaryLink from x to y;
		connector mid1 :> base;
		connector mid2 :> base;
		connector leaf :> mid1, mid2;
		connector leaf2 :> mid1, mid2 from x to y;
	}`)
	p := sym(t, root, "P")
	for _, name := range []string{"leaf", "leaf2"} {
		if n := m.ConnectorEndCount(nested(t, p.Scope, name)); n != 2 {
			t.Errorf("ConnectorEndCount(%s) = %d, want 2", name, n)
		}
	}
	// One referenced end reached through two abstract intermediates relates one feature.
	m, root = buildModel(t, `package Q {
		feature x;
		abstract connector base { end feature e references x; }
		abstract connector mid1 :> base;
		abstract connector mid2 :> base;
		connector leaf :> mid1, mid2;
	}`)
	q := sym(t, root, "Q")
	if n := m.RelatedFeatureCount(nested(t, q.Scope, "leaf")); n != 1 {
		t.Errorf("RelatedFeatureCount(leaf) = %d, want 1", n)
	}
}

// An owned end claims the end at its position in every general as well as the
// ends its `:>>` clauses name — so naming an end of one general still masks the
// other general's end at that position, and only a third owned end adds arity.
func TestEndsClaimedAcrossSeveralGenerals(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def T;
		connection def A { end [1] part a1 : T; end [1] part a2 : T; }
		connection def B { end [1] part b1 : T; end [1] part b2 : T; }
		connection def OneSide :> A, B {
			end [1] part c1 : T :>> A::a1;
			end [1] part c2 : T :>> A::a2;
		}
		connection def Positional :> A, B { end [1] part d1 : T; end [1] part d2 : T; }
		connection def BothSides :> A, B {
			end [1] part e1 : T :>> A::a1, B::b1;
			end [1] part e2 : T :>> A::a2, B::b2;
		}
		connection def Swapped :> A { end [1] part f1 : T :>> A::a2; end [1] part f2 : T :>> A::a1; }
		connection def Third :> A { end [1] part g1 : T :>> A::a1; end [1] part g2 : T :>> A::a2; end [1] part g3 : T; }
	}`)
	p := sym(t, root, "P")
	ty, a, b := nested(t, p.Scope, "T"), nested(t, p.Scope, "A"), nested(t, p.Scope, "B")
	oneSide := nested(t, p.Scope, "OneSide")
	if ends := m.endsOf(oneSide); len(ends) != 2 ||
		ends[0] != nested(t, oneSide.Scope, "c1") || ends[1] != nested(t, oneSide.Scope, "c2") {
		t.Errorf("endsOf(OneSide) = %v, want [c1 c2]", ends)
	}
	if supers := m.DirectSupertypes(nested(t, oneSide.Scope, "c1")); len(supers) != 3 ||
		supers[0] != ty || supers[1] != nested(t, a.Scope, "a1") || supers[2] != nested(t, b.Scope, "b1") {
		t.Errorf("DirectSupertypes(c1) = %v, want [T A::a1 B::b1]", supers)
	}
	swapped := nested(t, p.Scope, "Swapped")
	if supers := m.DirectSupertypes(nested(t, swapped.Scope, "f1")); len(supers) != 3 ||
		supers[0] != ty || supers[1] != nested(t, a.Scope, "a2") || supers[2] != nested(t, a.Scope, "a1") {
		t.Errorf("DirectSupertypes(f1) = %v, want [T A::a2 A::a1]", supers)
	}
	for name, want := range map[string]int{"Positional": 2, "BothSides": 2, "Swapped": 2, "Third": 3} {
		if n := m.ConnectorEndCount(nested(t, p.Scope, name)); n != want {
			t.Errorf("ConnectorEndCount(%s) = %d, want %d", name, n, want)
		}
	}
}

// A binding's clause states its ends outside a `connect` clause — `of a = b` in
// KerML, `bind a = b` in SysML — so each is a related feature and occupies an
// end position beside the body's ends.
func TestBindingClauseEndsAreRelatedFeatures(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class C {
			feature x; feature y; feature z;
			binding b1 of x = y;
			binding b2 of x = y { end feature e3 references z; }
			binding b3 { end feature e1 references x; }
			binding b4;
			binding b5 :> b1;
		}
	}`)
	c := nested(t, sym(t, root, "P").Scope, "C")
	for name, want := range map[string]int{"b1": 2, "b2": 3, "b3": 1, "b4": 0, "b5": 2} {
		if n := m.RelatedFeatureCount(nested(t, c.Scope, name)); n != want {
			t.Errorf("RelatedFeatureCount(%s) = %d, want %d", name, n, want)
		}
	}
	if n := m.ConnectorEndCount(nested(t, c.Scope, "b2")); n != 3 {
		t.Errorf("ConnectorEndCount(b2) = %d, want 3", n)
	}

	m, root = buildModel(t, `package P {
		part def C {
			attribute x; attribute y; attribute z;
			binding b1 bind x = y;
			binding b2 bind x = y { end e3 references z; }
		}
	}`)
	c = nested(t, sym(t, root, "P").Scope, "C")
	for name, want := range map[string]int{"b1": 2, "b2": 3} {
		if n := m.RelatedFeatureCount(nested(t, c.Scope, name)); n != want {
			t.Errorf("RelatedFeatureCount(%s) = %d, want %d", name, n, want)
		}
	}
}
