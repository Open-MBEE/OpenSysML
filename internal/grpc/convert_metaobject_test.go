package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
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

	back, err := ProtoToValueIn(elems[1], idx, sem)
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
	again := ValueToProto(back, idx)
	if got := again.GetMetaobject(); got.GetElementId() != want.ElementId || got.GetMetaclassId() != want.MetaclassId {
		t.Errorf("re-sent as %v, want %v", again, want)
	}

	// A client may omit the metaclass: the model's is used.
	bare, err := ProtoToValueIn(metaobjectValue("M::seatBelt", ""), idx, sem)
	if err != nil {
		t.Fatalf("ProtoToValueIn without metaclass_id: %v", err)
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
		{"empty element_id", metaobjectValue("", "KerML::Feature"), ErrMetaobjectUnbound},
		{"unknown element", metaobjectValue("M::nope", ""), ErrMetaobjectUnbound},
		{"metaclass of another kind", metaobjectValue("M::seatBelt", "SysML::Systems::PartDefinition"), ErrMetaclassMismatch},
		{"metaclass the element conforms to but is not", metaobjectValue("M::seatBelt", "KerML::Feature"), ErrMetaclassMismatch},
		{"nested in a sequence", sequenceValue(intValue(1), metaobjectValue("M::nope", "")), ErrMetaobjectUnbound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ProtoToValueIn(tc.val, idx, sem)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ProtoToValueIn = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.val.GetMetaobject().GetElementId()) && tc.val.GetMetaobject() != nil && tc.val.GetMetaobject().GetElementId() != "" {
				t.Errorf("error %q does not name the element", err)
			}
		})
	}
	if _, err := ProtoToValueIn(metaobjectValue("M::seatBelt", ""), idx, nil); !errors.Is(err, ErrMetaobjectUnbound) {
		t.Errorf("ProtoToValueIn without a model = %v, want %v", err, ErrMetaobjectUnbound)
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
	if calc.Result.GetStringValue() != "seatBelt" {
		t.Errorf("nameOf(meta) = %v, want \"seatBelt\"", calc.Result)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::sameAs", Arguments: []*pb.Value{
		metaobjectValue("M::seatBelt", ""),
	}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(sameAs): err = %v, error = %q", err, calc.GetError())
	}
	if !calc.Result.GetBoolValue() {
		t.Errorf("sameAs(meta) = %v, want true", calc.Result)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::nameOf", Arguments: []*pb.Value{
		metaobjectValue("M::seatBelt", "KerML::Feature"),
	}})
	if err != nil {
		t.Fatalf("EvaluateCalc(mismatched): %v", err)
	}
	if !strings.Contains(calc.Error, ErrMetaclassMismatch.Error()) {
		t.Errorf("EvaluateCalc(mismatched) error = %q, want one naming %v", calc.Error, ErrMetaclassMismatch)
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

	if !ValueCarriesMetaobject(sequenceValue(intValue(1), setOf(metaobjectValue("M::seatBelt", "")))) {
		t.Error("ValueCarriesMetaobject misses a metaobject nested in a set in a sequence")
	}
	if ValueCarriesMetaobject(sequenceValue(intValue(1), stringValue("meta"))) {
		t.Error("ValueCarriesMetaobject reports one where there is none")
	}
}
