// Exercised from outside the package: resolving a chain or an inherited member
// needs the semantic model, which imports resolve.
package resolve_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// referencesModel writes the declaration forms whose references an editor has to
// find, from imports and aliases through chains, ends, transitions and states.
const referencesModel = `package Lib {
	attribute def Torque;
	part def Wheel;
	part def Engine {
		attribute rate : Torque;
		action generateTorque;
	}
	item def Fuel;
	port def FuelPort {
		out item fuel : Fuel;
	}
}
package App {
	private import Lib::*;
	alias Motor for Lib::Engine;
	dependency App::Car to Lib::Wheel;
	comment about App::Car /* the car */

	part def Car {
		part engine : Engine;
		part wheel : Wheel[4];
		attribute peak : Torque = engine.rate;
		port intake : FuelPort;
		perform engine.generateTorque;
	}
	part def Racecar :> Car {
		attribute :>> peak = 1;
		part :>> engine : Engine;
	}
	connection def Feed {
		end part driver : Car;
		end part driven : Car;
	}
	part car : Car;
	connection feed : Feed connect car.engine to car.wheel;
	flow of Fuel from car.intake.fuel to car.intake.fuel;
	action def Drive {
		in attribute speed : Torque;
		first start;
		then action step {
			assign speed := speed + 1;
		}
		if speed > 1 { action fast; }
		while speed > 1 { action loop; }
		action prep;
		action launch;
		action left;
		action right;
		first prep then launch { attribute delay : Torque; attribute margin : Torque = delay; }
		then launch { attribute wait : Torque; attribute slack : Torque = wait; }
		then decide;
		if speed > 1 then left;
		else right;
	}
	state def Trip {
		entry action begin;
		state running;
		state stopped;
		transition running_to_stopped first running then stopped;
		transition first running accept f : Fuel then stopped {
			action halt;
			action rest;
			first halt then rest;
			attribute got : Fuel = f;
		}
	}
	constraint def Positive {
		in attribute limit : Torque;
		limit > 0
	}
	requirement def Fast {
		subject vehicle : Car;
		assume constraint { vehicle.peak > 0 }
		require Positive;
	}
}`

// A form the collector misses is a name the editor can neither navigate nor
// rename, so every collected reference must resolve on its own to what it named.
func TestEveryCollectedReferenceResolvesOnItsOwn(t *testing.T) {
	walk, root, rootScope := resolvedDoc(t, referencesModel)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}

	refs := resolve.References(root, rootScope)
	if len(refs) == 0 {
		t.Fatal("References found no name in a document full of them")
	}
	query, _, _ := resolvedDoc(t, referencesModel) // a fresh resolver, as the editor's is
	for _, ref := range refs {
		sym, ok := query.ResolveReference(ref)
		if !ok {
			t.Errorf("%s at offset %d does not resolve on its own, though the document walk resolved it",
				nameText(ref.QN), ref.QN.Span().Offset)
			continue
		}
		if walked, ok := walk.PartSymbol(ref.QN, len(ref.QN.Parts)-1); ok && walked.Name != sym.Name {
			t.Errorf("%s resolves to %q on its own but to %q in the document walk",
				nameText(ref.QN), sym.Name, walked.Name)
		}
	}
}

// The kind of an occurrence decides where it resolves — a redefinition in the
// generals, a chain member in the preceding segment — so it has to be tagged.
func TestCollectedReferencesCarryTheirResolutionKind(t *testing.T) {
	_, root, rootScope := resolvedDoc(t, referencesModel)
	byName := map[string][]resolve.Reference{}
	for _, ref := range resolve.References(root, rootScope) {
		byName[nameText(ref.QN)] = append(byName[nameText(ref.QN)], ref)
	}

	// `attribute :>> peak` redefines an inherited feature; `vehicle.peak` names
	// a member of the subject's type.
	redefines, chained := 0, 0
	for _, ref := range byName["peak"] {
		if ref.Redefines {
			redefines++
		}
		if ref.Chain != nil {
			chained++
		}
	}
	if redefines != 1 || chained != 1 {
		t.Errorf("`peak` yields %d redefinitions and %d chain members, want 1 and 1", redefines, chained)
	}
	if refs := byName["engine"]; len(refs) != 4 {
		// Not the declaration `part engine : Engine`: the redefinition in Racecar,
		// `engine.rate`, `perform engine.…` and `connect car.engine`.
		t.Errorf("`engine` appears %d times, want 4: %v", len(refs), kindsOf(refs))
	}
	// A chain's member segment carries the chain so it is looked up in the
	// operand rather than in the enclosing scope.
	for _, name := range []string{"rate", "generateTorque"} {
		refs := byName[name]
		if len(refs) != 1 || refs[0].Chain == nil {
			t.Errorf("`%s` = %v, want one reference tagged as a chain member", name, kindsOf(refs))
		}
	}
	// `perform engine.generateTorque` declares no name, so the performing usage is
	// the referrer: its borrowed binding must not shadow what the chain names.
	for _, ref := range byName["generateTorque"] {
		if ref.Referrer == nil {
			t.Error("`perform engine.generateTorque` records no referrer, so the target could resolve to the borrowed name")
		}
	}
}

// A filter condition's names are not restricted by the namespace's own filters,
// so the collector marks them and resolution keeps them unfiltered.
func TestAFilterConditionsOwnNamesAreMarked(t *testing.T) {
	const src = `package Meta {
	metadata def Safety;
}
package App {
	private import Meta::*;
	filter @Safety;
	part def Belt;
}`
	r, root, rootScope := resolvedDoc(t, src)
	var conditions, plain int
	for _, ref := range resolve.References(root, rootScope) {
		if ref.Condition {
			conditions++
			if _, ok := r.ResolveReference(ref); !ok {
				t.Errorf("the condition's own name %s must resolve unfiltered", nameText(ref.QN))
			}
			continue
		}
		plain++
	}
	if conditions != 1 {
		t.Errorf("%d references marked as a filter condition's own name, want 1 (@Safety)", conditions)
	}
	if plain == 0 {
		t.Error("the import's own name must be reported as an ordinary reference")
	}
}

// A succession body is a scope of its own, whether written `first a then b { … }`
// or `then b { … }`: a name it declares is found only by a reference collected
// with that scope. The edge's own ends are references like a transition's.
func TestSuccessionBodyReferencesCarryTheBodyScope(t *testing.T) {
	r, root, rootScope := resolvedDoc(t, referencesModel)
	fresh, _, _ := resolvedDoc(t, referencesModel) // resolution is memoized per name node
	byName := map[string][]resolve.Reference{}
	for _, ref := range resolve.References(root, rootScope) {
		byName[nameText(ref.QN)] = append(byName[nameText(ref.QN)], ref)
	}
	for name, form := range map[string]string{"delay": "*ast.InitialNode", "wait": "*ast.SuccessionEdge"} {
		refs := byName[name]
		if len(refs) != 1 {
			t.Errorf("`%s` is collected %d times, want 1: the `= %s` inside the %s body", name, len(refs), name, form)
			continue
		}
		ref := refs[0]
		if got := fmt.Sprintf("%T", ref.Scope.Node()); got != form {
			t.Errorf("`= %s` is collected in the scope of %s, want the body scope of the %s", name, got, form)
		}
		sym, ok := r.ResolveReference(ref)
		if !ok || sym.OwnerScope != ref.Scope {
			t.Errorf("`= %s` resolves to %v, %v; want the attribute the body declares", name, sym, ok)
		}
		if _, ok := fresh.ResolveReference(resolve.Reference{Scope: ref.Scope.Parent(), QN: ref.QN}); ok {
			t.Errorf("`%s` resolves from the action body, so the body scope was not needed", name)
		}
	}
	// `first prep then launch` refers to `prep` as the succession's source, so it
	// is a reference like `launch`, the end of both successions; `left` and `right`
	// are the branches of the decision.
	for name, want := range map[string]int{"prep": 1, "launch": 2, "left": 1, "right": 1} {
		if n := len(byName[name]); n != want {
			t.Errorf("edge end `%s` is collected %d times, want %d", name, n, want)
		}
	}
}

// A send's receiver (`via p to ground`) is a name like its payload and port; a
// constructor's argument label (`new T(frames = …)`) names a feature of T — its
// own or inherited — so it is collected as one and resolves to T's feature, never
// to a same-named feature in the sender's scope.
func TestSendReceiverAndConstructorLabelsAreReferences(t *testing.T) {
	const src = `package App {
	item def Telemetry { attribute frames; }
	item def Burst :> Telemetry { attribute rate; }
	port def Radio;
	part def Station;
	action def Downlink {
		port antenna : Radio;
		part ground : Station;
		attribute frames;
		send new Telemetry(frames = 3) via antenna to ground;
		send new Burst(frames = 4, rate = 2) via antenna to missing;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 1 || !strings.Contains(walk.Diagnostics[0].Message, "missing") {
		t.Fatalf("the unresolved receiver `missing` must be the one diagnostic, got %v", walk.Diagnostics)
	}
	pkg := unwrapMember(root.Members[0])
	telemetry := rootScope.ChildFor(pkg).ChildFor(memberOf(pkg, 0))
	want, ok := telemetry.LookupLocal("frames")
	if !ok || want == nil {
		t.Fatal("Telemetry's `attribute frames` was not indexed")
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "frames" && name != "rate" {
			continue
		}
		if ref.Constructed == nil {
			t.Errorf("label `%s` at %v is not tagged as a constructor argument", name, ref.QN.Span())
		}
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Fatalf("label `%s` at %v did not resolve", name, ref.QN.Span())
		}
		if name == "frames" && sym != want {
			t.Errorf("label `frames` at %v resolved to %s's feature, want Telemetry's", ref.QN.Span(), sym.Name)
		}
	}
	if byName["ground"] != 1 || byName["missing"] != 1 {
		t.Errorf("receivers collected: ground=%d missing=%d, want 1 and 1", byName["ground"], byName["missing"])
	}
	if byName["frames"] != 2 || byName["rate"] != 1 {
		t.Errorf("labels collected: frames=%d rate=%d, want 2 and 1", byName["frames"], byName["rate"])
	}
}

// A label naming no feature of the constructed type is unresolved where it is
// written, though the sender's scope declares that name.
func TestConstructorLabelMustNameAFeatureOfTheConstructedType(t *testing.T) {
	const src = `package App {
	item def Telemetry { attribute frames; }
	part def Station;
	action def Downlink {
		part ground : Station;
		attribute count;
		send new Telemetry(count = 3) to ground;
	}
}`
	walk, _, _ := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 1 || !strings.Contains(walk.Diagnostics[0].Message, "count") {
		t.Fatalf("the label `count` must be the one unresolved name, got %v", walk.Diagnostics)
	}
	if got, want := walk.Diagnostics[0].Span.Offset, strings.LastIndex(src, "count = 3"); got != want {
		t.Errorf("reported at offset %d, want the label at %d", got, want)
	}
}

// A transition's guard and effect are collected in the scope holding the
// parameter its trigger declares, as the document walk resolves them, so a send
// whose receiver is that parameter reaches it rather than a same-named feature
// of the machine — also on a fresh resolver, as rename uses, with nothing memoized
// from a document walk.
func TestTransitionEffectReferencesResolveInTriggerScope(t *testing.T) {
	const src = `package App {
	item def Request;
	state def Server {
		part origin : Request;
		state idle;
		state busy;
		transition first idle accept origin : Request if origin != null do send new Request() to origin then busy;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", walk.Diagnostics)
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	pkg := unwrapMember(root.Members[0])
	part, ok := rootScope.ChildFor(pkg).ChildFor(memberOf(pkg, 1)).LookupLocal("origin")
	if !ok || part == nil {
		t.Fatal("the machine's `part origin` was not indexed")
	}
	uses := 0
	for _, ref := range resolve.References(root, rootScope) {
		if nameText(ref.QN) != "origin" {
			continue
		}
		uses++
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Fatalf("`origin` at %v did not resolve", ref.QN.Span())
		}
		if sym == part {
			t.Errorf("`origin` at %v reached the machine's part, want the accept's parameter", ref.QN.Span())
		}
	}
	if uses != 2 {
		t.Errorf("collected %d `origin` reference(s) (guard and receiver), want 2", uses)
	}
}

// A transition's trailing body (`then busy { … }`) resolves and is collected like
// its effect: in the trigger's scope, so its sends reach the accept's parameter
// and an unresolved receiver or constructor label in it is reported.
func TestTransitionBodyResolvesInTriggerScope(t *testing.T) {
	const src = `package App {
	item def Request { attribute id; }
	state def Server {
		state idle;
		state busy;
		transition t first idle accept origin : Request then busy {
			action log;
			send new Request(id = 1) to origin;
			send new Request(count = 1) to missing;
		}
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 2 {
		t.Fatalf("the label `count` and the receiver `missing` must be the two unresolved names, got %v", walk.Diagnostics)
	}
	for _, d := range walk.Diagnostics {
		if !strings.Contains(d.Message, "count") && !strings.Contains(d.Message, "missing") {
			t.Errorf("unexpected diagnostic %v", d)
		}
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "origin" && name != "id" {
			continue
		}
		if _, ok := cold.ResolveReference(ref); !ok {
			t.Errorf("`%s` at %v did not resolve", name, ref.QN.Span())
		}
	}
	if byName["origin"] != 1 || byName["id"] != 1 || byName["count"] != 1 || byName["missing"] != 1 {
		t.Errorf("body references collected: %v, want origin, id, count and missing once each", byName)
	}
	pkg := unwrapMember(root.Members[0])
	trans, ok := rootScope.ChildFor(pkg).ChildFor(memberOf(pkg, 1)).LookupLocal("t")
	if !ok || trans == nil || trans.Scope == nil {
		t.Fatal("the transition `t` was not indexed with a scope")
	}
	if _, ok := trans.Scope.LookupLocal("log"); !ok {
		t.Error("the body's `action log` is not a member of the transition")
	}
}

// A body expression in a transition's trailing body declares its parameter in a
// scope beneath the trigger's, so both it and the accept's parameter resolve —
// also on a cold resolver, with nothing memoized from a document walk.
func TestTransitionBodyExpressionParametersResolveInTriggerScope(t *testing.T) {
	const src = `package App {
	attribute def Id;
	item def Request { attribute ids : Id[*]; }
	state def Server {
		state idle;
		state busy;
		transition t first idle accept origin : Request then busy {
			attribute positive = origin.ids->forAll { in i : Id; i == origin };
		}
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	for _, d := range walk.Diagnostics {
		if strings.Contains(d.Message, "unresolved reference: i") || strings.Contains(d.Message, "unresolved reference: origin") {
			t.Errorf("unexpected diagnostic %v", d)
		}
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		if name != "origin" && name != "i" {
			continue
		}
		byName[name]++
		if _, ok := cold.ResolveReference(ref); !ok {
			t.Errorf("`%s` at %v did not resolve", name, ref.QN.Span())
		}
	}
	if byName["origin"] != 2 || byName["i"] != 1 {
		t.Errorf("body references collected: %v, want origin twice and i once", byName)
	}
}

// An unnamed transition's trailing body declares into a scope of its own, nested
// under the state and holding the trigger's parameters below it: a feature it
// declares is seen by a later member, and its sends reach the accept's parameter.
func TestUnnamedTransitionBodyDeclaresItsOwnScope(t *testing.T) {
	const src = `package App {
	item def Request { attribute id; }
	state def Server {
		attribute retries;
		state idle;
		state busy;
		transition first idle accept origin : Request then busy {
			attribute retries;
			send new Request(id = retries) to origin;
			send new Request(count = 1) to missing;
		}
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 2 {
		t.Fatalf("the label `count` and the receiver `missing` must be the two unresolved names, got %v", walk.Diagnostics)
	}
	pkg := unwrapMember(root.Members[0])
	server := memberOf(pkg, 1)
	serverScope := rootScope.ChildFor(pkg).ChildFor(server)
	trans, ok := memberOf(server, 3).(*ast.TransitionMember)
	if !ok {
		t.Fatalf("the transition is a %T", memberOf(server, 3))
	}
	body := serverScope.ChildFor(trans)
	if body == nil {
		t.Fatal("the unnamed transition owns no scope")
	}
	local, ok := body.LookupLocal("retries")
	if !ok || local.Decl == memberOf(server, 0) {
		t.Fatal("the body's `attribute retries` is not a member of the transition's scope")
	}
	if _, ok := serverScope.LookupLocal("retries"); !ok {
		t.Fatal("the state's own `attribute retries` was not indexed")
	}
	trigger := symbols.TriggerScope(serverScope, trans)
	if trigger != body {
		t.Fatalf("TriggerScope is %v, want the transition's own scope", trigger)
	}
	if origin, ok := body.LookupLocal("origin"); !ok || origin.OwnerScope != body {
		t.Error("the accept's parameter is not a member of the transition")
	}
	if _, ok := serverScope.LookupLocal("origin"); ok {
		t.Error("the accept's parameter escaped into the state")
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "retries" && name != "origin" && name != "id" {
			continue
		}
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Errorf("`%s` at %v did not resolve", name, ref.QN.Span())
		}
		if name == "retries" && sym != local {
			t.Errorf("`retries` at %v reached %v, want the body's own declaration", ref.QN.Span(), sym)
		}
	}
	if byName["retries"] != 1 || byName["origin"] != 1 || byName["id"] != 1 || byName["count"] != 1 || byName["missing"] != 1 {
		t.Errorf("body references collected: %v, want retries, origin, id, count and missing once each", byName)
	}
}

// A control node's body — an initial node's, a named fork's, an unnamed decision's —
// declares into a scope of the node's own and is resolved and collected there, so
// a constructor's unknown type or label in it is reported like one anywhere else.
func TestControlNodeBodiesResolveInTheirOwnScope(t *testing.T) {
	const src = `package App {
	item def Request { attribute id; }
	action def Flow {
		attribute retries;
		action a;
		action b;
		first a then b {
			attribute retries;
			send new Request(id = retries) to a;
			send new Missing() to a;
		}
		fork f {
			attribute retries;
			send new Request(nope = retries) to b;
		}
		decide {
			attribute retries;
			send new Request(id = retries) to b;
			send new Request(id = 1) to gone;
		}
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 3 {
		t.Fatalf("the type `Missing`, the label `nope` and the receiver `gone` must be the three unresolved names, got %v", walk.Diagnostics)
	}
	pkg := unwrapMember(root.Members[0])
	flow := memberOf(pkg, 1)
	flowScope := rootScope.ChildFor(pkg).ChildFor(flow)
	outer, ok := flowScope.LookupLocal("retries")
	if !ok {
		t.Fatal("the action's own `attribute retries` was not indexed")
	}
	locals := map[*symbols.Symbol]bool{}
	for i := 3; i <= 5; i++ {
		node := memberOf(flow, i)
		body := flowScope.ChildFor(node)
		if body == nil {
			t.Fatalf("the %T owns no scope", node)
		}
		local, ok := body.LookupLocal("retries")
		if !ok || local == outer {
			t.Fatalf("the %T body's `attribute retries` is not a member of the node's scope", node)
		}
		locals[local] = true
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "retries" && name != "id" {
			continue
		}
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Errorf("`%s` at %v did not resolve", name, ref.QN.Span())
		}
		if name == "retries" && !locals[sym] {
			t.Errorf("`retries` at %v reached %v, want the body's own declaration", ref.QN.Span(), sym)
		}
	}
	if byName["retries"] != 3 || byName["id"] != 3 || byName["Missing"] != 1 || byName["nope"] != 1 || byName["gone"] != 1 {
		t.Errorf("body references collected: %v, want retries and id thrice, Missing, nope and gone once", byName)
	}
}

// memberOf returns the i-th member declaration of a package or definition node.
func memberOf(n ast.Node, i int) ast.Node {
	switch v := n.(type) {
	case *ast.Package:
		return unwrapMember(v.Members[i])
	case *ast.Definition:
		return unwrapMember(v.Members[i])
	}
	return nil
}

// unwrapMember is the member a membership wraps, or the node itself.
func unwrapMember(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

// A named multiplicity's bounds and the multiplicity it subsets are names an
// editor navigates, so the collector has to report them.
func TestNamedMultiplicitiesReferencesAreCollected(t *testing.T) {
	const src = `package M {
	attribute def Count;
	attribute limit : Count;
	multiplicity exactlyOne [1];
	multiplicity upToLimit [0..limit];
	multiplicity fewer subsets upToLimit;
}`
	r, root, rootScope := resolvedDocNamed(t, "m.kerml", src)
	found := map[string]bool{}
	for _, ref := range resolve.References(root, rootScope) {
		found[nameText(ref.QN)] = true
		if _, ok := r.ResolveReference(ref); !ok {
			t.Errorf("%s does not resolve on its own", nameText(ref.QN))
		}
	}
	for _, want := range []string{"Count", "limit", "upToLimit"} {
		if !found[want] {
			t.Errorf("References does not report %s", want)
		}
	}
}

// A named multiplicity's body declares members whose references resolve from
// that body, and the document walk and the collector both descend into it.
func TestMultiplicityBodyMembersResolveFromTheBody(t *testing.T) {
	const src = `package M {
	datatype T;
	feature base : T;
	multiplicity m [1..2] {
		datatype T;
		feature f : T;
		feature g subsets base;
	}
}`
	walk, root, rootScope := resolvedDocNamed(t, "m.kerml", src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}
	var found []string
	for _, ref := range resolve.References(root, rootScope) {
		sym, ok := walk.ResolveReference(ref)
		if !ok {
			t.Errorf("%s does not resolve on its own", nameText(ref.QN))
			continue
		}
		found = append(found, nameText(ref.QN)+" -> "+symbols.FQNOf(sym))
	}
	want := []string{"T -> M::T", "T -> M::m::T", "base -> M::base"}
	if !reflect.DeepEqual(found, want) {
		t.Errorf("References resolve as %v, want %v", found, want)
	}
}

// A cast names its type and may bound it by a feature; both are references the
// document walk resolves and the collector reports.
func TestCastReferencesAreCollected(t *testing.T) {
	const src = `package C {
	part def Shape;
	attribute limit;
	part s : Shape;
	attribute one = (as Shape);
	attribute some = (as Shape[0..limit]);
}`
	walk, root, rootScope := resolvedDocNamed(t, "c.sysml", src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}
	found := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		found[nameText(ref.QN)]++
		if _, ok := walk.ResolveReference(ref); !ok {
			t.Errorf("%s does not resolve on its own", nameText(ref.QN))
		}
	}
	if found["Shape"] != 3 || found["limit"] != 1 {
		t.Errorf("References reports Shape %d times and limit %d times, want 3 and 1", found["Shape"], found["limit"])
	}
}

// A chain segment spelled as a qualified name the operand has no member for
// reads outward, as the document walk reads it: `w.Gen::G2::x` names G2's x,
// not the x the operand inherits first.
func TestAQualifiedChainSegmentResolvesOutwardOnItsOwn(t *testing.T) {
	const src = `package Gen {
	part def G1 { attribute x; }
	part def G2 { attribute x; }
}
package Chains {
	part def Wheel :> Gen::G1, Gen::G2;
	part w : Wheel;
	attribute a = w.x;
	attribute b = w.Gen::G2::x;
}`
	_, root, rootScope := resolvedDocNamed(t, "chains.sysml", src)
	query, _, _ := resolvedDocNamed(t, "chains.sysml", src) // a fresh resolver, as the editor's is
	reached := map[string]string{}
	for _, ref := range resolve.References(root, rootScope) {
		if ref.Chain == nil {
			continue
		}
		sym, ok := query.ResolveReference(ref)
		if !ok {
			t.Errorf("%s does not resolve on its own", nameText(ref.QN))
			continue
		}
		reached[nameText(ref.QN)] = symbols.FQNOf(sym)
	}
	want := map[string]string{"x": "Gen::G1::x", "Gen::G2::x": "Gen::G2::x"}
	for name, fqn := range want {
		if reached[name] != fqn {
			t.Errorf("`w.%s` reaches %q, want %q", name, reached[name], fqn)
		}
	}
}

// A subsetting of a name a sibling redefines reaches that sibling, which shadows
// the inherited feature in the owning type, on its own and in the document walk
// alike (KerML 7.3.4.5): `y :> x` beside `:>> x` or `x :>> x` names the redefining x.
func TestASubsettingReachesTheSiblingRedefinitionOnItsOwn(t *testing.T) {
	for name, src := range map[string]string{
		"sibling.kerml": `package P {
	class A { feature x; }
	class B :> A {
		feature :>> x;
		feature y :> x;
	}
}`,
		"sibling.sysml": `package P {
	part def A { attribute x; }
	part def B :> A {
		attribute :>> x;
		attribute y :> x;
	}
}`,
		"named.kerml": `package P {
	class A { feature x; }
	class B :> A {
		feature x :>> x;
		feature y :> x;
	}
}`,
		"named.sysml": `package P {
	part def A { attribute x; }
	part def B :> A {
		attribute x :>> x;
		attribute y :> x;
	}
}`,
	} {
		walk, root, rootScope := resolvedDocNamed(t, name, src)
		if len(walk.Diagnostics) != 0 {
			t.Fatalf("%s: %v", name, walk.Diagnostics)
		}
		query, _, _ := resolvedDocNamed(t, name, src) // a fresh resolver, as the editor's is
		var subsetted []resolve.Reference
		for _, ref := range resolve.References(root, rootScope) {
			if nameText(ref.QN) == "x" && ref.Subsetting != nil {
				subsetted = append(subsetted, ref)
			}
		}
		if len(subsetted) != 1 {
			t.Fatalf("%s: %d subsetting references to x, want the one `y :> x` writes", name, len(subsetted))
		}
		ref := subsetted[0]
		walked, ok := walk.PartSymbol(ref.QN, 0)
		if !ok || symbols.FQNOf(walked) != "P::B::x" {
			t.Fatalf("%s: the document walk resolved `y :> x` to %v, want the sibling P::B::x", name, walked)
		}
		for how, read := range map[string]func(resolve.Reference) (*symbols.Symbol, bool){
			"ResolveReference": query.ResolveReference,
			"ProbeReference":   query.ProbeReference,
		} {
			if sym, ok := read(ref); !ok || sym != walked {
				t.Errorf("%s: %s resolves `y :> x` to %v, want the sibling %v the document walk reached", name, how, sym, walked)
			}
		}
		if sym, ok := walk.ResolveReference(ref); !ok || sym != walked {
			t.Errorf("%s: resolving `y :> x` again on the walked resolver gives %v, want %v", name, sym, walked)
		}
	}
}

// An import's target is collected as the import's reference, read with its rule:
// a spelling only the import itself surfaces reaches nothing, while an `import
// all` reaches a private member.
func TestImportTargetsResolveWithoutTheImport(t *testing.T) {
	const src = `package Lib {
	part def Cell;
	private part def Hidden;
}
package Consumer {
	package Lib {
		part def Cell;
	}
	private import Lib::Cell;
}
package Opener {
	private import all Lib::Hidden;
}`
	r, root, rootScope := resolvedDoc(t, src)
	var refs []resolve.Reference
	for _, ref := range resolve.References(root, rootScope) {
		if ref.Import != nil {
			refs = append(refs, ref)
		}
	}
	if len(refs) != 2 {
		t.Fatalf("References tags %d import targets, want 2", len(refs))
	}
	cell, ok := r.ProbeReference(refs[0])
	if !ok || nameText(refs[0].QN) != "Lib::Cell" {
		t.Fatalf("`import Lib::Cell` = %v, %v; want the nested Lib's Cell", cell, ok)
	}
	if sym, ok := r.ProbeReference(refs[0].Spelled(spelling(false, "Cell"))); ok {
		t.Errorf("`Cell` written as the import's own target reaches %s through the import itself", sym.Name)
	}
	if sym, ok := r.ProbeReference(refs[1]); !ok || sym.Name != "Hidden" {
		t.Errorf("`import all Lib::Hidden` = %v, %v; want the private member", sym, ok)
	}
	if sym, ok := r.ProbeReference(refs[1].Spelled(spelling(true, "Lib", "Hidden"))); !ok || sym.Name != "Hidden" {
		t.Errorf("`$::Lib::Hidden` read as an `import all` target = %v, %v; want the private member", sym, ok)
	}
}

// spelling is a qualified name on a fresh node, as a trial spelling is.
func spelling(global bool, parts ...string) *ast.QualifiedName {
	qn := &ast.QualifiedName{Global: global}
	for _, part := range parts {
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: part})
	}
	return qn
}

// `first x` may be written before `x` is declared in the same body, so the
// initial node names that later declaration rather than its own label.
func TestAnInitialReferenceReachesALaterDeclaration(t *testing.T) {
	const src = `package P {
	action def Drive {
		first go;
		then action stop;
		action go;
	}
	action def Idle {
		first start;
		then action wait;
	}
}`
	r, root, _ := resolvedDoc(t, src)
	initials := map[string]*ast.InitialNode{}
	for _, def := range unwrapMember(root.Members[0]).(*ast.Package).Members {
		for _, member := range unwrapMember(def).(*ast.Definition).Members {
			if n, ok := unwrapMember(member).(*ast.InitialNode); ok {
				initials[n.Name()] = n
			}
		}
	}
	sym, ok := r.InitialSymbol(initials["go"])
	if !ok {
		t.Fatal("`first go` names the action declared after it, but InitialSymbol reports none")
	}
	if usage, isUsage := sym.Decl.(*ast.Usage); !isUsage || usage.Ident.Name != "go" {
		t.Errorf("`first go` names %T, want the action usage `go`", sym.Decl)
	}
	if sym, ok := r.InitialSymbol(initials["start"]); ok {
		t.Errorf("`first start` names %T, but no member is called start", sym.Decl)
	}
}

// The name after `first` in an action body's `first a then b;` is the source
// of the succession: it resolves like any reference, to the declared node, and
// is diagnosed when nothing declares it. A one-ended `first a;` stays a start
// marker, bound to the member it names and reported by InitialSymbol alone.
func TestAnActionSuccessionFirstEndIsAnOrdinaryReference(t *testing.T) {
	const src = `package P {
	action def Drive {
		action prep;
		first prep then launch;
		action launch;
		first missing then launch;
	}
	action def Idle {
		first wait;
		action wait;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 1 || !strings.Contains(walk.Diagnostics[0].Message, "unresolved reference: missing") {
		t.Fatalf("`first missing then launch` should be the one unresolved reference, got %v", walk.Diagnostics)
	}
	initials := map[string]*ast.InitialNode{}
	for _, def := range unwrapMember(root.Members[0]).(*ast.Package).Members {
		for _, member := range unwrapMember(def).(*ast.Definition).Members {
			if n, ok := unwrapMember(member).(*ast.InitialNode); ok {
				initials[n.Name()] = n
			}
		}
	}
	prep, ok := walk.EndSymbol(initials["prep"].First)
	if !ok {
		t.Fatal("`first prep then launch` does not bind prep as an end")
	}
	if usage, isUsage := prep.Decl.(*ast.Usage); !isUsage || usage.Ident.Name != "prep" {
		t.Errorf("`first prep then launch` binds %T, want the action usage `prep`", prep.Decl)
	}
	if sym, ok := walk.InitialSymbol(initials["prep"]); !ok || sym != prep {
		t.Errorf("InitialSymbol of `first prep then launch` = %v, %v; want the bound source", sym, ok)
	}
	if _, ok := walk.EndSymbol(initials["missing"].First); ok {
		t.Error("`first missing then launch` binds a source, but nothing declares missing")
	}
	if _, ok := walk.EndSymbol(initials["wait"].First); ok {
		t.Error("`first wait;` is a start marker, not a reference, yet its name was bound as an end")
	}
	if sym, ok := walk.InitialSymbol(initials["wait"]); !ok || sym.Decl.(*ast.Usage).Ident.Name != "wait" {
		t.Errorf("InitialSymbol of `first wait;` = %v, %v; want the action usage `wait`", sym, ok)
	}
	refs := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		refs[nameText(ref.QN)]++
	}
	if refs["prep"] != 1 || refs["missing"] != 1 || refs["wait"] != 0 {
		t.Errorf("references collected: %v; want prep and missing once, wait never", refs)
	}
}

// A state body's `first X then Y;` is a succession whose ends are transition
// endpoints, reaching a nested state or one in a sibling region; an action
// body's `first start then Y;` names an ordinary member.
func TestAnInitialSuccessorInAMachineIsAnEndpoint(t *testing.T) {
	const src = `package P {
	state def M {
		state idle;
		first idle then nested;
		state outer {
			state nested;
		}
	}
	state def Q parallel {
		state a {
			state a1;
		}
		state b {
			state b1;
			first b1 then a1;
		}
	}
	action def A {
		action prep;
		first prep then step;
		action step;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every successor: %v", walk.Diagnostics)
	}
	want := map[string]bool{"nested": true, "a1": true, "step": false}
	seen := map[string]bool{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		endpoint, wanted := want[name]
		if !wanted {
			continue
		}
		seen[name] = true
		if ref.Endpoint != endpoint {
			t.Errorf("`then %s` collected with Endpoint=%v, want %v", name, ref.Endpoint, endpoint)
		}
		sym, ok := walk.ResolveReference(ref)
		if !ok {
			t.Errorf("`then %s` does not resolve on its own", name)
			continue
		}
		if walked, ok := walk.EndSymbol(ref.QN); endpoint && (!ok || walked != sym) {
			t.Errorf("`then %s` is not what the document walk bound as an endpoint", name)
		}
		if walked, ok := walk.PartSymbol(ref.QN, 0); !endpoint && (!ok || walked != sym) {
			t.Errorf("`then %s` is not what the document walk bound as a member", name)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("`then %s` was not collected", name)
		}
	}
}

// Every segment of a chained transition end is an endpoint: the root reaches a
// vertex nested anywhere in the machine, which no lexical lookup from the body does.
func TestAChainedEndpointRootIsAnEndpoint(t *testing.T) {
	const src = `package P {
	state def M {
		entry; then src;
		state src;
		state outer {
			state inner {
				state deep;
			}
		}
		first src then inner.deep;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve the chained end: %v", walk.Diagnostics)
	}
	query, _, _ := resolvedDoc(t, src) // a fresh resolver, as the editor's is
	want := map[string]string{"inner": "P::M::outer::inner", "deep": "P::M::outer::inner::deep"}
	seen := map[string]bool{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		fqn, wanted := want[name]
		if !wanted {
			continue
		}
		seen[name] = true
		if !ref.Endpoint {
			t.Errorf("`inner.deep` collected its %s with Endpoint=false", name)
		}
		if sym, ok := query.ProbeReference(ref); !ok || symbols.FQNOf(sym) != fqn {
			t.Errorf("%s of `inner.deep` = %v, %v; want %s", name, sym, ok, fqn)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s of `inner.deep` was not collected", name)
		}
	}
}

// A chained endpoint's member probed under another spelling reads that spelling
// in the operand's vertex, as the rename check trial-reads a respelled reference.
func TestAChainedEndpointProbesItsRespelledMember(t *testing.T) {
	const src = `package P {
	state def O { state old; }
	state def M {
		entry; then idle;
		state idle;
		state outer : O { state taken; }
		first idle then outer.old;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve the chained end: %v", walk.Diagnostics)
	}
	var ends []resolve.Reference
	for _, ref := range resolve.References(root, rootScope) {
		if ref.Endpoint && ref.Chain != nil {
			ends = append(ends, ref)
		}
	}
	if len(ends) != 1 {
		t.Fatalf("References tags %d chained endpoint members, want the one `outer.old` writes", len(ends))
	}
	ref := ends[0]
	if sym, ok := walk.ProbeReference(ref); !ok || symbols.FQNOf(sym) != "P::O::old" {
		t.Fatalf("`outer.old` = %v, %v; want the inherited P::O::old", sym, ok)
	}
	if sym, ok := walk.ProbeReference(ref.Spelled(spelling(false, "taken"))); !ok || symbols.FQNOf(sym) != "P::M::outer::taken" {
		t.Errorf("`outer.taken` = %v, %v; want the usage's own P::M::outer::taken", sym, ok)
	}
	if sym, ok := walk.ProbeReference(ref.Spelled(spelling(false, "fresh"))); ok {
		t.Errorf("`outer.fresh` = %v; want nothing, outer has no such vertex", sym)
	}
	if sym, ok := walk.EndSymbol(ref.QN); !ok || symbols.FQNOf(sym) != "P::O::old" {
		t.Errorf("the probes moved what the document walk bound `outer.old` to: %v, %v", sym, ok)
	}
}

// resolvedDoc parses and resolves src the way the workspace does, with the model
// attached before the walk.
func resolvedDoc(t *testing.T, src string) (*resolve.Resolver, *ast.RootNamespace, *symbols.Scope) {
	return resolvedDocNamed(t, "app.sysml", src)
}

func resolvedDocNamed(t *testing.T, name, src string) (*resolve.Resolver, *ast.RootNamespace, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	r.ResolveDocument(name, root)
	return r, root, idx.DocumentRoot(name)
}

// nameText renders a qualified name the way it was written.
func nameText(q *ast.QualifiedName) string {
	out := ""
	for i, p := range q.Parts {
		if i > 0 {
			out += "::"
		}
		out += p.Text
	}
	return out
}

// kindsOf describes references by the tags that decide where they resolve.
func kindsOf(refs []resolve.Reference) []string {
	var out []string
	for _, ref := range refs {
		kind := "plain"
		switch {
		case ref.Redefines:
			kind = "redefines"
		case ref.Chain != nil:
			kind = "chain"
		case ref.Referrer != nil:
			kind = "reference"
		}
		out = append(out, kind)
	}
	return out
}
