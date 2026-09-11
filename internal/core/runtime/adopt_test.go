package runtime

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const adoptSrc = `package Demo {
	part def Engine { attribute power = 300.0; }
	part def Vehicle { attribute mass = 1500.0; part engine : Engine; }
}`

// contextOver indexes src and gives the context its text, which is what lets a
// shape be compared by what a declaration says rather than by where it sits.
func contextOver(t *testing.T, src string) *Context {
	t.Helper()
	ctx, _ := contextForSource(t, src)
	ctx.Model().RegisterSource(source.New("<test>", []byte(src)))
	return ctx
}

// vehicleIn materializes a vehicle and the engine part inside it.
func vehicleIn(t *testing.T, ctx *Context) *Instance {
	t.Helper()
	idx := ctx.model.resolver.Index()
	obj, err := ctx.Instantiate(lookupOne(t, idx, "Demo::Vehicle"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := obj.GetFeatureValue(ctx, "engine"); err != nil {
		t.Fatalf("GetFeatureValue(engine): %v", err)
	}
	return obj
}

// An object is carried into a re-analysis of the document it was materialized
// from: it keeps its identity and its values, and everything it points at is the
// declaration the new analysis produced rather than the one it was built against.
func TestAdoptCarriesAnObjectIntoAReanalysis(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)
	nested, ok := obj.FeatureValues["engine"].Value.Object()
	if !ok {
		t.Fatalf("engine feature value holds %v, want an object", obj.FeatureValues["engine"].Value)
	}

	ctx := contextOver(t, adoptSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	if got, found := ctx.Instance(obj.ID); !found || got != obj {
		t.Fatalf("Instance(%d) = %v, %v; want the carried object", obj.ID, got, found)
	}
	if _, found := ctx.Instance(nested); !found {
		t.Errorf("the engine object %d it holds was not carried with it", nested)
	}
	if obj.Type != lookupOne(t, ctx.model.resolver.Index(), "Demo::Vehicle") {
		t.Error("the object is still of the declaration it was built against")
	}
	feat := obj.FeatureValues["mass"].Feature
	if features := ctx.FeaturesOf(obj.Type); feat != &features[indexOfFeature(t, features, "mass")] {
		t.Error("a feature value still fills a feature of the analysis the object was built against")
	}
	mass, err := obj.GetFeatureValue(ctx, "mass")
	if err != nil {
		t.Fatalf("GetFeatureValue(mass): %v", err)
	}
	if got := mass.Value; got.Kind != ValConst || !strings.Contains(fmt.Sprint(got.Const), "1500") {
		t.Errorf("mass = %v, want the value its declaration states", got)
	}
	// The identities carried over are taken, so the next object gets a new one.
	next, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "Demo::Engine"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if next.ID == obj.ID || next.ID == nested {
		t.Errorf("new object took the identity %d of one carried over", next.ID)
	}
}

const adoptCalcSrc = `package Demo {
	calc def double { in x; return : ScalarValues::Real = x * 2.0; }
	part def Gauge { attribute reading = double(3.0); }
}`

// A feature value that holds what a value expression states is derived again in the context
// it is carried into, so it reads the declarations that expression names as they
// are now rather than keeping what they said when it was materialized.
func TestAdoptDerivesAValueAgainstTheNewDeclarations(t *testing.T) {
	prev := contextOver(t, adoptCalcSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Gauge"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if fv, err := obj.GetFeatureValue(prev, "reading"); err != nil {
		t.Fatalf("GetFeatureValue(reading): %v", err)
	} else if got := fmt.Sprint(fv.Value.Const); !strings.Contains(got, "6") {
		t.Fatalf("reading = %s, want 6", got)
	}
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptCalcSrc, "x * 2.0", "x * 3.0", 1))
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	fv, err := obj.GetFeatureValue(ctx, "reading")
	if err != nil {
		t.Fatalf("GetFeatureValue(reading): %v", err)
	}
	if got := fmt.Sprint(fv.Value.Const); !strings.Contains(got, "9") {
		t.Errorf("reading = %s, want 9 from the calc as it is declared now", got)
	}
}

const adoptConnectSrc = `package Demo {
	port def P;
	part def A { port p : P; }
	part def B { port q : P; }
	part def Sys { part a : A; part b : B; connect a.p to b.q; }
}`

// A connector its owner declares no name for is a member no name can be looked
// up again, so it is left to the new context to materialize rather than costing
// the object that owns it.
func TestAdoptCarriesAnObjectOwningAnAnonymousConnector(t *testing.T) {
	prev := contextOver(t, adoptConnectSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if conns, err := obj.OwnedConnectors(prev); err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	} else if len(conns) != 1 {
		t.Fatalf("the object owns %d anonymous connectors, want 1", len(conns))
	}
	before := obj.anonymous[0]
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, adoptConnectSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	conns, err := obj.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors after the carry-over: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("the carried object owns %d anonymous connectors, want 1", len(conns))
	}
	if _, found := ctx.Instance(conns[0].ID); !found {
		t.Errorf("the connector object %d is not held by the context that materialized it", conns[0].ID)
	}
	// It is the same connector of the same object, so it is named the same.
	if conns[0].ID != before {
		t.Errorf("the connector object is %d after the carry-over, want the identity %d it had", conns[0].ID, before)
	}
	port := fvInstance(t, ctx, obj, "a", "p")
	if end := conns[0].Ends[0].Value; !holdsObject(end, port.ID) {
		t.Errorf("the connector end holds %v, want the port object %d of the carried object", end, port.ID)
	}
}

// The identities of the connectors a carry-over set aside are known without
// materializing them, and asking for one materializes it again under its identity.
func TestAdoptKeepsTheIdentitiesOfConnectorsSetAside(t *testing.T) {
	src := strings.Replace(adoptConnectSrc, "connect a.p to b.q;", "connect a.p to b.q; connection c connect a.p to b.q;", 1)
	prev := contextOver(t, src)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := obj.KeptConnectorIDs(); len(got) != 0 {
		t.Fatalf("KeptConnectorIDs() = %v before any carry-over, want none", got)
	}
	conns, err := obj.OwnedConnectors(prev)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	named := fvInstance(t, prev, obj, "c")
	anon := conns[0].ID
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	want := []int64{min(anon, named.ID), max(anon, named.ID)}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("KeptConnectorIDs() = %v after the carry-over, want %v", got, want)
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 0 {
		t.Errorf("MaterializedConnectors() = %v, want none until they are asked for", got)
	}
	held := len(ctx.InstanceIDs())
	if got, err := obj.RestoreConnector(ctx, 99); got != nil || err != nil {
		t.Errorf("RestoreConnector(99) = %v, %v; want nothing for an identity never kept", got, err)
	}
	if len(ctx.InstanceIDs()) != held {
		t.Fatalf("asking after the identities materialized objects: %v", ctx.InstanceIDs())
	}

	for _, id := range []int64{anon, named.ID} {
		conn, err := obj.RestoreConnector(ctx, id)
		if err != nil {
			t.Fatalf("RestoreConnector(%d): %v", id, err)
		}
		if conn == nil || conn.ID != id {
			t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", id, conn)
		}
		if got, found := ctx.Instance(id); !found || got != conn {
			t.Errorf("Instance(%d) = %v, %v; want the connector materialized again", id, got, found)
		}
		port := fvInstance(t, ctx, obj, "a", "p")
		if end := conn.Ends[0].Value; !holdsObject(end, port.ID) {
			t.Errorf("connector %d's end holds %v, want the port object %d", id, end, port.ID)
		}
	}
	if got := obj.KeptConnectorIDs(); len(got) != 0 {
		t.Errorf("KeptConnectorIDs() = %v once both are materialized again, want none", got)
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 1 || got[0].ID != anon {
		t.Errorf("MaterializedConnectors() = %v, want the anonymous connector %d", got, anon)
	}
	if len(ctx.InstanceIDs()) != held+2 {
		t.Errorf("materializing the two connectors again left %v, want the two objects more", ctx.InstanceIDs())
	}
}

// carriedSysWithThreeConnectors materializes a Sys owning three anonymous
// connectors, carries it into a re-analysis, and returns the carried object with
// the identities its connectors had, in declaration order.
func carriedSysWithThreeConnectors(t *testing.T) (*Context, *Instance, []int64) {
	t.Helper()
	src := strings.Replace(adoptConnectSrc, "connect a.p to b.q;", "connect a.p to b.q; connect a.p to b.q; connect a.p to b.q;", 1)
	prev := contextOver(t, src)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	conns, err := obj.OwnedConnectors(prev)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	if len(conns) != 3 {
		t.Fatalf("the object owns %d anonymous connectors, want 3", len(conns))
	}
	ids := make([]int64, 0, 3)
	for _, conn := range conns {
		ids = append(ids, conn.ID)
	}
	shapes := prev.ShapesOf(obj)
	ctx := contextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	return ctx, obj, ids
}

// restoreCost measures the steps restoring one of those connectors alone spends.
func restoreCost(t *testing.T) int64 {
	t.Helper()
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	spent := ctx.run.steps
	if _, err := obj.RestoreConnector(ctx, ids[2]); err != nil {
		t.Fatalf("RestoreConnector(%d): %v", ids[2], err)
	}
	return ctx.run.steps - spent
}

// Asking for one connector a carry-over set aside materializes that connector
// alone: its siblings stay set aside, under their identities, until each is asked
// for — so a budget that admits the one asked for does not have to admit them all.
func TestRestoreConnectorMaterializesTheOneAskedForAlone(t *testing.T) {
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	held := len(ctx.InstanceIDs())
	last, err := obj.RestoreConnector(ctx, ids[2])
	if err != nil {
		t.Fatalf("RestoreConnector(%d): %v", ids[2], err)
	}
	if last == nil || last.ID != ids[2] {
		t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", ids[2], last)
	}
	// The ends read objects carried with the owner, so the connector is all that is new.
	if got := len(ctx.InstanceIDs()); got != held+1 {
		t.Errorf("restoring one connector left %d objects, want %d: it alone", got, held+1)
	}
	for _, id := range ids[:2] {
		if _, found := ctx.Instance(id); found {
			t.Errorf("restoring %d materialized its sibling %d", ids[2], id)
		}
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[:2]) {
		t.Errorf("KeptConnectorIDs() = %v after restoring %d, want its siblings %v still set aside", got, ids[2], ids[:2])
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 1 || got[0].ID != ids[2] {
		t.Errorf("MaterializedConnectors() = %v, want the one restored, %d", got, ids[2])
	}

	// A budget admitting exactly the one asked for admits it, whatever its siblings would cost.
	cost := restoreCost(t)
	ctx, obj, ids = carriedSysWithThreeConnectors(t)
	ctx.maxSteps = ctx.run.steps + cost
	if conn, err := obj.RestoreConnector(ctx, ids[2]); err != nil || conn == nil || conn.ID != ids[2] {
		t.Fatalf("RestoreConnector(%d) under a budget admitting it alone = %v, %v; want the connector", ids[2], conn, err)
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[:2]) {
		t.Errorf("KeptConnectorIDs() = %v, want the siblings %v still set aside", got, ids[:2])
	}
	// The siblings take their identities back when asked for, in either order.
	ctx.maxSteps = ctx.run.steps + 2*cost
	for _, id := range []int64{ids[0], ids[1]} {
		conn, err := obj.RestoreConnector(ctx, id)
		if err != nil || conn == nil || conn.ID != id {
			t.Fatalf("RestoreConnector(%d) = %v, %v; want the connector under that identity", id, conn, err)
		}
	}
	if got := obj.KeptConnectorIDs(); len(got) != 0 {
		t.Errorf("KeptConnectorIDs() = %v once all are materialized again, want none", got)
	}
	conns, err := obj.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	var got []int64
	for _, conn := range conns {
		got = append(got, conn.ID)
	}
	if fmt.Sprint(got) != fmt.Sprint(ids) {
		t.Errorf("OwnedConnectors() = %v, want the connectors under their identities %v in declaration order", got, ids)
	}
}

// A probe that restores one set-aside connector discards it with the probe and
// leaves the object as it found it: every identity kept, the one restored
// included, and what was materialized before still so.
func TestProbedRestorationLeavesTheIdentitiesKept(t *testing.T) {
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	if first, err := obj.RestoreConnector(ctx, ids[0]); err != nil || first == nil || first.ID != ids[0] {
		t.Fatalf("RestoreConnector(%d) = %v, %v; want the connector", ids[0], first, err)
	}
	held := len(ctx.InstanceIDs())

	end := ctx.beginProbe()
	mark := len(ctx.created)
	if last, err := obj.RestoreConnector(ctx, ids[2]); err != nil || last == nil || last.ID != ids[2] {
		t.Fatalf("RestoreConnector(%d) under a probe = %v, %v; want the connector under that identity", ids[2], last, err)
	}
	ctx.abandonInstancesSince(mark)
	end()

	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[1:]) {
		t.Errorf("KeptConnectorIDs() after the probe = %v, want %v set aside as before it", got, ids[1:])
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 1 || got[0].ID != ids[0] {
		t.Errorf("MaterializedConnectors() after the probe = %v, want the one restored before it, %d", got, ids[0])
	}
	if got := len(ctx.InstanceIDs()); got != held {
		t.Errorf("the probe left %d objects, want %d as before it", got, held)
	}
	if last, err := obj.RestoreConnector(ctx, ids[2]); err != nil || last == nil || last.ID != ids[2] {
		t.Errorf("RestoreConnector(%d) after the probe = %v, %v; want the connector under that identity", ids[2], last, err)
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[1:2]) {
		t.Errorf("KeptConnectorIDs() = %v, want the sibling %v alone still set aside", got, ids[1:2])
	}
}

// A restoration that fails leaves every identity set aside, the one asked for
// included, and no object behind, so asking again under a budget that admits it
// materializes the connector under its identity.
func TestRestoreConnectorThatFailsKeepsEveryIdentity(t *testing.T) {
	cost := restoreCost(t)
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	held := ctx.InstanceIDs()
	ctx.maxSteps = ctx.run.steps + cost - 1
	conn, err := obj.RestoreConnector(ctx, ids[1])
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("RestoreConnector(%d) one step short = %v, %v; want the budget spent", ids[1], conn, err)
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids) {
		t.Errorf("KeptConnectorIDs() = %v after the failure, want all of %v still set aside", got, ids)
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 0 {
		t.Errorf("MaterializedConnectors() = %v after the failure, want none", got)
	}
	for _, id := range ids {
		if _, found := ctx.Instance(id); found {
			t.Errorf("the failed restoration left connector %d behind", id)
		}
	}

	ctx.maxSteps = ctx.Budgets().MaxSteps + 1000
	conn, err = obj.RestoreConnector(ctx, ids[1])
	if err != nil || conn == nil || conn.ID != ids[1] {
		t.Fatalf("RestoreConnector(%d) asked again = %v, %v; want the connector under that identity", ids[1], conn, err)
	}
	port := fvInstance(t, ctx, obj, "a", "p")
	if end := conn.Ends[0].Value; !holdsObject(end, port.ID) {
		t.Errorf("the connector's end holds %v, want the port object %d", end, port.ID)
	}
	want := []int64{ids[0], ids[2]}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("KeptConnectorIDs() = %v, want the siblings %v still set aside", got, want)
	}
	if got := ctx.InstanceIDs(); len(got) != len(held)+1 {
		t.Errorf("the objects held are %v, want those before, %v, and the connector alone", got, held)
	}
}

const adoptTwoConnectSrc = `package Demo {
	port def P;
	part def A { port p : P; port r : P; }
	part def B { port q : P; port s : P; }
	part def Sys { part a : A; part b : B; connect a.p to b.q; connect a.r to b.s; }
}`

// endsOf names the ports the connector's ends hold, `a.p-b.q`, on the object owning them.
func endsOf(t *testing.T, ctx *Context, owner, conn *Instance) string {
	t.Helper()
	var names []string
	for _, end := range conn.Ends {
		id, held := end.Value.Object()
		if !held {
			t.Fatalf("connector %d's end holds %v, want a port object", conn.ID, end.Value)
		}
		for _, path := range [][2]string{{"a", "p"}, {"a", "r"}, {"b", "q"}, {"b", "s"}} {
			if fvInstance(t, ctx, owner, path[0], path[1]).ID == id {
				names = append(names, path[0]+"."+path[1])
			}
		}
	}
	return strings.Join(names, "-")
}

// A kept identity follows its declaration wherever it now stands among its siblings;
// the identity of a declaration edited away or removed is dropped, not handed on.
func TestKeptConnectorIdentitiesFollowTheirDeclarations(t *testing.T) {
	const (
		first  = "connect a.p to b.q;"
		second = "connect a.r to b.s;"
	)
	cases := []struct {
		name  string
		decls string
		// want maps the ends of each connector materialized before to those the
		// connector under its identity has after, "" for an identity dropped.
		want map[string]string
		// fresh is the connectors the new declarations add, by ends.
		fresh []string
	}{
		{"inserted before", "connect a.p to b.s; " + first + " " + second, map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": "a.r-b.s"}, []string{"a.p-b.s"}},
		{"reordered", second + " " + first, map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": "a.r-b.s"}, nil},
		{"edited", first + " connect a.r to b.q;", map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": ""}, []string{"a.r-b.q"}},
		{"removed", first, map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": ""}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prev := contextOver(t, adoptTwoConnectSrc)
			owner, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Sys"))
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			conns, err := owner.OwnedConnectors(prev)
			if err != nil {
				t.Fatalf("OwnedConnectors: %v", err)
			}
			before := make(map[string]int64, len(conns))
			for _, conn := range conns {
				before[endsOf(t, prev, owner, conn)] = conn.ID
			}
			if len(before) != 2 {
				t.Fatalf("the connectors before are %v, want two with distinct ends", before)
			}
			shapes := prev.ShapesOf(owner)

			ctx := contextOver(t, strings.Replace(adoptTwoConnectSrc, first+" "+second, tc.decls, 1))
			if _, err := ctx.Adopt(prev, shapes, owner); err != nil {
				t.Fatalf("Adopt: %v", err)
			}
			var kept []int64
			for ends, id := range before {
				if tc.want[ends] != "" {
					kept = append(kept, id)
				}
			}
			slices.Sort(kept)
			if got := owner.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(kept) {
				t.Errorf("KeptConnectorIDs() = %v, want %v: the identities of the declarations still there", got, kept)
			}
			for ends, id := range before {
				conn, err := owner.RestoreConnector(ctx, id)
				if err != nil {
					t.Fatalf("RestoreConnector(%d): %v", id, err)
				}
				if tc.want[ends] == "" {
					if conn != nil {
						t.Errorf("RestoreConnector(%d) of the %s connector = the %s connector, want nothing: its declaration is gone", id, ends, endsOf(t, ctx, owner, conn))
					}
					continue
				}
				if conn == nil || conn.ID != id {
					t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", id, conn)
				}
				if got := endsOf(t, ctx, owner, conn); got != tc.want[ends] {
					t.Errorf("connector %d connects %s after the carry-over, want %s as before", id, got, tc.want[ends])
				}
			}
			after, err := owner.OwnedConnectors(ctx)
			if err != nil {
				t.Fatalf("OwnedConnectors after the carry-over: %v", err)
			}
			var fresh []string
			for _, conn := range after {
				if slices.Contains(slices.Collect(maps.Values(before)), conn.ID) {
					continue
				}
				fresh = append(fresh, endsOf(t, ctx, owner, conn))
			}
			slices.Sort(fresh)
			if fmt.Sprint(fresh) != fmt.Sprint(tc.fresh) {
				t.Errorf("the connectors under new identities connect %v, want %v", fresh, tc.fresh)
			}
			if got := owner.KeptConnectorIDs(); len(got) != 0 {
				t.Errorf("KeptConnectorIDs() = %v once all are materialized again, want none", got)
			}
		})
	}
}

// holdsObject reports whether the value is the object of the given identity.
func holdsObject(val Value, id int64) bool {
	got, ok := val.Object()
	return ok && got == id
}

const adoptClassifiedSrc = `package Demo {
	private import ScalarValues::*;
	item def Segment { attribute span : Real = 2.0; item ends [2]; }
	item def Loop { item edges : Segment [*]; }
	item def Square :> Loop {
		item :>> edges [4] = (e1, e2, e3, e4);
		item e1 [1]; item e2 [1]; item e3 [1]; item e4 [1];
	}
}`

// A classification carries over adoption: the feature is rebound to its declaration in
// the new analysis and the feature values it added fill the features declared there.
func TestAdoptCarriesAClassifiedObject(t *testing.T) {
	prev := contextOver(t, adoptClassifiedSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Square"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := obj.GetFeatureValue(prev, "edges"); err != nil {
		t.Fatalf("GetFeatureValue(edges): %v", err)
	}
	id, ok := obj.FeatureValues["e1"].Value.Object()
	if !ok {
		t.Fatalf("e1 holds %v, want an object", obj.FeatureValues["e1"].Value)
	}
	e1 := prev.instances[id]
	if len(e1.classifiers) != 1 {
		t.Fatalf("e1 has %d classifiers, want the edges feature alone", len(e1.classifiers))
	}
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, adoptClassifiedSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	edges := lookupOne(t, ctx.model.resolver.Index(), "Demo::Square::edges")
	if len(e1.classifiers) != 1 || e1.classifiers[0] != edges {
		t.Errorf("e1 is classified by %v, want the edges feature of the new analysis", e1.classifiers)
	}
	if !ctx.instanceConforms(e1, lookupOne(t, ctx.model.resolver.Index(), "Demo::Segment")) {
		t.Error("e1 no longer conforms to Segment in the new analysis")
	}
	feat := e1.FeatureValues["span"].Feature
	if features := ctx.FeaturesOf(edges); feat != &features[indexOfFeature(t, features, "span")] {
		t.Error("span still fills a feature of the analysis e1 was classified in")
	}
	span, err := e1.GetFeatureValue(ctx, "span")
	if err != nil {
		t.Fatalf("GetFeatureValue(span): %v", err)
	}
	if got := fmt.Sprint(span.Value.Const); !strings.Contains(got, "2") {
		t.Errorf("span = %s, want 2 from Segment as it is declared now", got)
	}
}

const adoptRefinedSrc = `package Demo {
	private import ScalarValues::*;
	item def Engine;
	item def V8 :> Engine;
	item def Car { attribute tags : String [0..2]; item engine : Engine [1]; }
	item def Coupe :> Car { attribute :>> tags : String [0..1]; item :>> engine : V8; }
	item def Rack { item raw : Car [1]; item coupe : Coupe [1] = raw; }
}`

// A carried feature a classifier refined keeps reading the refining declaration across
// adoption, so the narrowed type and multiplicity still govern its reads and writes.
func TestAdoptKeepsTheFeaturesAClassifierRefined(t *testing.T) {
	prev := contextOver(t, adoptRefinedSrc)
	rack, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Rack"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := rack.GetFeatureValue(prev, "coupe"); err != nil {
		t.Fatalf("GetFeatureValue(coupe): %v", err)
	}
	id, ok := rack.FeatureValues["raw"].Value.Object()
	if !ok {
		t.Fatalf("raw holds %v, want an object", rack.FeatureValues["raw"].Value)
	}
	raw := prev.instances[id]
	if err := raw.SetFeatureValue(prev, "tags", sequenceOf([]Value{NewStringValue("x")})); err != nil {
		t.Fatalf("write raw.tags: %v", err)
	}
	shapes := prev.ShapesOf(rack)

	ctx := contextOver(t, adoptRefinedSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, rack); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	idx := ctx.model.resolver.Index()
	coupe := lookupOne(t, idx, "Demo::Rack::coupe")
	if len(raw.classifiers) != 1 || raw.classifiers[0] != coupe {
		t.Fatalf("raw is classified by %v, want the coupe feature of the new analysis", raw.classifiers)
	}
	features := ctx.FeaturesOf(coupe)
	for _, name := range []string{"tags", "engine"} {
		if feat := raw.FeatureValues[name].Feature; feat != &features[indexOfFeature(t, features, name)] {
			t.Errorf("raw.%s reads %s's declaration after adoption, want Coupe's refinement", name, feat.OwnerType.Name)
		}
	}
	if err := raw.SetFeatureValue(ctx, "tags", sequenceOf([]Value{NewStringValue("x"), NewStringValue("y")})); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("write of two tags = %v, want ErrMultiplicityViolation under Coupe's tags [0..1]", err)
	}
	tags, err := raw.GetFeatureValue(ctx, "tags")
	if err != nil {
		t.Fatalf("GetFeatureValue(tags): %v", err)
	}
	if got := FormatValue(tags.HeldValue()); !strings.Contains(got, "x") || strings.Contains(got, "y") {
		t.Errorf("raw.tags = %s after the refused write, want the carried (\"x\")", got)
	}
	engine, err := raw.GetFeatureValue(ctx, "engine")
	if err != nil {
		t.Fatalf("GetFeatureValue(engine): %v", err)
	}
	if held, ok := engine.Value.Object(); !ok || !ctx.instanceConforms(ctx.instances[held], lookupOne(t, idx, "Demo::V8")) {
		t.Errorf("raw.engine = %s after adoption, want an object conforming to Coupe's engine : V8", FormatValue(engine.Value))
	}
}

// A declaration that no longer resolves to the shape an object was materialized
// against cannot hold that object, so it is refused rather than rebound onto
// feature values that no longer mean the same thing.
func TestAdoptRefusesAChangedShape(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptSrc, "mass = 1500.0", "mass = 900.0", 1))
	_, err := ctx.Adopt(prev, shapes, obj)
	if err == nil {
		t.Fatal("Adopt accepted an object of a declaration that changed")
	}
	if _, found := ctx.Instance(obj.ID); found {
		t.Error("the refused object was left in the context")
	}
}

// A change to a declaration an object only depends on invalidates it too: its
// feature values hold what that declaration says.
func TestAdoptRefusesAChangedDependency(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptSrc, "power = 300.0", "power = 100.0", 1))
	if _, err := ctx.Adopt(prev, shapes, obj); err == nil {
		t.Fatal("Adopt accepted an object whose engine declaration changed")
	}
}

// The objects are shared with the context they came from, which a run started
// before the re-analysis still materializes through, so every context that holds
// them hands out identities from one sequence — including the first one, several
// re-analyses later.
func TestAdoptSharesTheIdentitySequence(t *testing.T) {
	first := contextOver(t, adoptSrc)
	obj := vehicleIn(t, first)
	shapes := first.ShapesOf(obj)

	ctx := first
	for _, extra := range []string{"\npart def Widget;", "\npart def Gadget;"} {
		next := contextOver(t, adoptSrc+extra)
		if _, err := next.Adopt(ctx, shapes, obj); err != nil {
			t.Fatalf("Adopt: %v", err)
		}
		ctx = next
	}

	handed := map[int64]bool{}
	for _, id := range []int64{ctx.allocateID(), first.allocateID(), ctx.allocateID()} {
		if _, found := ctx.Instance(id); found {
			t.Errorf("identity %d is handed out again while the object holding it is live", id)
		}
		if handed[id] {
			t.Errorf("identity %d was handed out twice", id)
		}
		handed[id] = true
	}
}

const adoptBehaviorSrc = `package Demo {
	part def Monitor {
		attribute count = 0;
		exhibit state modes {
			entry; then idle;
			state idle { entry action bump { assign count := count + 1; } }
		}
	}
}`

// A carried object's behavior is started again in the context carrying it: the
// object keeps its identity, its execution is a new one bound to the new
// analysis, and what the discarded run wrote is not read by the new one.
func TestAdoptRestartsACarriedObjectsBehavior(t *testing.T) {
	prev := contextOver(t, adoptBehaviorSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Monitor"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	before, ok := obj.ExhibitedState()
	if !ok {
		t.Fatal("the object exhibits no machine")
	}
	id, shapes := obj.ID, prev.ShapesOf(obj)

	ctx := contextOver(t, adoptBehaviorSrc+"\npart def Widget;")
	restarted, err := ctx.Adopt(prev, shapes, obj)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if len(restarted) != 1 || !strings.Contains(restarted[0], "modes") {
		t.Errorf("Adopt reported %v restarted, want the machine modes", restarted)
	}
	if obj.ID != id {
		t.Errorf("the carried object has identity %d, want %d", obj.ID, id)
	}
	after, ok := obj.ExhibitedState()
	if !ok {
		t.Fatal("the carried object runs no machine after the carry-over")
	}
	if after.State == before.State {
		t.Error("the carried object still runs the execution of the previous analysis")
	}
	// The entry action ran once in the new execution, over the declared initial
	// value rather than over the 1 the discarded run left.
	count, err := obj.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatalf("GetFeatureValue(count): %v", err)
	}
	if got := count.Value; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("count = %v, want 1: the restarted machine read the declared initial value", got)
	}
}

const adoptNestedSrc = `package Demo {
	part def Bolt { attribute torque = 1.0; }
	part def Engine { part bolt : Bolt; part nut : Bolt; }
	part def Vehicle { part engine : Engine; part spare : Engine; }
}`

// Every carried part begins with its whole, whatever order the objects are
// carried in, and a carry-over of a carry-over keeps that so.
func TestAdoptBeginsEveryPartWithItsWhole(t *testing.T) {
	prev := contextOver(t, adoptNestedSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Vehicle"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	var held []int64
	for _, engine := range []string{"engine", "spare"} {
		fv, err := obj.GetFeatureValue(prev, engine)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", engine, err)
		}
		id, _ := fv.HeldValue().Object()
		held = append(held, id)
		inner, _ := prev.Instance(id)
		for _, bolt := range []string{"bolt", "nut"} {
			fv, err := inner.GetFeatureValue(prev, bolt)
			if err != nil {
				t.Fatalf("GetFeatureValue(%s.%s): %v", engine, bolt, err)
			}
			id, _ := fv.HeldValue().Object()
			held = append(held, id)
		}
	}
	for round := 0; round < 8; round++ {
		ctx := contextOver(t, adoptNestedSrc+fmt.Sprintf("\npart def Widget%d;", round))
		if _, err := ctx.Adopt(prev, prev.ShapesOf(obj), obj); err != nil {
			t.Fatalf("round %d: Adopt: %v", round, err)
		}
		whole, ok := ctx.OccurrenceLife(obj.ID)
		if !ok || !whole.Alive() {
			t.Fatalf("round %d: OccurrenceLife(vehicle) = %v, %v; want alive", round, whole, ok)
		}
		for _, id := range held {
			if part, ok := ctx.OccurrenceLife(id); !ok || part.Began != whole.Began || part.Ended != 0 {
				t.Errorf("round %d: OccurrenceLife(#%d) = %v, %v; want began at %d, alive", round, id, part, ok, whole.Began)
			}
		}
		prev = ctx
	}
}

const adoptCompletingSrc = `package Demo {
	part def Counter {
		attribute count = 0;
		perform action go {
			first start;
			then action bump { assign count := count + 1; }
			then done;
		}
	}
}`

// A destroyed object stays destroyed across a carry-over: the behavior it
// performed is not started again over it, so nothing writes to what has ended.
func TestAdoptKeepsADestroyedObjectFromPerformingAgain(t *testing.T) {
	prev := contextOver(t, adoptCompletingSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Counter"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if count, err := obj.GetFeatureValue(prev, "count"); err != nil || count.Value.Const.Int != 1 {
		t.Fatalf("count = %v, %v; want 1 after the action completed", count, err)
	}
	if err := prev.destroy(obj); err != nil {
		t.Fatalf("destroy: %v", err)
	}

	ctx := contextOver(t, adoptCompletingSrc+"\npart def Widget;")
	restarted, err := ctx.Adopt(prev, prev.ShapesOf(obj), obj)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if len(restarted) != 0 {
		t.Errorf("Adopt restarted %v over a destroyed object, want nothing", restarted)
	}
	if l, ok := ctx.OccurrenceLife(obj.ID); !ok || !l.Destroyed {
		t.Errorf("OccurrenceLife = %v, %v; want destroyed", l, ok)
	}
	if len(obj.Behaviors()) != 0 {
		t.Errorf("the destroyed object performs %d behaviors, want none", len(obj.Behaviors()))
	}
	if _, err := obj.GetFeatureValue(ctx, "count"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("read of the carried destroyed object: %v, want %v", err, ErrOccurrenceDestroyed)
	}
}

// A behavior that cannot be started again in the new context refuses the
// carry-over rather than leaving an object running nothing, and the object is not
// left registered in a context that cannot offer it.
func TestAdoptRefusesAnObjectWhoseBehaviorCannotRestart(t *testing.T) {
	prev := contextOver(t, adoptBehaviorSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Monitor"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, adoptBehaviorSrc+"\npart def Widget;")
	tight := DefaultBudgets()
	tight.MaxSteps = 1
	if err := ctx.SetBudgets(tight); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	if _, err := ctx.Adopt(prev, shapes, obj); err == nil {
		t.Fatal("Adopt accepted an object whose behavior could not be started")
	}
	if _, found := ctx.Instance(obj.ID); found {
		t.Error("the refused object was left in the context")
	}
	if len(obj.Behaviors()) != 0 {
		t.Errorf("the refused object still holds %d behaviors", len(obj.Behaviors()))
	}
	// Neither the refused object nor anything its restart registered stays behind.
	if _, ok := ctx.OccurrenceLife(obj.ID); ok {
		t.Error("the refused object was left a lifetime in the context")
	}
	if len(ctx.lives) != 0 || len(ctx.instances) != 0 {
		t.Errorf("the refused carry-over left %d lifetimes and %d objects in the context", len(ctx.lives), len(ctx.instances))
	}
}

// Changed names the declaration a context resolves differently, which is what a
// caller reports as having invalidated the state it holds.
func TestChangedNamesTheDeclarationThatMoved(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	shapes := prev.ShapesOf(vehicleIn(t, prev))

	if fqn, ok := contextOver(t, adoptSrc+"\npart def Widget;").Changed(shapes); ok {
		t.Errorf("Changed reported %q over an unrelated declaration", fqn)
	}
	// A change to the engine is a change to what a vehicle is, but what moved is
	// the engine, so that is what is named rather than the type holding it.
	ctx := contextOver(t, strings.Replace(adoptSrc, "power = 300.0", "power = 100.0", 1))
	fqn, ok := ctx.Changed(shapes)
	if !ok || fqn != "Demo::Engine" {
		t.Errorf("Changed() = %q, %v; want Demo::Engine", fqn, ok)
	}
}

// An edit confined to a body governing over an inherited value changes what
// instantiating produces, so the shape follows it and the object is not carried
// with the value that body replaced.
func TestShapeFollowsAGoverningValueBody(t *testing.T) {
	const src = `package Demo {
	attribute def Cost { attribute v = 1.0; }
	part def Ring { attribute template : Cost { attribute :>> v = 9.0; } attribute cost : Cost = template; }
	part def Band :> Ring { attribute :>> cost { attribute :>> v = 11.0; } }
}`
	prev := contextOver(t, src)
	sym := lookupOne(t, prev.Resolver().Index(), "Demo::Band")
	before := prev.ShapeDigest(sym)

	ctx := contextOver(t, strings.Replace(src, "v = 11.0", "v = 12.0", 1))
	if after := ctx.ShapeDigest(lookupOne(t, ctx.model.resolver.Index(), "Demo::Band")); after == before {
		t.Errorf("shape unchanged by an edit to the governing body: %s", after)
	}
}

// indexOfFeature is the position of the named effective feature.
func indexOfFeature(t *testing.T, features []EffectiveFeature, name string) int {
	t.Helper()
	for i := range features {
		if features[i].Name == name {
			return i
		}
	}
	t.Fatalf("no feature %q among %d", name, len(features))
	return 0
}

const crateSrc = `package Demo {
	private import Shapes::*;
	part def Crate { part box : Box; }
}`

// crateContextOver indexes the crate model over a Shapes library of its own,
// Domain content whose text the index vouches for unless vouch is false.
func crateContextOver(t *testing.T, lib string, vouch bool) *Context {
	t.Helper()
	idx := symbols.NewIndex()
	idx.AddDocument("Shapes.sysml", parser.New(source.New("Shapes.sysml", []byte(lib))).ParseFile())
	doc := symbols.LibraryDocument{Tier: symbols.TierDomain}
	if vouch {
		doc.Digest = symbols.TextDigest([]byte(lib))
	}
	idx.MarkLibraryDocument("Shapes.sysml", doc)
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(crateSrc))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
	ctx.Model().RegisterSource(source.New("<test>", []byte(crateSrc)))
	ctx.Model().RegisterSource(source.New("Shapes.sysml", []byte(lib)))
	return ctx
}

// crateIn materializes a crate and the box part inside it.
func crateIn(t *testing.T, ctx *Context) *Instance {
	t.Helper()
	obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "Demo::Crate"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := obj.GetFeatureValue(ctx, "box"); err != nil {
		t.Fatalf("GetFeatureValue(box): %v", err)
	}
	return obj
}

const adoptStructuredSrc = `package Demo {
	private import ScalarValues::*;
	private import Collections::*;
	private import VectorValues::*;
	attribute def LabeledGrid :> Array { attribute label : String; }
	attribute def Tagged :> NumericalVectorValue { attribute tag : String default "v"; }
	attribute grid : LabeledGrid { :>> dimensions = (2, 2); :>> elements = (1, 2, 3, 4); :>> label = "grid"; }
	attribute vec : Tagged { :>> dimension = 2; :>> elements = (1, 2); }
	part def Holder { attribute cells : LabeledGrid; attribute axis : Tagged; }
	part holder : Holder;
}`

// libraryContextOver indexes src over the standard library and gives the context its text.
func libraryContextOver(t *testing.T, src string) *Context {
	t.Helper()
	ctx, _ := libraryModelContext(t, src)
	ctx.Model().RegisterSource(source.New("<test>", []byte(src)))
	return ctx
}

// An array or vector a run wrote is carried with the object it was read from, so
// the members its specialization adds are still answered after the carry-over.
func TestAdoptCarriesTheObjectsBehindWrittenArraysAndVectors(t *testing.T) {
	prev := libraryContextOver(t, adoptStructuredSrc)
	scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
	holder, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::holder"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	backing := make(map[string]int64)
	for feature, src := range map[string]string{"cells": "grid", "axis": "vec"} {
		val, err := evalIn(t, prev, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if backing[feature] = backingObject(val); backing[feature] == 0 {
			t.Fatalf("%s = %s is backed by no object", src, FormatValue(val))
		}
		if err := holder.SetFeatureValue(prev, feature, val); err != nil {
			t.Fatalf("write %s: %v", feature, err)
		}
	}
	shapes := prev.ShapesOf(holder)

	ctx := libraryContextOver(t, adoptStructuredSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, holder); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	for feature, id := range backing {
		obj, found := ctx.Instance(id)
		if !found {
			t.Errorf("the object %d behind %s was not carried", id, feature)
			continue
		}
		if fqn := ctx.fqnOf(obj.Type); fqn == "" || obj.Type != lookupOne(t, ctx.model.resolver.Index(), fqn) {
			t.Errorf("the object behind %s is still of the declaration it was built against", feature)
		}
	}
	for expr, want := range map[string]string{
		"holder.cells":       "Array(2, 2)[1, 2, 3, 4]",
		"holder.cells.label": `"grid"`,
		"holder.cells.rank":  "2",
		"holder.axis":        "⟨1, 2⟩",
		"holder.axis.tag":    `"v"`,
	} {
		got, err := evalIn(t, ctx, lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope, expr)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s after the carry-over = %s, %v; want %s", expr, FormatValue(got), err, want)
		}
	}
}

// A shape digest names a library type instead of expanding it, so it must say
// which library: two indexes loaded on their own may declare a type of the same
// name with different features, and an object of one is refused by the other.
func TestAdoptRefusesASameNamedLibraryTypeOfAnotherLibrary(t *testing.T) {
	const lib = `package Shapes { part def Box { attribute n = 1; } }`
	prev := crateContextOver(t, lib, true)
	obj := crateIn(t, prev)
	shapes := prev.ShapesOf(obj)

	same := crateContextOver(t, lib, true)
	if _, err := same.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt over the same library: %v", err)
	}

	other := crateContextOver(t, strings.Replace(lib, "attribute n = 1;", "attribute n = 1; attribute m = 2;", 1), true)
	if _, err := other.Adopt(prev, shapes, obj); err == nil {
		t.Fatal("Adopt accepted an object whose box type gained a feature in the other library")
	}
	if _, found := other.Instance(obj.ID); found {
		t.Error("the refused object was left in the context")
	}
}

// A library whose text the index does not vouch for is expanded like the model,
// so the digest still follows what its types declare.
func TestShapeDigestExpandsALibraryOfUnknownText(t *testing.T) {
	const lib = `package Shapes { part def Box { attribute n = 1; } }`
	ctx := crateContextOver(t, lib, false)
	if _, known := ctx.model.resolver.Index().LibraryIdentity(); known {
		t.Fatal("LibraryIdentity is known for a library document of no digest")
	}
	digest := ctx.ShapeDigest(lookupOne(t, ctx.model.resolver.Index(), "Demo::Crate"))
	if !strings.Contains(digest, "Shapes::Box/partDef{n:1..1=1@") {
		t.Errorf("digest names the library type without expanding it: %s", digest)
	}
	other := crateContextOver(t, strings.Replace(lib, "n = 1", "n = 2", 1), false)
	if other.ShapeDigest(lookupOne(t, other.Resolver().Index(), "Demo::Crate")) == digest {
		t.Error("digest unchanged by an edit to the library type it expands")
	}
}

const adoptUnitSrc = `package Demo {
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;
	attribute furlong : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 201.168; } }
	part def Field { attribute width : LengthValue; attribute unit : LengthUnit; }
	part field : Field;
}`

// A quantity or measurement reference a run wrote names unit declarations, which
// are rebound to the declarations of the re-analysis like everything else the
// object points at, so the reference still answers its declaration's type.
func TestAdoptRebindsTheUnitsAWrittenValueNames(t *testing.T) {
	prev := libraryContextOver(t, adoptUnitSrc)
	scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
	field, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::field"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for feature, src := range map[string]string{"width": "3 [furlong]", "unit": "furlong"} {
		val, err := evalIn(t, prev, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if err := field.SetFeatureValue(prev, feature, val); err != nil {
			t.Fatalf("write %s: %v", feature, err)
		}
	}
	shapes := prev.ShapesOf(field)

	ctx := libraryContextOver(t, adoptUnitSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, field); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	furlong := lookupOne(t, ctx.model.resolver.Index(), "Demo::furlong")
	unit, err := field.GetFeatureValue(ctx, "unit")
	if err != nil {
		t.Fatalf("GetFeatureValue(unit): %v", err)
	}
	if decl := unit.Value.MeasurementRef().Declaration(); decl != furlong {
		t.Errorf("unit names %p, want the furlong declared by the re-analysis %p", decl, furlong)
	}
	width, err := field.GetFeatureValue(ctx, "width")
	if err != nil {
		t.Fatalf("GetFeatureValue(width): %v", err)
	}
	if decl := width.Value.Quantity().Unit.Product.Powers[0].Unit; decl != furlong {
		t.Errorf("width is measured in %p, want the furlong declared by the re-analysis %p", decl, furlong)
	}
	newScope := lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope
	for expr, want := range map[string]string{
		"field.unit":                                            "furlong",
		"field.unit istype LengthUnit":                          "true",
		"field.unit == furlong":                                 "true",
		"field.width.mRef == field.unit":                        "true",
		"QuantityCalculations::ConvertQuantity(field.width, m)": "603.504 [m]",
	} {
		got, err := evalIn(t, ctx, newScope, expr)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s after the carry-over = %s, %v; want %s", expr, FormatValue(got), err, want)
		}
	}

	gone := libraryContextOver(t, strings.Replace(adoptUnitSrc, "attribute furlong : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 201.168; } }", "", 1))
	var adoptErr *AdoptError
	if _, err := gone.Adopt(prev, shapes, field); !errors.As(err, &adoptErr) {
		t.Fatalf("Adopt into a re-analysis without the unit: %v, want an AdoptError", err)
	} else if !strings.Contains(err.Error(), "the unit furlong it is measured in is no longer declared") {
		t.Errorf("Adopt refused for %q, want the missing unit named", err)
	}
}

// A tensor a run wrote is carried over as a tensor: its shape and magnitudes
// intact, every component's unit rebound to the re-analysis's declaration, and
// the reference object it was built over carried along so `mRef` still answers it.
func TestAdoptRebindsATensorsComponentUnits(t *testing.T) {
	src := strings.Replace(adoptUnitSrc,
		"part def Field { attribute width : LengthValue; attribute unit : LengthUnit; }",
		"part def Field { attribute width : LengthValue; attribute unit : LengthUnit; attribute strain : Quantities::TensorQuantityValue; }\n"+
			"attribute strainRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = (furlong, m, furlong, m); :>> isBound = true; attribute label : ScalarValues::String = \"strain\"; }", 1)
	prev := libraryContextOver(t, src)
	scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
	field, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::field"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	val, err := evalIn(t, prev, scope, "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), strainRef)")
	if err != nil {
		t.Fatalf("tensor: %v", err)
	}
	if err := field.SetFeatureValue(prev, "strain", val); err != nil {
		t.Fatalf("write strain: %v", err)
	}
	shapes := prev.ShapesOf(field)

	ctx := libraryContextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, field); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	strain, err := field.GetFeatureValue(ctx, "strain")
	if err != nil {
		t.Fatalf("GetFeatureValue(strain): %v", err)
	}
	if strain.Value.Kind != ValTensorQuantity {
		t.Fatalf("strain carried over as %s, want a tensor quantity", FormatValue(strain.Value))
	}
	furlong := lookupOne(t, ctx.model.resolver.Index(), "Demo::furlong")
	if decl := strain.Value.TensorQuantity().Units[0].Product.Powers[0].Unit; decl != furlong {
		t.Errorf("component 1 is measured in %p, want the furlong declared by the re-analysis %p", decl, furlong)
	}
	newScope := lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope
	for expr, want := range map[string]string{
		"field.strain":                       "Tensor(2, 2)[1.0 [furlong], 2.0 [m], 3.0 [furlong], 4.0 [m]]",
		"field.strain.dimensions":            "[2, 2]",
		"field.strain#(1, 1) == 201.168 [m]": "true",
		"field.strain.mRef":                  "Array(2, 2)[furlong, m, furlong, m]",
		"field.strain.mRef.mRefs":            "[furlong, m, furlong, m]",
		"field.strain.mRef.isBound":          "true",
		"field.strain.isBound":               "true",
		"field.strain.mRef.label":            "\"strain\"",
		"TensorCalculations::scalarTensorMult(2, field.strain)#(2, 1)": "6.0 [furlong]",
	} {
		got, err := evalIn(t, ctx, newScope, expr)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s after the carry-over = %s, %v; want %s", expr, FormatValue(got), err, want)
		}
	}
}

// A value measured in a unit the re-analysis redefined (same name, other
// reduction) is refused rather than carried over with a stale conversion.
func TestAdoptRefusesAUnitWhoseReductionChanged(t *testing.T) {
	src := strings.Replace(adoptUnitSrc,
		"part def Field { attribute width : LengthValue; attribute unit : LengthUnit; }",
		"part def Field { attribute width : LengthValue; attribute unit : LengthUnit; attribute pos : Quantities::VectorQuantityValue; attribute strain : Quantities::TensorQuantityValue; }\n"+
			"attribute strainRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = (furlong, furlong, furlong, furlong); }", 1)
	changed := libraryContextOver(t, strings.Replace(src, "conversionFactor = 201.168", "conversionFactor = 220", 1))
	for feature, value := range map[string]string{
		"width":  "3 [furlong]",
		"unit":   "furlong",
		"pos":    "VectorFunctions::VectorOf((1.0, 2.0)) [furlong]",
		"strain": "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), strainRef)",
	} {
		prev := libraryContextOver(t, src)
		scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
		field, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::field"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		val, err := evalIn(t, prev, scope, value)
		if err != nil {
			t.Fatalf("%s: %v", value, err)
		}
		if err := field.SetFeatureValue(prev, feature, val); err != nil {
			t.Fatalf("write %s: %v", feature, err)
		}
		shapes := prev.ShapesOf(field)

		same := libraryContextOver(t, src+"\npart def Widget;")
		if _, err := same.Adopt(prev, shapes, field); err != nil {
			t.Fatalf("Adopt of %s into a re-analysis with the same furlong: %v", feature, err)
		}
		var adoptErr *AdoptError
		_, err = changed.Adopt(prev, shapes, field)
		if !errors.As(err, &adoptErr) {
			t.Fatalf("Adopt of %s into a re-analysis redefining furlong: %v, want an AdoptError", feature, err)
		}
		if !strings.Contains(err.Error(), "the unit furlong it is measured in now reduces to 220·metre, not 201.168·metre") {
			t.Errorf("Adopt of %s refused for %q, want the changed reduction named", feature, err)
		}
	}
}

// A model's own base unit (no conversion to a library unit) reduces to itself,
// so an unchanged one is carried over with its reduction rebound, not refused.
func TestAdoptRebindsAModelsOwnBaseUnit(t *testing.T) {
	src := `package Demo {
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;
	attribute chain : LengthUnit;
	attribute furlong : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = chain; :>> conversionFactor = 10; } }
	part def Field { attribute width : LengthValue; attribute unit : LengthUnit; attribute len : LengthValue; }
	part field : Field;
}`
	prev := libraryContextOver(t, src)
	scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
	field, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::field"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for feature, value := range map[string]string{"width": "3 [chain]", "unit": "chain", "len": "2 [furlong]"} {
		val, err := evalIn(t, prev, scope, value)
		if err != nil {
			t.Fatalf("%s: %v", value, err)
		}
		if err := field.SetFeatureValue(prev, feature, val); err != nil {
			t.Fatalf("write %s: %v", feature, err)
		}
	}
	shapes := prev.ShapesOf(field)

	ctx := libraryContextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, field); err != nil {
		t.Fatalf("Adopt into a re-analysis with the same units: %v", err)
	}
	chain := lookupOne(t, ctx.model.resolver.Index(), "Demo::chain")
	for _, feature := range []string{"width", "unit", "len"} {
		fv, err := field.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", feature, err)
		}
		for _, unit := range unitsOf(fv.Value) {
			if got := unit.Term.Factors[0].Unit; got != chain {
				t.Errorf("%s reduces over %p, want the chain declared by the re-analysis %p", feature, got, chain)
			}
		}
	}
	newScope := lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope
	for expr, want := range map[string]string{
		"field.unit == chain":                                         "true",
		"field.width == 3 [chain]":                                    "true",
		"field.width.mRef == field.unit":                              "true",
		"field.len == 20 [chain]":                                     "true",
		"QuantityCalculations::ConvertQuantity(field.len, chain)":     "20.0 [chain]",
		"QuantityCalculations::ConvertQuantity(field.width, furlong)": "0.3 [furlong]",
	} {
		got, err := evalIn(t, ctx, newScope, expr)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s after the carry-over = %s, %v; want %s", expr, FormatValue(got), err, want)
		}
	}

	gone := libraryContextOver(t, strings.Replace(src, "attribute chain : LengthUnit;", "attribute chain : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 20.1168; } }", 1))
	var adoptErr *AdoptError
	if _, err := gone.Adopt(prev, shapes, field); !errors.As(err, &adoptErr) {
		t.Fatalf("Adopt into a re-analysis converting chain to metres: %v, want an AdoptError", err)
	} else if !strings.Contains(err.Error(), "·metre, not ") || !strings.HasSuffix(err.Error(), "chain") {
		t.Errorf("Adopt refused for %q, want the changed reduction named", err)
	}
}

// An empty quantity collection remembers the unit its elements would measure in
// (the zero its sum yields), so that unit is rebound and judged like a value's:
// carried over with the same declaration, refused when it is gone or redefined.
func TestAdoptRebindsTheUnitOfAnEmptyQuantitySequence(t *testing.T) {
	src := strings.Replace(adoptUnitSrc,
		"part def Field { attribute width : LengthValue; attribute unit : LengthUnit; }",
		"part def Field { attribute widths : LengthValue[*]; }", 1)
	prev := libraryContextOver(t, src)
	scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
	field, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::field"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	furlong, err := evalIn(t, prev, scope, "furlong")
	if err != nil {
		t.Fatalf("furlong: %v", err)
	}
	if err := field.SetFeatureValue(prev, "widths", NewEmptySequenceOf(furlong.MeasurementRef().Unit)); err != nil {
		t.Fatalf("write widths: %v", err)
	}
	shapes := prev.ShapesOf(field)

	ctx := libraryContextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, field); err != nil {
		t.Fatalf("Adopt into a re-analysis with the same furlong: %v", err)
	}
	widths, err := field.GetFeatureValue(ctx, "widths")
	if err != nil {
		t.Fatalf("GetFeatureValue(widths): %v", err)
	}
	unit, ok := widths.Values.Sequence().ElementUnit()
	if !ok {
		t.Fatalf("widths after the carry-over = %s, want an empty sequence measured in furlong", FormatValue(widths.Values))
	}
	if decl, want := unit.Product.Powers[0].Unit, lookupOne(t, ctx.model.resolver.Index(), "Demo::furlong"); decl != want {
		t.Errorf("widths is measured in %p, want the furlong declared by the re-analysis %p", decl, want)
	}
	if got, want := unit.Term.Factors[0].Unit, lookupOne(t, ctx.model.resolver.Index(), "SI::m"); got != want {
		t.Errorf("widths reduces over %p, want the metre the re-analysis resolves %p", got, want)
	}

	for name, tc := range map[string]struct{ src, reason string }{
		"gone": {strings.Replace(src, "attribute furlong : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 201.168; } }", "", 1),
			"the unit furlong it is measured in is no longer declared"},
		"redefined": {strings.Replace(src, "conversionFactor = 201.168", "conversionFactor = 220", 1),
			"the unit furlong it is measured in now reduces to 220·metre, not 201.168·metre"},
	} {
		var adoptErr *AdoptError
		_, err := libraryContextOver(t, tc.src).Adopt(prev, shapes, field)
		if !errors.As(err, &adoptErr) {
			t.Fatalf("Adopt into a re-analysis with furlong %s: %v, want an AdoptError", name, err)
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("Adopt into a re-analysis with furlong %s refused for %q, want %q", name, err, tc.reason)
		}
	}
}

const adoptWrittenSrc = `package Demo {
	private import ScalarValues::*;
	part def Holder { attribute n : Integer; }
	part holder : Holder;
}`

// documentContextOver indexes src over the standard library and registers the
// scope tree the document builds for itself, which a workspace resolves references in.
func documentContextOver(t *testing.T, src string) (*Context, *symbols.Scope) {
	t.Helper()
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	scope := symbols.Build(file)
	symbols.SetDocName(scope, "<test>")
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
	ctx.Model().RegisterSource(source.New("<test>", []byte(src)))
	ctx.Model().RegisterScope(scope)
	return ctx, scope
}

// A workspace resolves references in the scope tree its document builds, not in
// the index's, so an object carried into such a context is rebound to that tree's
// symbols: a feature chain from the document reads the carried object and the
// value written to it rather than materializing another.
func TestAdoptRebindsIntoTheScopeTreeTheCallerResolvesIn(t *testing.T) {
	prev, prevScope := documentContextOver(t, adoptWrittenSrc)
	prevDemo := resolveSymbol(t, prevScope, "Demo").Scope
	holder, err := prev.Instantiate(resolveSymbol(t, prevDemo, "holder"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := holder.SetFeatureValue(prev, "n", constInt(5)); err != nil {
		t.Fatalf("write n: %v", err)
	}
	if got, err := evalIn(t, prev, prevDemo, "holder.n"); err != nil || FormatValue(got) != "5" {
		t.Fatalf("holder.n before the carry-over = %s, %v; want 5", FormatValue(got), err)
	}
	shapes := prev.ShapesOf(holder)

	ctx, scope := documentContextOver(t, adoptWrittenSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, holder); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	demo := resolveSymbol(t, scope, "Demo").Scope
	if holder.Type != resolveSymbol(t, demo, "holder") {
		t.Error("the object is of the index's symbol, not the one the document declares")
	}
	got, err := evalIn(t, ctx, demo, "holder.n")
	if err != nil || FormatValue(got) != "5" {
		t.Errorf("holder.n after the carry-over = %s, %v; want the written 5", FormatValue(got), err)
	}
	if n := len(ctx.instances); n != 1 {
		t.Errorf("the context holds %d objects, want the carried one alone", n)
	}
}

const adoptDependentSrc = `package Demo {
	part def Source { attribute x default 3; }
	part def Reader { attribute twice = src.x * 2; }
	part src : Source;
	part reader : Reader;
}`

// A value derived from another object's feature is listed as that feature's
// dependent, an edge of the analysis both were read in: carrying either object over
// drops it, and the value derived again there follows a write here.
func TestAdoptDropsDependencyEdgesAndDerivesAgain(t *testing.T) {
	prev := contextOver(t, adoptDependentSrc)
	src := instantiateNamed(t, prev, prev.Resolver().Index(), "Demo::src")
	reader := instantiateNamed(t, prev, prev.Resolver().Index(), "Demo::reader")
	if got := readInt(t, prev, reader, "twice"); got != 6 {
		t.Fatalf("reader.twice = %d before the carry-over, want 6", got)
	}
	x := src.FeatureValues["x"]
	if len(x.dependents) != 1 {
		t.Fatalf("src.x lists %d dependents before the carry-over, want reader.twice", len(x.dependents))
	}
	srcShapes, readerShapes := prev.ShapesOf(src), prev.ShapesOf(reader)

	ctx := contextOver(t, adoptDependentSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, readerShapes, reader); err != nil {
		t.Fatalf("Adopt(reader): %v", err)
	}
	if len(x.dependents) != 0 {
		t.Errorf("src.x, left behind, still lists %d dependents carried over", len(x.dependents))
	}
	if _, err := ctx.Adopt(prev, srcShapes, src); err != nil {
		t.Fatalf("Adopt(src): %v", err)
	}
	if len(x.dependents) != 0 {
		t.Errorf("src.x still lists %d dependents of the previous analysis", len(x.dependents))
	}
	if twice := reader.FeatureValues["twice"]; twice.Materialized {
		t.Errorf("reader.twice still holds %s from the previous analysis", FormatValue(twice.HeldValue()))
	}
	if got := readInt(t, ctx, reader, "twice"); got != 6 {
		t.Fatalf("reader.twice = %d after the carry-over, want 6", got)
	}
	if err := src.SetFeatureValue(ctx, "x", constInt(9)); err != nil {
		t.Fatalf("SetFeatureValue(x): %v", err)
	}
	if got := readInt(t, ctx, reader, "twice"); got != 18 {
		t.Errorf("reader.twice = %d after x := 9, want 18 from the edge recorded here", got)
	}
}

// Carrying one object over leaves what depended on it where it was: a write to
// the carried object reaches nothing in the analysis it left.
func TestAdoptOfASourceLeavesItsDependentsBehind(t *testing.T) {
	prev := contextOver(t, adoptDependentSrc)
	src := instantiateNamed(t, prev, prev.Resolver().Index(), "Demo::src")
	reader := instantiateNamed(t, prev, prev.Resolver().Index(), "Demo::reader")
	if got := readInt(t, prev, reader, "twice"); got != 6 {
		t.Fatalf("reader.twice = %d before the carry-over, want 6", got)
	}
	shapes := prev.ShapesOf(src)

	ctx := contextOver(t, adoptDependentSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, src); err != nil {
		t.Fatalf("Adopt(src): %v", err)
	}
	if err := src.SetFeatureValue(ctx, "x", constInt(9)); err != nil {
		t.Fatalf("SetFeatureValue(x): %v", err)
	}
	if twice := reader.FeatureValues["twice"]; !twice.Materialized || FormatValue(twice.HeldValue()) != "6" {
		t.Errorf("a write in the new context reached reader.twice left behind (materialized %t, %s)", twice.Materialized, FormatValue(twice.HeldValue()))
	}
	if _, found := ctx.Instance(reader.ID); found {
		t.Error("the reader was carried over with the source it read")
	}
}

const adoptFrameSrc = `package Demo {
	private import ISQ::*;
	private import ISQSpaceTime::*;
	private import SI::*;
	private import MeasurementReferences::*;
	attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
	attribute placed : CartesianSpatial3dCoordinateFrame {
		:>> mRefs = (mm, mm, mm);
		:>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (10.0, 0.0, 0.0) [datum]; }
	}
	part def Body { attribute cf : CoordinateFrame; attribute velocity : CoordinateFrame; attribute pos : Quantities::VectorQuantityValue; attribute placement : CoordinateTransformation; }
	part body : Body;
}`

// A frame a run wrote, one composed from it, a vector over it and its placement
// all name declarations; each is rebound so the frame keeps its identity and its
// axes their units, and a re-analysis without the frame refuses the carry-over.
func TestAdoptRebindsTheFramesAWrittenValueNames(t *testing.T) {
	prev := libraryContextOver(t, adoptFrameSrc)
	scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
	body, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::body"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for feature, src := range map[string]string{
		"cf": "datum", "velocity": "datum / s", "pos": "(1.0, 2.0, 3.0) [datum]", "placement": "placed.transformation",
	} {
		val, err := evalIn(t, prev, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if err := body.SetFeatureValue(prev, feature, val); err != nil {
			t.Fatalf("write %s: %v", feature, err)
		}
	}
	shapes := prev.ShapesOf(body)

	ctx := libraryContextOver(t, adoptFrameSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, body); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	datum := lookupOne(t, ctx.model.resolver.Index(), "Demo::datum")
	frame, err := body.GetFeatureValue(ctx, "cf")
	if err != nil {
		t.Fatalf("GetFeatureValue(cf): %v", err)
	}
	if decl := frame.Value.CoordinateFrame().Decl; decl != datum {
		t.Errorf("frame names %p, want the datum declared by the re-analysis %p", decl, datum)
	}
	pos, err := body.GetFeatureValue(ctx, "pos")
	if err != nil {
		t.Fatalf("GetFeatureValue(pos): %v", err)
	}
	if decl := pos.Value.VectorQuantity().Frame.Decl; decl != datum {
		t.Errorf("pos is over %p, want the datum declared by the re-analysis %p", decl, datum)
	}
	newScope := lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope
	for expr, want := range map[string]string{
		"body.cf":          "datum [mm, mm, mm]",
		"body.cf == datum": "true",
		"body.cf istype CartesianSpatial3dCoordinateFrame": "true",
		"body.velocity":                                           "datum / s [mm/s, mm/s, mm/s]",
		"body.velocity == datum / s":                              "true",
		"body.pos.mRef == body.cf":                                "true",
		"body.placement == placed.transformation":                 "true",
		"VectorCalculations::transform(body.placement, body.pos)": "⟨-9.0, 2.0, 3.0⟩ [placed]",
	} {
		got, err := evalIn(t, ctx, newScope, expr)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s after the carry-over = %s, %v; want %s", expr, FormatValue(got), err, want)
		}
	}

	gone := libraryContextOver(t, strings.Replace(adoptFrameSrc, "attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }", "", 1))
	var adoptErr *AdoptError
	if _, err := gone.Adopt(prev, shapes, body); !errors.As(err, &adoptErr) {
		t.Fatalf("Adopt into a re-analysis without the frame: %v, want an AdoptError", err)
	} else if !strings.Contains(err.Error(), "the coordinate frame datum it holds is no longer declared") {
		t.Errorf("Adopt refused for %q, want the missing frame named", err)
	}
}

const adoptFunctionSrc = `package Demo {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	part def Scaler {
		attribute k : Real = 2.0;
		calc scale { in x : Real; return : Real = x * k; }
	}
	part def Holder { attribute fn; attribute scaled; part scaler : Scaler; }
	part holder : Holder;
}`

// A function value a run wrote is carried over as a function of the calc the
// re-analysis declares, so applying it runs the calc as it is declared now; one
// closing over an object keeps that object with it.
func TestAdoptRebindsAFunctionValue(t *testing.T) {
	prev := libraryContextOver(t, adoptFunctionSrc)
	scope := lookupOne(t, prev.Resolver().Index(), "Demo").Scope
	holder, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::holder"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for feature, expr := range map[string]string{"fn": "Sq", "scaled": "holder.scaler.scale"} {
		val, err := evalIn(t, prev, scope, expr)
		if err != nil || val.Kind != ValFunction {
			t.Fatalf("%s = %s, %v; want a function", expr, FormatValue(val), err)
		}
		if err := holder.SetFeatureValue(prev, feature, val); err != nil {
			t.Fatalf("write %s: %v", feature, err)
		}
	}
	shapes := prev.ShapesOf(holder)

	ctx := libraryContextOver(t, strings.Replace(adoptFunctionSrc, "v * v", "v * v * v", 1))
	if _, err := ctx.Adopt(prev, shapes, holder); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	fn, err := holder.GetFeatureValue(ctx, "fn")
	if err != nil {
		t.Fatalf("GetFeatureValue(fn): %v", err)
	}
	if want := lookupOne(t, ctx.model.resolver.Index(), "Demo::Sq"); fn.Value.Function() != want {
		t.Errorf("fn is of %p, want the Sq declared by the re-analysis %p", fn.Value.Function(), want)
	}
	newScope := lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope
	for expr, want := range map[string]string{
		"holder.fn":                            "Demo::Sq",
		"holder.fn == Sq":                      "true",
		"Fn(holder.fn, 2.0)":                   "8.0",
		"Fn(holder.scaled, 5.0)":               "10.0",
		"holder.scaled == holder.scaler.scale": "true",
	} {
		got, err := evalIn(t, ctx, newScope, expr)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s after the carry-over = %s, %v; want %s", expr, FormatValue(got), err, want)
		}
	}

	gone := libraryContextOver(t, strings.Replace(adoptFunctionSrc, "calc def Sq { in v : Real; return : Real = v * v; }", "", 1))
	var adoptErr *AdoptError
	if _, err := gone.Adopt(prev, shapes, holder); !errors.As(err, &adoptErr) {
		t.Fatalf("Adopt into a re-analysis without the calc: %v, want an AdoptError", err)
	} else if !strings.Contains(err.Error(), "the function Demo::Sq it holds is no longer declared") {
		t.Errorf("Adopt refused for %q, want the missing calc named", err)
	}
}
