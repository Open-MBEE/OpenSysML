package runtime

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// lampBulb materializes a Bulb over lampSource and answers the document root with it.
func lampBulb(t *testing.T) (*symbols.Scope, *Context, *Instance) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
	root := idx.DocumentRoot("lamp.sysml")
	bulb, err := ctx.Instantiate(resolveSymbol(t, root, "Bulb"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return root, ctx, bulb
}

// dispatchTo posts a signal to the bulb and lets its machine dispatch it.
func dispatchTo(t *testing.T, root *symbols.Scope, ctx *Context, bulb *Instance, signal string, args map[string]Value) {
	t.Helper()
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, signal), args, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(%s): %v", signal, err)
	}
	ctx.PostMessage(msg)
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatalf("object #%d exhibits no machine", bulb.ID)
	}
	if err := behavior.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(%s): %v", signal, err)
	}
}

// lampLeaf is the active leaf of the bulb's machine.
func lampLeaf(t *testing.T, bulb *Instance) string {
	t.Helper()
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatalf("object #%d exhibits no machine", bulb.ID)
	}
	return activeLeaf(behavior.State)
}

// lampBrightness is the brightness the bulb's machine holds.
func lampBrightness(t *testing.T, bulb *Instance) string {
	t.Helper()
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatalf("object #%d exhibits no machine", bulb.ID)
	}
	return FormatValue(behavior.State.StateData()["brightness"])
}

// imageInto takes an image of the objects and materializes it into a fresh
// context over the same model, answering that context.
func imageInto(t *testing.T, ctx *Context, objects ...*Instance) *Context {
	t.Helper()
	img, err := ctx.Image(objects...)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	dst := NewContext(ctx.Model(), 10000)
	if err := img.Materialize(dst); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	return dst
}

// A state machine that fired a transition is imaged as it stands: the copy is the
// same object under the same identity in the other context, in the same state,
// and goes on from there as the original does — while neither sees the other's moves.
func TestHeldImageCarriesAMovedStateMachine(t *testing.T) {
	root, src, bulb := lampBulb(t)
	dispatchTo(t, root, src, bulb, "go", nil)
	if got := lampLeaf(t, bulb); got != "on" {
		t.Fatalf("state after go = %s, want on", got)
	}
	if err := src.Pristine(bulb); err == nil {
		t.Fatal("a bulb whose machine fired reads as pristine")
	}

	dst := imageInto(t, src, bulb)
	copied, held := dst.Instance(bulb.ID)
	if !held || copied == bulb || copied.Type != bulb.Type {
		t.Fatalf("Instance(#%d) in the destination = %v, %v; want an object of its own of the same type", bulb.ID, copied, held)
	}
	if got := lampLeaf(t, copied); got != "on" {
		t.Errorf("the copy is at %s, want on as imaged", got)
	}
	var hse *HeldStateError
	if err := dst.Pristine(copied); !errors.As(err, &hse) || !strings.Contains(err.Error(), "has moved") {
		t.Errorf("Pristine(copy) = %v, want the moved machine refused as in the source", err)
	}
	if next := dst.ids.next; next <= bulb.ID {
		t.Errorf("the destination's next identity is %d, want past the imaged #%d", next, bulb.ID)
	}

	level := map[string]Value{"level": integerValue(7)}
	dispatchTo(t, root, src, bulb, "Dim", level)
	dispatchTo(t, root, dst, copied, "Dim", level)
	for name, obj := range map[string]*Instance{"source": bulb, "copy": copied} {
		if got := lampLeaf(t, obj); got != "dimmed" {
			t.Errorf("%s at %s after Dim, want dimmed", name, got)
		}
		if got := lampBrightness(t, obj); got != "7" {
			t.Errorf("%s brightness = %s after Dim 7, want 7", name, got)
		}
	}

	// Only the copy's clock advances: the source's machine stays dimmed.
	if _, err := dst.Advance(5); err != nil {
		t.Fatalf("Advance(copy): %v", err)
	}
	if got := lampLeaf(t, copied); got != "off" {
		t.Errorf("the copy is at %s after 5 s, want off", got)
	}
	if got := lampLeaf(t, bulb); got != "dimmed" {
		t.Errorf("the source moved to %s with the copy's clock, want dimmed", got)
	}
	if src.clock.now != 0 {
		t.Errorf("the source's clock reads %v, want 0", src.clock.now)
	}
}

// A fresh object's image is pristine where the object is: the copy's execution is
// as its start left it.
func TestHeldImageOfAFreshObjectIsPristine(t *testing.T) {
	_, src, bulb := lampBulb(t)
	if err := src.Pristine(bulb); err != nil {
		t.Fatalf("Pristine(fresh bulb) = %v, want admitted", err)
	}
	dst := imageInto(t, src, bulb)
	copied, _ := dst.Instance(bulb.ID)
	if err := dst.Pristine(copied); err != nil {
		t.Errorf("Pristine(copy of a fresh bulb) = %v, want admitted", err)
	}
	if got := lampLeaf(t, copied); got != "off" {
		t.Errorf("the copy is at %s, want off", got)
	}
}

const waiterSource = `
	package test {
		part def Waiter {
			attribute woken: Integer = 0;
			perform action await {
				first start;
				action heard accept g : Integer;
				action mark { assign woken := g; }
				done;
				succession first start then heard;
				succession first heard then mark;
				succession first mark then done;
			}
		}
	}
`

// A performed action parked at an accept is imaged with its token where it parked:
// the copy takes the message it awaits in the other context and writes its own object.
// Parking there is where the start left it, so the object stays pristine.
func TestHeldImageCarriesAParkedAction(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, waiterSource)
	pkg := resolveSymbol(t, root, "test")
	src := NewContext(NewModel(model, resolver), 10000)
	waiter, err := src.Instantiate(resolveSymbol(t, pkg.Scope, "Waiter"))
	if err != nil {
		t.Fatalf("Instantiate Waiter: %v", err)
	}
	if err := src.Pristine(waiter); err != nil {
		t.Fatalf("Pristine(waiter parked by its start) = %v, want admitted", err)
	}

	dst := imageInto(t, src, waiter)
	copied, _ := dst.Instance(waiter.ID)
	behavior, ok := copied.Behavior("await")
	if !ok || behavior.Action == nil {
		t.Fatalf("the copy performs no await action, behaviors: %v", copied.Behaviors())
	}
	if behavior.Action.State() != StateWaiting {
		t.Fatalf("the copy's await is %v, want waiting at its accept", behavior.Action.State())
	}
	tokens := behavior.Action.Tokens()
	if len(tokens) != 1 || tokens[0].Wait == nil || tokens[0].Wait.ParamName != "g" {
		t.Fatalf("the copy's tokens = %+v, want one parked at the accept of g", tokens)
	}

	nine := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 9}}
	dst.PostMessage(Message{SignalType: "Integer", Object: copied.ID, Value: &nine})
	if err := behavior.Action.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion(copy): %v", err)
	}
	if behavior.Action.State() != StateCompleted {
		t.Errorf("the copy's await is %v after its message, want completed", behavior.Action.State())
	}
	if got := featureInt(t, dst, copied, "woken"); got != 9 {
		t.Errorf("the copy's woken = %d, want 9", got)
	}
	if got := featureInt(t, src, waiter, "woken"); got != 0 {
		t.Errorf("the source's woken = %d after the copy's message, want 0", got)
	}
	if original, _ := waiter.Behavior("await"); original.Action.State() != StateWaiting {
		t.Errorf("the source's await is %v, want still waiting", original.Action.State())
	}
}

// An image is refused, by its typed reason, where the state cannot be carried: a
// context inside a step, an object of another context, a destination holding the
// identity or past the image's instant, a feature being written, a value of a body, a destroyed root.
func TestHeldImageRefusesWhatItCannotCarry(t *testing.T) {
	root, src, bulb := lampBulb(t)
	src.runDepth++
	if _, err := src.Image(bulb); !errors.Is(err, ErrSnapshotMidRun) {
		t.Errorf("Image inside a step = %v, want ErrSnapshotMidRun", err)
	}
	src.runDepth--

	other := NewContext(src.Model(), 10000)
	if _, err := other.Image(bulb); !errors.Is(err, ErrImageRoot) {
		t.Errorf("Image of another context's object = %v, want ErrImageRoot", err)
	}

	img, err := src.Image(bulb)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	taken := imageInto(t, src, bulb)
	var hie *HeldImageError
	if err := img.Materialize(taken); !errors.Is(err, ErrImageIdentityTaken) || !errors.As(err, &hie) || hie.ID != bulb.ID {
		t.Errorf("Materialize over a held identity = %v, want ErrImageIdentityTaken naming #%d", err, bulb.ID)
	}
	if err := img.Materialize(src); !errors.Is(err, ErrImageIdentityTaken) {
		t.Errorf("Materialize into the source = %v, want ErrImageIdentityTaken", err)
	}
	ahead := NewContext(src.Model(), 10000)
	ahead.clock.now = 5
	if err := img.Materialize(ahead); !errors.Is(err, ErrImageClock) {
		t.Errorf("Materialize into a context at t=5 = %v, want ErrImageClock", err)
	}
	if n := len(ahead.instances); n != 0 {
		t.Errorf("the refused materialization left %d objects behind", n)
	}

	// A feature being written is state bound to the context writing it.
	src.instances[bulb.ID].FeatureValues["lamp"].changing = true
	if _, err := src.Image(bulb); !errors.Is(err, ErrImageBound) || !errors.As(err, &hie) || hie.ID != bulb.ID {
		t.Errorf("Image over a feature being written = %v, want ErrImageBound naming #%d", err, bulb.ID)
	}
	src.instances[bulb.ID].FeatureValues["lamp"].changing = false

	// A value closed over the run that made it names what no other context holds.
	inBody := Value{Kind: ValFunction, ref: &functionValue{
		shape:     &calcShape{Sym: &symbols.Symbol{Name: "inBody"}, Name: "inBody"},
		enclosing: []frame{{vars: map[string]Value{"k": integerValue(1)}, run: 1}},
	}}
	src.instances[bulb.ID].FeatureValues["lamp"].Value = inBody
	var notPortable *NotPortableError
	if _, err := src.Image(bulb); !errors.As(err, &notPortable) || !errors.As(err, &hie) || hie.ID != bulb.ID {
		t.Errorf("Image over a function of a body = %v, want a NotPortableError naming #%d", err, bulb.ID)
	}

	// A destroyed object is no longer one to sweep on: the image refuses it as a root.
	plain, err := src.Instantiate(resolveSymbol(t, root, "Plain"))
	if err != nil {
		t.Fatalf("Instantiate(Plain): %v", err)
	}
	if err := src.destroy(plain); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if _, err := src.Image(plain); !errors.Is(err, ErrOccurrenceDestroyed) || !errors.As(err, &hie) || hie.ID != plain.ID {
		t.Errorf("Image of a destroyed object = %v, want ErrOccurrenceDestroyed naming #%d", err, plain.ID)
	}
}

// A materialization that fails after its objects stand — here on the last message
// carried — leaves the destination as it found it: no object, behavior, identity,
// counter, clock or run of the image stays behind, and the image goes in whole next time.
func TestHeldImageMaterializeFailsWhole(t *testing.T) {
	root, src, bulb := lampBulb(t)
	dispatchTo(t, root, src, bulb, "go", nil)
	if _, err := src.Advance(2); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	img, err := src.Image(bulb)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	if len(img.behaviors) == 0 || img.clock != 2 {
		t.Fatalf("the image carries %d behaviors at t=%v, want a machine at t=2", len(img.behaviors), img.clock)
	}
	sound := img.messages
	inBody := Value{Kind: ValFunction, ref: &functionValue{
		shape:     &calcShape{Sym: &symbols.Symbol{Name: "inBody"}, Name: "inBody"},
		enclosing: []frame{{vars: map[string]Value{"k": integerValue(1)}, run: 1}},
	}}
	img.messages = append(slices.Clone(sound), Message{Object: bulb.ID, SignalType: "go", Payload: map[string]Value{"k": inBody}})

	dst := NewContext(src.Model(), 10000)
	before := destinationStateOf(dst)
	var notPortable *NotPortableError
	if err := img.Materialize(dst); !errors.As(err, &notPortable) {
		t.Fatalf("Materialize with a message it cannot carry = %v, want a NotPortableError", err)
	}
	if after := destinationStateOf(dst); after != before {
		t.Errorf("the failed materialization changed the destination:\n before %+v\n after  %+v", before, after)
	}

	// A destination on a sequence another context took ahead of it holds what it
	// took, not the sequence's high-water mark, once the failed materialization is undone.
	ahead := NewContext(src.Model(), 10000)
	ahead.claimID(99)
	shared := NewContext(src.Model(), 10000)
	shared.AdoptIdentities(ahead)
	if shared.ids.next != 100 || shared.took.high != 1 {
		t.Fatalf("the shared sequence is at %d with the destination's mark at %d, want 100 and 1", shared.ids.next, shared.took.high)
	}
	before = destinationStateOf(shared)
	if err := img.Materialize(shared); !errors.As(err, &notPortable) {
		t.Fatalf("Materialize into the shared sequence = %v, want a NotPortableError", err)
	}
	if after := destinationStateOf(shared); after != before {
		t.Errorf("the failed materialization changed the destination on a shared sequence:\n before %+v\n after  %+v", before, after)
	}

	img.messages = sound
	if err := img.Materialize(dst); err != nil {
		t.Fatalf("Materialize after the failure: %v", err)
	}
	copied, ok := dst.Instance(bulb.ID)
	if !ok {
		t.Fatalf("the destination holds no #%d", bulb.ID)
	}
	if got := lampLeaf(t, copied); got != "on" {
		t.Errorf("the copy's state = %s, want on", got)
	}
	if dst.clock.now != 2 {
		t.Errorf("the destination's clock = %v, want 2", dst.clock.now)
	}
}

// The identities a carry-over set aside for connectors not materialized again are the
// image's too: the destination hands none of them out, so a connector asked for after
// the materialization takes its identity back; a destination already holding an object
// under one refuses the image whole.
func TestHeldImageKeepsTheIdentitiesSetAsideForConnectors(t *testing.T) {
	src := strings.Replace(adoptConnectSrc, "connect a.p to b.q;", "connect a.p to b.q; connection c connect a.p to b.q;", 1)
	prev := contextOver(t, src)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := obj.OwnedConnectors(prev); err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	fvInstance(t, prev, obj, "c")
	shapes := prev.ShapesOf(obj)
	ctx := contextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	kept := obj.KeptConnectorIDs()
	if len(kept) != 2 {
		t.Fatalf("KeptConnectorIDs() = %v after the carry-over, want the two connectors", kept)
	}
	img, err := ctx.Image(obj)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	widget := lookupOne(t, ctx.Resolver().Index(), "Widget")

	dst := NewContext(ctx.Model(), 10000)
	if err := img.Materialize(dst); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	copied, ok := dst.Instance(obj.ID)
	if !ok {
		t.Fatalf("the destination holds no #%d", obj.ID)
	}
	if got := copied.KeptConnectorIDs(); !slices.Equal(got, kept) {
		t.Fatalf("the copy's KeptConnectorIDs() = %v, want %v", got, kept)
	}
	for range kept {
		made, err := dst.Instantiate(widget)
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		if slices.Contains(kept, made.ID) {
			t.Errorf("the destination handed out #%d, an identity set aside for a connector", made.ID)
		}
	}
	for _, id := range kept {
		conn, err := copied.RestoreConnector(dst, id)
		if err != nil {
			t.Fatalf("RestoreConnector(%d): %v", id, err)
		}
		if conn == nil || conn.ID != id || len(conn.Ends) != 2 {
			t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", id, conn)
		}
		port := fvInstance(t, dst, copied, "a", "p")
		if end := conn.Ends[0].Value; !holdsObject(end, port.ID) {
			t.Errorf("connector %d's end holds %v, want the port object %d of the copy", id, end, port.ID)
		}
	}

	taken := NewContext(ctx.Model(), 10000)
	taken.claimID(kept[0] - 1)
	holder, err := taken.Instantiate(widget)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if holder.ID != kept[0] {
		t.Fatalf("the holder is #%d, want #%d", holder.ID, kept[0])
	}
	before := destinationStateOf(taken)
	err = img.Materialize(taken)
	var imageErr *HeldImageError
	if !errors.Is(err, ErrImageIdentityTaken) || !errors.As(err, &imageErr) || imageErr.ID != obj.ID {
		t.Fatalf("Materialize into a context holding #%d = %v, want ErrImageIdentityTaken about #%d", kept[0], err, obj.ID)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("connector #%d set aside", kept[0])) {
		t.Errorf("Materialize = %v, want it to name connector #%d", err, kept[0])
	}
	if after := destinationStateOf(taken); after != before {
		t.Errorf("the refused materialization changed the destination:\n before %+v\n after  %+v", before, after)
	}
}

// An identity the destination set aside for a connector is held though no object is
// under it yet: an image whose object has that identity is refused, so the connector
// takes its identity back when asked for.
func TestHeldImageRefusesAnIdentitySetAsideByTheDestination(t *testing.T) {
	src := strings.Replace(adoptConnectSrc, "connect a.p to b.q;", "connect a.p to b.q; connection c connect a.p to b.q;", 1)
	prev := contextOver(t, src)
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := obj.OwnedConnectors(prev); err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	fvInstance(t, prev, obj, "c")
	dst := contextOver(t, src+"\npart def Widget;")
	if _, err := dst.Adopt(prev, prev.ShapesOf(obj), obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	kept := obj.KeptConnectorIDs()
	if len(kept) != 2 {
		t.Fatalf("KeptConnectorIDs() = %v after the carry-over, want the two connectors", kept)
	}
	if _, held := dst.Instance(kept[0]); held {
		t.Fatalf("the destination holds an object under #%d, want the identity set aside only", kept[0])
	}

	other := NewContext(dst.Model(), 10000)
	other.claimID(kept[0] - 1)
	widget, err := other.Instantiate(lookupOne(t, dst.Resolver().Index(), "Widget"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if widget.ID != kept[0] {
		t.Fatalf("the widget is #%d, want #%d", widget.ID, kept[0])
	}
	img, err := other.Image(widget)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	before := destinationStateOf(dst)
	err = img.Materialize(dst)
	var imageErr *HeldImageError
	if !errors.Is(err, ErrImageIdentityTaken) || !errors.As(err, &imageErr) || imageErr.ID != widget.ID {
		t.Fatalf("Materialize over an identity set aside = %v, want ErrImageIdentityTaken about #%d", err, widget.ID)
	}
	if after := destinationStateOf(dst); after != before {
		t.Errorf("the refused materialization changed the destination:\n before %+v\n after  %+v", before, after)
	}
	conn, err := obj.RestoreConnector(dst, kept[0])
	if err != nil {
		t.Fatalf("RestoreConnector(%d): %v", kept[0], err)
	}
	if conn == nil || conn.ID != kept[0] {
		t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", kept[0], conn)
	}
}

// A usage the image has denote an object of its own is refused where the destination
// already has it denote another live object; a destination not yet holding one has the
// usage denote the copy.
func TestHeldImageRefusesAUsageDenotingAnotherObjectOfTheDestination(t *testing.T) {
	root, src, _ := lampBulb(t)
	usage := resolveSymbol(t, root, "plain")
	plain, err := src.Instantiate(usage)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := src.OccurrenceUsage(plain); got != "plain" {
		t.Fatalf("OccurrenceUsage(#%d) = %q, want plain", plain.ID, got)
	}
	img, err := src.Image(plain)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}

	dst := NewContext(src.Model(), 10000)
	dst.claimID(100)
	own, err := dst.Instantiate(usage)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	before := destinationStateOf(dst)
	err = img.Materialize(dst)
	var imageErr *HeldImageError
	if !errors.Is(err, ErrImageBindingTaken) || !errors.As(err, &imageErr) || imageErr.ID != plain.ID {
		t.Fatalf("Materialize where plain denotes #%d = %v, want ErrImageBindingTaken about #%d", own.ID, err, plain.ID)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("usage plain denotes #%d", own.ID)) {
		t.Errorf("Materialize = %v, want it to name the usage and #%d", err, own.ID)
	}
	if after := destinationStateOf(dst); after != before {
		t.Errorf("the refused materialization changed the destination:\n before %+v\n after  %+v", before, after)
	}
	if got := dst.OccurrenceUsage(own); got != "plain" {
		t.Errorf("OccurrenceUsage(#%d) = %q after the refusal, want plain", own.ID, got)
	}

	free := NewContext(src.Model(), 10000)
	if err := img.Materialize(free); err != nil {
		t.Fatalf("Materialize into a context not holding plain: %v", err)
	}
	copied, ok := free.Instance(plain.ID)
	if !ok {
		t.Fatalf("the destination holds no #%d", plain.ID)
	}
	if got := free.OccurrenceUsage(copied); got != "plain" {
		t.Errorf("OccurrenceUsage(copy) = %q, want plain", got)
	}
}

// destinationState is every part of a context a materialization writes, as one value to compare.
type destinationState struct {
	instances, created, lives, behaviors, messages, occurrences, metadata, variants, selected int
	onClock                                                                                   int
	nextID, tookHigh, activations, runs                                                       int64
	clock                                                                                     float64
	clockRun                                                                                  *runState
}

func destinationStateOf(ctx *Context) destinationState {
	return destinationState{
		instances: len(ctx.instances), created: len(ctx.created), lives: len(ctx.lives), behaviors: len(ctx.objectBehaviors),
		messages: len(ctx.messages), occurrences: len(ctx.occurrences), metadata: len(ctx.metadataObjects),
		variants: len(ctx.variantObjects), selected: len(ctx.selectedVariants),
		onClock: len(ctx.clock.waiters),
		nextID:  ctx.ids.next, tookHigh: ctx.took.high, activations: ctx.activations, runs: ctx.runs,
		clock: ctx.clock.now, clockRun: ctx.clockRun.state,
	}
}

// An object whose start left a do action's body paused at its accept is refused
// as the in-place snapshot refuses it: the body's statement is mid-run.
func TestHeldImageRefusesAPausedBody(t *testing.T) {
	const source = `
	private import SI::*;
	state def Watching {
		entry; then watching;
		state watching {
			do action poll {
				first start;
				then action wait accept after 3 [s];
				then done;
			}
		}
	}
	part def Watcher { exhibit state w : Watching; }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "watcher.sysml", parseAndBuild(t, source))
	watcher, err := ctx.Instantiate(resolveSymbol(t, idx.DocumentRoot("watcher.sysml"), "Watcher"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := ctx.Pristine(watcher); err != nil {
		t.Fatalf("Pristine = %v, want the watcher as its start left it", err)
	}
	var hie *HeldImageError
	_, err = ctx.Image(watcher)
	if !errors.Is(err, ErrSnapshotPausedBody) || !errors.As(err, &hie) || hie.ID != watcher.ID {
		t.Fatalf("Image = %v, want ErrSnapshotPausedBody naming #%d", err, watcher.ID)
	}
	if _, err := ctx.Snapshot(); !errors.Is(err, ErrSnapshotPausedBody) {
		t.Errorf("Snapshot = %v, want the same ErrSnapshotPausedBody", err)
	}
}

// heldDigest renders the objects held under ids, their lifetimes, features,
// behaviors and executions, so a copy can be compared with what it was taken from.
func heldDigest(ctx *Context, held func(id int64) bool) string {
	var b strings.Builder
	for _, id := range ctx.created {
		inst := ctx.instances[id]
		if inst == nil || !held(id) {
			continue
		}
		owner := int64(0)
		if inst.owner != nil {
			owner = inst.owner.ID
		}
		fmt.Fprintf(&b, "#%d %s owner=#%d.%s life=%+v ends=%d\n", id, symbolText(inst.Type), owner, inst.ownerFeature, ctx.lives[id], len(inst.Ends))
		for _, name := range slices.Sorted(maps.Keys(inst.FeatureValues)) {
			fv := inst.FeatureValues[name]
			fmt.Fprintf(&b, "  %s = %s written=%t materialized=%t\n", name, FormatValue(fv.HeldValue()), fv.Written, fv.Materialized)
		}
		for _, behavior := range inst.behaviors {
			fmt.Fprintf(&b, "  behavior %s %s moved=%t\n", behavior.Kind, behavior.Name, behavior.Moved())
			if exec := behavior.Action; exec != nil {
				fmt.Fprintf(&b, "    action state=%v inputs=%s\n", exec.State(), formatValues(exec.inputs))
				for _, token := range exec.Tokens() {
					fmt.Fprintf(&b, "    token %d at %s wait=%+v\n", token.ID, ActionNodeName(token.Location), token.Wait)
				}
			}
			if exec := behavior.State; exec != nil {
				var active []string
				for _, state := range exec.ActiveStates() {
					active = append(active, getNodeName(state))
				}
				fmt.Fprintf(&b, "    state=%v active=%v queue=%d deferred=%d data=%s\n",
					exec.State(), active, exec.eventQueue.Len(), len(exec.deferred), formatValues(exec.StateData()))
			}
		}
	}
	fmt.Fprintf(&b, "clock=%v\n", ctx.clock.now)
	for _, msg := range ctx.messages {
		if msg.Object == 0 || held(msg.Object) {
			fmt.Fprintf(&b, "message %s -> %s #%d\n", msg.SignalType, msg.Target, msg.Object)
		}
	}
	return b.String()
}

// TestHeldImageRoundTrip images the object graph of every instance conformance case
// as its materialization left it, materializes the image into another context, and
// runs the case's object machines in both: the same graph, trace and error either
// side, the source untouched by the copy's run. No instance case is refused; the
// refusals are pinned by TestHeldImageRefusesWhatItCannotCarry.
func TestHeldImageRoundTrip(t *testing.T) {
	forEachConformanceCase(t, func(t *testing.T, conformanceDir, testName string, expected ExpectedOutcome) {
		if expected.Type != "instance" {
			t.Skip("not an instance case")
		}
		ctx, idx, _ := loadTraceCase(t, conformanceDir, testName, expected, DefaultSchedulePolicy)
		typeSym := oneSymbol(t, idx, expected.Instantiate)
		first, err := ctx.Instantiate(typeSym)
		if err != nil {
			t.Skipf("materialization fails, so nothing is held: %v", err)
		}
		objects := materializations(t, ctx, typeSym, first, expected.Objects)

		img, err := ctx.Image(objects...)
		if err != nil {
			t.Fatalf("Image as materialized: %v", err)
		}
		dst := NewContext(ctx.Model(), 10000)
		mustSchedule(t, dst, casePolicy(t, expected, DefaultSchedulePolicy))
		if err := img.Materialize(dst); err != nil {
			t.Fatalf("Materialize: %v", err)
		}
		before := heldDigest(ctx, img.Holds)
		if got := heldDigest(dst, everyObject); got != before {
			t.Fatalf("the copy differs from the graph imaged, line %d\n=== SOURCE ===\n%s\n=== COPY ===\n%s", firstDifferingLine(before, got), before, got)
		}
		if dst.ids.next <= ctx.ids.next-1 && len(img.objects) > 0 && dst.ids.next <= img.objects[len(img.objects)-1].id {
			t.Fatalf("the copy's next identity %d is not past the imaged objects", dst.ids.next)
		}

		// The copy's run leaves the source as imaged.
		copyTrace := NewTraceRecorder()
		dst.SetTrace(copyTrace)
		copyErr := runImagedMachines(t, dst, objects, expected)
		if got := heldDigest(ctx, img.Holds); got != before {
			t.Fatalf("the copy's run wrote the source, line %d\n=== BEFORE ===\n%s\n=== AFTER ===\n%s", firstDifferingLine(before, got), before, got)
		}
		sourceTrace := NewTraceRecorder()
		ctx.SetTrace(sourceTrace)
		sourceErr := runImagedMachines(t, ctx, objects, expected)
		want := roundTripOutcome{trace: sourceTrace.String(), values: heldDigest(ctx, everyObject), err: errorText(sourceErr)}
		got := roundTripOutcome{trace: copyTrace.String(), values: heldDigest(dst, everyObject), err: errorText(copyErr)}
		if got != want {
			t.Fatalf("the copy's run differs from the source's\n%s", outcomeDiff(want, got))
		}

		// The graph the run left, moved executions and all, images too.
		ran, err := ctx.Image(objects...)
		if err != nil {
			t.Fatalf("Image after the run: %v", err)
		}
		later := NewContext(ctx.Model(), 10000)
		mustSchedule(t, later, casePolicy(t, expected, DefaultSchedulePolicy))
		if err := ran.Materialize(later); err != nil {
			t.Fatalf("Materialize after the run: %v", err)
		}
		if got := heldDigest(later, everyObject); got != want.values {
			t.Fatalf("the copy of the graph the run left differs, line %d\n=== SOURCE ===\n%s\n=== COPY ===\n%s", firstDifferingLine(want.values, got), want.values, got)
		}
	})
}

// runImagedMachines runs the case's object machines on the objects ctx holds under
// the given objects' identities, as runObjectMachines does on the objects themselves.
func runImagedMachines(t *testing.T, ctx *Context, objects []*Instance, expected ExpectedOutcome) error {
	t.Helper()
	for _, run := range expected.Objects {
		obj, held := ctx.Instance(objects[instanceIndexOf(run)].ID)
		if !held {
			t.Fatalf("the context holds no object #%d", objects[instanceIndexOf(run)].ID)
		}
		if run.Path != "" {
			obj = instanceAtPath(t, ctx, obj, run.Path)
		}
		exec := objectMachine(t, obj, run.Behavior)
		injectEvents(t, exec, run.Events)
		if err := exec.RunToCompletion(); err != nil {
			return err
		}
	}
	return nil
}

// everyObject holds every identity: the digest of a whole context.
func everyObject(int64) bool { return true }

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// One image serves two sweeps at once: each on a goroutine of its own materializes it
// into eight contexts, each over a model of its own on the shared index as the parallel
// sweep's jobs are, and runs the copies concurrently; the source — the digest of its
// objects and the state of its machine — is as it was before.
func TestHeldImageServesConcurrentSweeps(t *testing.T) {
	idx, _, src := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
	root := idx.DocumentRoot("lamp.sysml")
	bulb, err := src.Instantiate(resolveSymbol(t, root, "Bulb"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	dispatchTo(t, root, src, bulb, "go", nil)
	img, err := src.Image(bulb)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	before := heldDigest(src, everyObject)
	dim := resolveSymbol(t, root, "Dim")

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for sweep := 0; sweep < 2; sweep++ {
		for row := 0; row < 8; row++ {
			wg.Add(1)
			go func(level int64) {
				defer wg.Done()
				resolver := resolve.New(idx)
				dst := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
				if err := img.Materialize(dst); err != nil {
					errs <- fmt.Errorf("Materialize: %w", err)
					return
				}
				copied, held := dst.Instance(bulb.ID)
				if !held {
					errs <- fmt.Errorf("the copy holds no #%d", bulb.ID)
					return
				}
				msg, err := dst.SignalMessage(dim, map[string]Value{"level": integerValue(level)}, copied)
				if err != nil {
					errs <- err
					return
				}
				dst.PostMessage(msg)
				behavior, _ := copied.ExhibitedState()
				if err := behavior.State.ProcessNextEvent(); err != nil {
					errs <- err
					return
				}
				if got := FormatValue(behavior.State.StateData()["brightness"]); got != fmt.Sprint(level) {
					errs <- fmt.Errorf("a row dimmed to %d reads brightness %s", level, got)
				}
			}(int64(sweep*8 + row + 1))
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := lampLeaf(t, bulb); got != "on" {
		t.Errorf("the source is at %s after the sweeps, want on", got)
	}
	if after := heldDigest(src, everyObject); after != before {
		t.Errorf("the sweeps changed the source:\n%s\nwas\n%s", after, before)
	}
}
