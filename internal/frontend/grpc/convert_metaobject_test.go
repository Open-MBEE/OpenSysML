package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
)

// metaobjectWireModel yields metaobjects through `meta` and `.metadata`, and
// takes one back as a calc argument whose features it reads.
const metaobjectWireModel = `
package M {
  private import ScalarValues::*;

  metadata def Safety { attribute level : Integer = 2; }

  part def Vehicle { attribute mass : Real; }
  part seatBelt : Vehicle { @Safety { level = 4; } }

  attribute asFeature [*] = seatBelt meta KerML::Feature;
  attribute everything [*] = seatBelt.metadata;
  attribute notADefinition [*] = seatBelt meta SysML::PartDefinition;

  calc def NameOf { in m : Metaobjects::Metaobject; return : String = m.declaredName; }
  calc nameOf : NameOf;
  calc def SameAs { in m : Metaobjects::Metaobject; return : Boolean = m === (seatBelt meta KerML::Type)#(1); }
  calc sameAs : SameAs;
}
`

func metaobjectValue(elementID, metaclassID string) *pb.Value {
	return &pb.Value{Kind: &pb.Value_Metaobject{Metaobject: &pb.Metaobject{ElementId: elementID, MetaclassId: metaclassID}}}
}

// A metaobject crosses as its own arm naming the element it reflects and that
// element's own metaclass by FQN, after any annotation objects it follows, and
// reads back as the metaobject of the same element.
func TestMetaobjectRoundTrip(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, metaobjectWireModel)
	cached, ok := srv.cache.Get(modelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	want := &pb.Metaobject{ElementId: "M::seatBelt", MetaclassId: "SysML::Systems::PartUsage"}
	asFeature := mustEvaluate(t, srv, modelHash, "M::asFeature")
	elems := asFeature.GetSequence().GetElements()
	if len(elems) != 1 || elems[0].GetMetaobject() == nil {
		t.Fatalf("M::asFeature crossed as %v, want a sequence of one metaobject", asFeature)
	}
	if got := elems[0].GetMetaobject(); got.GetElementId() != want.ElementId || got.GetMetaclassId() != want.MetaclassId {
		t.Errorf("M::asFeature metaobject = %v, want %v", got, want)
	}

	// `.metadata` sends the annotation object first, the reflective metaobject last.
	everything := mustEvaluate(t, srv, modelHash, "M::everything")
	elems = everything.GetSequence().GetElements()
	if len(elems) != 2 || elems[0].GetInstanceId() == 0 || elems[1].GetMetaobject() == nil {
		t.Fatalf("M::everything crossed as %v, want (instance, metaobject)", everything)
	}
	if got := elems[1].GetMetaobject(); got.GetElementId() != want.ElementId || got.GetMetaclassId() != want.MetaclassId {
		t.Errorf("M::everything metaobject = %v, want %v", got, want)
	}
	if got := mustEvaluate(t, srv, modelHash, "M::notADefinition"); len(got.GetSequence().GetElements()) != 0 {
		t.Errorf("M::notADefinition crossed as %v, want ()", got)
	}

	back, err := protoconv.ProtoToValueIn(elems[1], idx, sem)
	if err != nil {
		t.Fatalf("ProtoToValueIn: %v", err)
	}
	if back.Kind != runtime.ValMetaobject {
		t.Fatalf("read back as %s", back.Kind)
	}
	if got := runtime.FormatValue(back); got != "meta(M::seatBelt : SysML::Systems::PartUsage)" {
		t.Errorf("round trip = %s", got)
	}
	if idx.GetFQN(back.MetaobjectElement()) != want.ElementId || idx.GetFQN(back.MetaobjectClass()) != want.MetaclassId {
		t.Errorf("round trip names %s : %s", idx.GetFQN(back.MetaobjectElement()), idx.GetFQN(back.MetaobjectClass()))
	}
	again := protoconv.ValueToProto(back, idx)
	if got := again.GetMetaobject(); got.GetElementId() != want.ElementId || got.GetMetaclassId() != want.MetaclassId {
		t.Errorf("re-sent as %v, want %v", again, want)
	}

	// A client may omit the metaclass: the model's is used.
	bare, err := protoconv.ProtoToValueIn(metaobjectValue("M::seatBelt", ""), idx, sem)
	if err != nil {
		t.Fatalf("protoconv.ProtoToValueIn without metaclass_id: %v", err)
	}
	if idx.GetFQN(bare.MetaobjectClass()) != want.MetaclassId {
		t.Errorf("metaobject without metaclass_id read back under %s", idx.GetFQN(bare.MetaobjectClass()))
	}
	if bare.MetaobjectElement() != back.MetaobjectElement() {
		t.Errorf("metaobjects of one element read back as distinct: %s and %s", runtime.FormatValue(bare), runtime.FormatValue(back))
	}
}

// A metaobject naming no element, an element no metaclass classifies, or a
// metaclass other than the element's own is refused with a typed error.
func TestMalformedMetaobjectsAreRejected(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, metaobjectWireModel)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	cases := []struct {
		name string
		val  *pb.Value
		want error
	}{
		{"empty element_id", metaobjectValue("", "KerML::Feature"), protoconv.ErrMetaobjectUnbound},
		{"unknown element", metaobjectValue("M::nope", ""), protoconv.ErrMetaobjectUnbound},
		{"metaclass of another kind", metaobjectValue("M::seatBelt", "SysML::Systems::PartDefinition"), protoconv.ErrMetaclassMismatch},
		{"metaclass the element conforms to but is not", metaobjectValue("M::seatBelt", "KerML::Feature"), protoconv.ErrMetaclassMismatch},
		{"nested in a sequence", sequenceValue(intValue(1), metaobjectValue("M::nope", "")), protoconv.ErrMetaobjectUnbound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := protoconv.ProtoToValueIn(tc.val, idx, sem)
			if !errors.Is(err, tc.want) {
				t.Fatalf("protoconv.ProtoToValueIn = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.val.GetMetaobject().GetElementId()) && tc.val.GetMetaobject() != nil && tc.val.GetMetaobject().GetElementId() != "" {
				t.Errorf("error %q does not name the element", err)
			}
		})
	}
	if _, err := protoconv.ProtoToValueIn(metaobjectValue("M::seatBelt", ""), idx, nil); !errors.Is(err, protoconv.ErrMetaobjectUnbound) {
		t.Errorf("protoconv.ProtoToValueIn without a model = %v, want %v", err, protoconv.ErrMetaobjectUnbound)
	}
}

// unnamedMetaobjectModel yields metaobjects of elements no qualified name
// reaches: an unnamed part, and the receiver argument an accept synthesizes.
const unnamedMetaobjectModel = `
package M {
  private import ScalarValues::*;
  part def V;
  part outer : V { part : V; part named : V; }
  attribute features [*] = (outer meta KerML::Type).ownedFeature;
  attribute multiplicities [*] = (outer::named meta KerML::Feature).ownedElement;
  port def P;
  part def Machine {
    port p : P;
    state sm { entry; then idle; state idle; transition t first idle accept sig : Integer via p then idle; }
  }
  attribute receiverArgument [*] = (Machine::sm::t meta SysML::TransitionUsage).triggerAction.receiverArgument;
  calc def IsAnonymous { in m : Metaobjects::Metaobject; return : Boolean[1] = m === (outer meta KerML::Type).ownedFeature#(1); }
  calc isAnonymous : IsAnonymous;
  calc def IsReceiver { in m : Metaobjects::Metaobject; return : Boolean[1] = m === receiverArgument#(1); }
  calc isReceiver : IsReceiver;
}
`

// An unnamed element's metaobject crosses under a positional id or form @id,
// which GetElementProperties reads and the service binds back to the element.
func TestUnnamedMetaobjectCrossesUnderItsElementID(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, unnamedMetaobjectModel)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem, sc := cached.Index, NewSymbolContext(cached.Index).Semantics, cached.SymbolContext()

	features := mustEvaluate(t, srv, modelHash, "M::features").GetSequence().GetElements()
	if len(features) != 2 {
		t.Fatalf("M::features crossed as %v, want two elements", features)
	}
	if got := features[0].GetMetaobject(); got.GetElementId() != "M::outer::@0" || got.GetMetaclassId() != "SysML::Systems::PartUsage" {
		t.Errorf("anonymous part crossed as %v, want M::outer::@0 : SysML::Systems::PartUsage", features[0])
	}
	if got := features[1].GetMetaobject(); got.GetElementId() != "M::outer::named" || got.GetMetaclassId() != "SysML::Systems::PartUsage" {
		t.Errorf("named part crossed as %v, want M::outer::named : SysML::Systems::PartUsage", features[1])
	}
	receiver := mustEvaluate(t, srv, modelHash, "M::receiverArgument").GetSequence().GetElements()
	const receiverID = "M__Machine__sm__t___40trigger___401_pvalue"
	if len(receiver) != 1 || receiver[0].GetMetaobject().GetElementId() != receiverID {
		t.Fatalf("M::receiverArgument crossed as %v, want the metaobject of %s", receiver, receiverID)
	}
	if got := receiver[0].GetMetaobject().GetMetaclassId(); !strings.HasSuffix(got, "::FeatureReferenceExpression") {
		t.Errorf("receiver argument classified as %s, want a FeatureReferenceExpression", got)
	}

	for _, tc := range []struct {
		sent         *pb.Value
		formID, calc string
	}{
		{features[0], "M__outer___400", "M::isAnonymous"},
		{receiver[0], receiverID, "M::isReceiver"},
	} {
		id := tc.sent.GetMetaobject().GetElementId()
		back, err := protoconv.ProtoToRuntimeValueWith(nil, tc.sent, idx, sem, sc)
		if err != nil {
			t.Fatalf("ProtoToRuntimeValueWith(%s): %v", id, err)
		}
		if back.Kind != runtime.ValMetaobject || sc.ElementIdentity(back.MetaobjectElement()) != id {
			t.Errorf("%s read back as %s", id, runtime.FormatValue(back))
		}
		if again := protoconv.ValueToProtoWith(nil, back, idx, sc); again.GetMetaobject().GetElementId() != id {
			t.Errorf("%s re-sent as %v", id, again)
		}
		props, err := srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{ModelHash: modelHash, ElementId: id})
		if err != nil || props.GetError() != "" || props.GetElement().GetId() != tc.formID {
			t.Errorf("GetElementProperties(%s) = %v, %v; want the element %s", id, props.GetElement(), err, tc.formID)
		}
		calc, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: tc.calc, Arguments: []*pb.Value{tc.sent}})
		if err != nil || calc.GetError() != "" || !calc.GetResult().GetBoolValue() {
			t.Errorf("EvaluateCalc(%s, %s) = %v, %v; want true", tc.calc, id, calc.GetResult(), err)
		}
		if _, err := protoconv.ProtoToValueIn(tc.sent, idx, sem); !errors.Is(err, protoconv.ErrMetaobjectUnbound) {
			t.Errorf("an index alone resolved %s: %v", id, err)
		}
	}

	// An element the form gives no id either crosses as the unsupported null.
	multiplicities := mustEvaluate(t, srv, modelHash, "M::multiplicities").GetSequence().GetElements()
	if len(multiplicities) != 1 || multiplicities[0].GetNull() != protoconv.UnsupportedUnidentifiedMetaobject {
		t.Errorf("M::multiplicities crossed as %v, want one unsupported null", multiplicities)
	}
}

// An element_id two documents both declare identifies neither declaration, so
// it is refused as ambiguous rather than bound to whichever the index lists first.
func TestAmbiguousMetaobjectIsRejected(t *testing.T) {
	srv := mustNewService(t, 4)
	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("a.sysml", "package M { part def V; part seatBelt : V; }", "b.sysml", "package M { part def W; part seatBelt : W; }"),
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("ParseSources: err = %v, error = %q", err, resp.GetError())
	}
	cached, ok := srv.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics
	if n := len(idx.LookupQualified("M::seatBelt")); n != 2 {
		t.Fatalf("M::seatBelt resolves to %d elements, want the two declarations", n)
	}
	_, err = protoconv.ProtoToValueIn(metaobjectValue("M::seatBelt", ""), idx, sem)
	if !errors.Is(err, protoconv.ErrMetaobjectAmbiguous) || !strings.Contains(err.Error(), "M::seatBelt") {
		t.Fatalf("protoconv.ProtoToValueIn = %v, want %v naming M::seatBelt", err, protoconv.ErrMetaobjectAmbiguous)
	}
	if _, err := protoconv.ProtoToValueIn(metaobjectValue("M::V", ""), idx, sem); err != nil {
		t.Errorf("protoconv.ProtoToValueIn(M::V), declared once: %v", err)
	}
}

// A metaobject sent as a calc argument binds to the element it names, so the
// calc reads that element's features and finds it identical to the model's own.
func TestMetaobjectCrossesAsCalcArgument(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, metaobjectWireModel)

	meta := mustEvaluate(t, srv, modelHash, "M::asFeature").GetSequence().GetElements()[0]
	calc, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::nameOf", Arguments: []*pb.Value{meta}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(nameOf): err = %v, error = %q", err, calc.GetError())
	}
	if soleElement(t, "nameOf", calc.Result).GetStringValue() != "seatBelt" {
		t.Errorf("nameOf(meta) = %v, want \"seatBelt\"", calc.Result)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::sameAs", Arguments: []*pb.Value{
		metaobjectValue("M::seatBelt", ""),
	}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(sameAs): err = %v, error = %q", err, calc.GetError())
	}
	if !soleElement(t, "sameAs", calc.Result).GetBoolValue() {
		t.Errorf("sameAs(meta) = %v, want true", calc.Result)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::nameOf", Arguments: []*pb.Value{
		metaobjectValue("M::seatBelt", "KerML::Feature"),
	}})
	if err != nil {
		t.Fatalf("EvaluateCalc(mismatched): %v", err)
	}
	if !strings.Contains(calc.Error, protoconv.ErrMetaclassMismatch.Error()) {
		t.Errorf("EvaluateCalc(mismatched) error = %q, want one naming %v", calc.Error, protoconv.ErrMetaclassMismatch)
	}
}

// Without metaobject_values a metaobject is withheld as an unsupported null
// naming the element and its metaclass, and one sent is refused UNIMPLEMENTED.
func TestMetaobjectCapability(t *testing.T) {
	ctx := context.Background()
	found := false
	for _, c := range Capabilities() {
		found = found || c == CapabilityMetaobjectValues
	}
	if !found {
		t.Errorf("capabilities %v do not include %q", Capabilities(), CapabilityMetaobjectValues)
	}

	without := mustNewServiceWithout(t, CapabilityMetaobjectValues)
	modelHash := mustParse(t, without, metaobjectWireModel)
	elems := mustEvaluate(t, without, modelHash, "M::everything").GetSequence().GetElements()
	if len(elems) != 2 || elems[0].GetInstanceId() == 0 {
		t.Fatalf("M::everything without %s = %v, want (instance, null)", CapabilityMetaobjectValues, elems)
	}
	if want := "unsupported: metaobject M::seatBelt : SysML::Systems::PartUsage"; elems[1].GetNull() != want {
		t.Errorf("metaobject without %s = %v, want null %q", CapabilityMetaobjectValues, elems[1], want)
	}
	for name, input := range map[string]*pb.Value{
		"a metaobject":           metaobjectValue("M::seatBelt", ""),
		"a sequence holding one": sequenceValue(metaobjectValue("M::seatBelt", "")),
	} {
		_, err := without.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::nameOf", Arguments: []*pb.Value{input}})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityMetaobjectValues) {
			t.Errorf("EvaluateCalc with %s argument without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityMetaobjectValues, err)
		}
	}

	// A set of two metaobjects is named as two, not as one null the set collapsed.
	set := setOf(metaobjectValue("M::seatBelt", "SysML::Systems::PartUsage"), metaobjectValue("M::Vehicle", "SysML::Systems::PartDefinition"))
	without.filterValueCapabilities(set)
	if want := "unsupported: set Set{meta(M::Vehicle : SysML::Systems::PartDefinition), meta(M::seatBelt : SysML::Systems::PartUsage)} holding metaobject M::seatBelt : SysML::Systems::PartUsage"; set.GetNull() != want {
		t.Errorf("set of metaobjects without %s = %q, want %q", CapabilityMetaobjectValues, set.GetNull(), want)
	}

	if !protoconv.ValueCarriesMetaobject(sequenceValue(intValue(1), setOf(metaobjectValue("M::seatBelt", "")))) {
		t.Error("protoconv.ValueCarriesMetaobject misses a metaobject nested in a set in a sequence")
	}
	if protoconv.ValueCarriesMetaobject(sequenceValue(intValue(1), stringValue("meta"))) {
		t.Error("protoconv.ValueCarriesMetaobject reports one where there is none")
	}
}
