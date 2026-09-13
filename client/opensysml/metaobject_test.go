package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

const metaobjectSource = `package M {
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
}`

var seatBeltMeta = opensysml.Metaobject{ElementID: "M::seatBelt", MetaclassID: "SysML::Systems::PartUsage"}

// An element reflected on arrives as a Metaobject naming it and its own
// metaclass over every transport, after the annotations `.metadata` lists
// before it, and a Metaobject sent back binds a Metaobject-typed parameter.
func TestMetaobjectsCrossEveryTransport(t *testing.T) {
	address := startService(t)
	for name, client := range map[string]opensysml.Client{
		"in-process":    newClient(t),
		"connect-proto": dialClient(t, address),
		"connect-json":  dialClient(t, address, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			info, err := client.ServerInfo(ctx)
			if err != nil {
				t.Fatalf("ServerInfo: %v", err)
			}
			if !info.Has(opensysml.CapabilityMetaobjectValues) {
				t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityMetaobjectValues)
			}
			model := parse(t, client, metaobjectSource)

			asFeature, err := client.Evaluate(ctx, model, "M::asFeature")
			if err != nil {
				t.Fatalf("Evaluate(M::asFeature): %v", err)
			}
			if !opensysml.Equal(asFeature, opensysml.Sequence{seatBeltMeta}) || asFeature.(opensysml.Sequence)[0] != seatBeltMeta {
				t.Errorf("M::asFeature = %#v, want [%v]", asFeature, seatBeltMeta)
			}

			everything, err := client.Evaluate(ctx, model, "M::everything")
			if err != nil {
				t.Fatalf("Evaluate(M::everything): %v", err)
			}
			seq, ok := everything.(opensysml.Sequence)
			if !ok || len(seq) != 2 {
				t.Fatalf("M::everything = %#v, want the Safety annotation then the metaobject", everything)
			}
			if _, ok := seq[0].(opensysml.InstanceID); !ok {
				t.Errorf("M::everything[0] = %#v, want the Safety annotation object", seq[0])
			}
			if seq[1] != seatBeltMeta {
				t.Errorf("M::everything[1] = %#v, want %v", seq[1], seatBeltMeta)
			}

			none, err := client.Evaluate(ctx, model, "M::notADefinition")
			if err != nil {
				t.Fatalf("Evaluate(M::notADefinition): %v", err)
			}
			if !opensysml.Equal(none, opensysml.Sequence{}) {
				t.Errorf("M::notADefinition = %#v, want the empty sequence", none)
			}

			named, err := client.EvaluateCalc(ctx, model, "M::nameOf", seatBeltMeta)
			if err != nil {
				t.Fatalf("EvaluateCalc(nameOf): %v", err)
			}
			if named.Result != opensysml.String("seatBelt") {
				t.Errorf("nameOf(meta) = %#v, want \"seatBelt\"", named.Result)
			}

			// The metaclass may be left to the model; identity is the element's.
			same, err := client.EvaluateCalc(ctx, model, "M::sameAs", opensysml.Metaobject{ElementID: "M::seatBelt"})
			if err != nil {
				t.Fatalf("EvaluateCalc(sameAs): %v", err)
			}
			if same.Result != opensysml.Bool(true) {
				t.Errorf("sameAs(meta) = %#v, want true", same.Result)
			}

			for label, bad := range map[string]opensysml.Metaobject{
				"an unknown element":     {ElementID: "M::nothing"},
				"a mismatched metaclass": {ElementID: "M::seatBelt", MetaclassID: "SysML::Systems::PartDefinition"},
			} {
				if _, err := client.EvaluateCalc(ctx, model, "M::nameOf", bad); err == nil {
					t.Errorf("EvaluateCalc with a metaobject naming %s succeeded", label)
				}
			}
			var status *opensysml.StatusError
			_, err = client.EvaluateCalc(ctx, model, "M::nameOf", opensysml.Metaobject{})
			if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
				t.Errorf("EvaluateCalc with an empty metaobject: err = %v, want CodeInvalidArgument", err)
			}
		})
	}
}

// A service without metaobject_values would read a metaobject input as null,
// so the client refuses to send one, however deeply nested; and what such a
// service reports for an element reflected on is an unsupported null naming it.
func TestMetaobjectInputNeedsMetaobjectValues(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityMetaobjectValues})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	for name, client := range map[string]opensysml.Client{
		"connect-proto": dialClient(t, server.URL),
		"connect-json":  dialClient(t, server.URL, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			model := parse(t, client, metaobjectSource)
			for label, input := range map[string]opensysml.Value{
				"metaobject":  seatBeltMeta,
				"nested":      opensysml.Sequence{opensysml.Int(1), opensysml.Sequence{seatBeltMeta}},
				"in a set":    opensysml.Set{seatBeltMeta},
				"in an array": opensysml.Array{Dimensions: []int64{1}, Elements: []opensysml.Value{seatBeltMeta}},
			} {
				_, err := client.EvaluateCalc(ctx, model, "M::nameOf", input)
				var status *opensysml.StatusError
				if !errors.As(err, &status) || status.Code != opensysml.CodeUnimplemented || !strings.Contains(status.Message, opensysml.CapabilityMetaobjectValues) {
					t.Errorf("EvaluateCalc %s: err = %v, want CodeUnimplemented naming %s", label, err, opensysml.CapabilityMetaobjectValues)
				}
			}

			asFeature, err := client.Evaluate(ctx, model, "M::asFeature")
			if err != nil {
				t.Fatalf("Evaluate(M::asFeature): %v", err)
			}
			want := opensysml.Sequence{opensysml.Null("unsupported: metaobject M::seatBelt : SysML::Systems::PartUsage")}
			if !opensysml.Equal(asFeature, want) || asFeature.(opensysml.Sequence)[0] != want[0] {
				t.Errorf("M::asFeature without metaobject_values = %#v, want %#v", asFeature, want)
			}
		})
	}
}

// Two metaobjects are the same element whatever type each was cast to.
func TestMetaobjectEqualityIsTheElements(t *testing.T) {
	asType := opensysml.Metaobject{ElementID: "M::seatBelt", MetaclassID: "SysML::Systems::PartUsage"}
	bare := opensysml.Metaobject{ElementID: "M::seatBelt"}
	other := opensysml.Metaobject{ElementID: "M::Vehicle", MetaclassID: "SysML::Systems::PartDefinition"}
	if !opensysml.Equal(asType, bare) {
		t.Errorf("Equal(%v, %v) = false, want the same element", asType, bare)
	}
	if opensysml.Equal(asType, other) {
		t.Errorf("Equal(%v, %v) = true, want different elements", asType, other)
	}
	if got := fmt.Sprintf("%v", asType); got != "meta(M::seatBelt : SysML::Systems::PartUsage)" {
		t.Errorf("%#v renders as %q", asType, got)
	}
}
