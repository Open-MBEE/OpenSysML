package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// valueParityModel declares what the value corpus names and the action that
// hands an input back as its output.
const valueParityModel = `package valuedemo {
	private import ScalarValues::*;
	private import Collections::*;
	private import SI::*;

	attribute distinct : Set { :>> elements = (3, 1, 2, 2, 3); }

	enum def Color { red; green; }
	part def Wheel { attribute diameter : Real = 0.5; attribute wear : Real; }
	part wheel : Wheel;
	calc def Twice { in x : Real; return : Real = x * 2; }
	attribute open : Real;

	action Echo {
		in x;
		out y;

		first start;
		action work { assign y := x; }
		done;

		succession first start then work;
		succession first work then done;
	}
}`

// valueParityExpressions are evaluated by both sides, and each result is then
// handed back to both as an action input.
var valueParityExpressions = []string{
	"7", "1.5", "true", `"text"`, "null", "(1, 2, 3)",
	"2 ** 70", "1 / 3", "*",
	"10.0 [SI::m]", "3 [SI::m]", "SI::m", "SI::m / SI::s",
	"valuedemo::Color::red", "valuedemo::wheel", "valuedemo::open",
	"valuedemo::Twice", "valuedemo::Wheel meta KerML::Element",
	"valuedemo::distinct.elements",
	"enginedemo::datum", "enginedemo::posed.transformation",
}

// valueParityInputs are Values no corpus expression yields, handed to both
// sides as an action input: protojson, keyed by the arm each one exercises.
var valueParityInputs = map[string]string{
	"unset":           `{"unset":true}`,
	"complex":         `{"complex":{"real":1.5,"imaginary":-2}}`,
	"array":           `{"array":{"dimensions":["2","2"],"elements":[{"intValue":"1"},{"intValue":"2"},{"intValue":"3"},{"intValue":"4"}]}}`,
	"vector":          `{"vector":{"components":[{"realValue":1},{"realValue":2},{"realValue":3}]}}`,
	"vector_quantity": `{"vectorQuantity":{"components":[{"realMagnitude":1,"unit":"SI::m","unitTerm":{"scaleNum":1,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]}},{"realMagnitude":2,"unit":"SI::m","unitTerm":{"scaleNum":1,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]}}]}}`,
	"set":             `{"set":{"elements":[{"intValue":"1"},{"stringValue":"a"}]}}`,
	"tensor_quantity": `{"tensorQuantity":{"dimensions":["2","2"],"components":[{"realMagnitude":1,"unit":"SI::m","unitTerm":{"scaleNum":1,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]}},{"realMagnitude":2,"unit":"SI::m","unitTerm":{"scaleNum":1,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]}},{"realMagnitude":3,"unit":"SI::m","unitTerm":{"scaleNum":1,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]}},{"realMagnitude":4,"unit":"SI::m","unitTerm":{"scaleNum":1,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]}}]}}`,
}

// TestEngineValueArmParity holds the engine's restated value conversion to
// protoconv's over every arm of Value, so a new arm fails here until both carry it.
func TestEngineValueArmParity(t *testing.T) {
	ctx := context.Background()
	svc, err := sysmlgrpc.NewService(4, "parity")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	var parse map[string][]map[string]string
	if err := json.Unmarshal([]byte(engineParseParams(t)), &parse); err != nil {
		t.Fatalf("decoding the ParseSources request: %v", err)
	}
	parse["documents"] = append(parse["documents"], map[string]string{"name": "values.sysml", "content": valueParityModel})
	parseBody, err := json.Marshal(parse)
	if err != nil {
		t.Fatalf("rendering the ParseSources request: %v", err)
	}
	parsed, err := svc.ParseSources(ctx, mustUnmarshal[pb.ParseSourcesRequest](t, string(parseBody)))
	if err != nil {
		t.Fatalf("grpc ParseSources: %v", err)
	}
	mustCall(t, eng, "ParseSources", string(parseBody))
	hash := parsed.GetModelHash()

	// written: arms both sides rendered alike; read: arms both sides were handed.
	written, read := map[string]bool{}, map[string]bool{}
	note := func(into map[string]bool, msg proto.Message) {
		for _, arm := range valueArms(msg) {
			into[arm] = true
		}
	}
	echo := func(what, value string) {
		params := fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"valuedemo::Echo","inputs":{"x":%s}}`, hash, value)
		req := mustUnmarshal[pb.ExecuteActionRequest](t, params)
		res, err := svc.ExecuteAction(ctx, req)
		if err != nil {
			t.Fatalf("grpc ExecuteAction %s: %v", what, err)
		}
		sameJSON(t, "ExecuteAction "+what, mustMarshal(t, res), mustCall(t, eng, "ExecuteAction", params))
		note(read, req)
		note(written, res)
	}

	for _, expression := range valueParityExpressions {
		params := fmt.Sprintf(`{"modelHash":%q,"expression":%s}`, hash, mustJSON(t, expression))
		res, err := svc.Evaluate(ctx, mustUnmarshal[pb.EvaluateRequest](t, params))
		if err != nil {
			t.Fatalf("grpc Evaluate %q: %v", expression, err)
		}
		if res.GetResult() == nil {
			t.Fatalf("Evaluate %q yields no value: %s", expression, res.GetError())
		}
		sameJSON(t, "Evaluate "+expression, mustMarshal(t, res), mustCall(t, eng, "Evaluate", params))
		note(written, res)
		echo(expression, string(mustMarshal(t, res.GetResult())))
	}
	for arm, value := range valueParityInputs {
		echo(arm, value)
	}

	instParams := fmt.Sprintf(`{"modelHash":%q,"symbolId":"valuedemo::Wheel"}`, hash)
	instRes, err := svc.Instantiate(ctx, mustUnmarshal[pb.InstantiateRequest](t, instParams))
	if err != nil {
		t.Fatalf("grpc Instantiate: %v", err)
	}
	sameJSON(t, "Instantiate", mustMarshal(t, instRes), mustCall(t, eng, "Instantiate", instParams))
	note(written, instRes)

	kinds := (&pb.Value{}).ProtoReflect().Descriptor().Oneofs().ByName("kind").Fields()
	for i := 0; i < kinds.Len(); i++ {
		arm := string(kinds.Get(i).Name())
		if !written[arm] {
			t.Errorf("no compared response carries Value.%s: add an expression or input that yields it", arm)
		}
		if !read[arm] {
			t.Errorf("no compared request carries Value.%s: add an input that supplies it", arm)
		}
	}
}

// sameJSON fails when the service's and the engine's answers decode differently.
func sameJSON(t *testing.T, what string, service, engine []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(service, &a); err != nil {
		t.Fatalf("%s: the service answer is not JSON: %v\n%s", what, err, service)
	}
	if err := json.Unmarshal(engine, &b); err != nil {
		t.Fatalf("%s: the engine answer is not JSON: %v\n%s", what, err, engine)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%s diverges:\nservice: %s\nengine:  %s", what, service, engine)
	}
}

// valueArms names the arm of every Value held anywhere in msg, sorted.
func valueArms(msg proto.Message) []string {
	seen := map[string]bool{}
	var walk func(m protoreflect.Message)
	walk = func(m protoreflect.Message) {
		if m.Descriptor().FullName() == (&pb.Value{}).ProtoReflect().Descriptor().FullName() {
			if fd := m.WhichOneof(m.Descriptor().Oneofs().ByName("kind")); fd != nil {
				seen[string(fd.Name())] = true
			}
		}
		m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			switch {
			case fd.IsMap():
				if fd.MapValue().Message() != nil {
					v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool { walk(mv.Message()); return true })
				}
			case fd.Message() == nil:
			case fd.IsList():
				for i := 0; i < v.List().Len(); i++ {
					walk(v.List().Get(i).Message())
				}
			default:
				walk(v.Message())
			}
			return true
		})
	}
	if msg != nil && msg.ProtoReflect().IsValid() {
		walk(msg.ProtoReflect())
	}
	arms := make([]string, 0, len(seen))
	for a := range seen {
		arms = append(arms, a)
	}
	sort.Strings(arms)
	return arms
}
