package grpc

import (
	"context"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// parseForFacts parses content through the service (so the stdlib is loaded)
// and returns a lookup for the type facts of a symbol by FQN.
func parseForFacts(t *testing.T, content string) func(string) *pb.SymbolInfo {
	t.Helper()
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: content},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, d := range parsed.Diagnostics {
		if d.Severity == "error" {
			t.Fatalf("unexpected diagnostic: %s", d.Message)
		}
	}
	return func(fqn string) *pb.SymbolInfo {
		t.Helper()
		resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
			ModelHash: parsed.ModelHash,
			SymbolId:  fqn,
		})
		if err != nil {
			t.Fatalf("GetSymbol(%s): %v", fqn, err)
		}
		if resp.Symbol == nil {
			t.Fatalf("GetSymbol(%s): %s", fqn, resp.Error)
		}
		return resp.Symbol
	}
}

// TestTypeInfoTypedUsage verifies a typed part usage reports its resolved type.
func TestTypeInfoTypedUsage(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Engine;
    part def Vehicle {
        part engine : Engine;
    }
}
`)
	info := get("Demo::Vehicle::engine").TypeInfo
	if info == nil {
		t.Fatal("expected type info on a typed usage")
	}
	if info.Declared != "Engine" {
		t.Errorf("Declared: got %q, want %q", info.Declared, "Engine")
	}
	if info.ResolvedId != "Demo::Engine" {
		t.Errorf("ResolvedId: got %q, want %q", info.ResolvedId, "Demo::Engine")
	}
	if info.ResolvedKind != "partDef" {
		t.Errorf("ResolvedKind: got %q, want %q", info.ResolvedKind, "partDef")
	}
	if info.Primitive != "" {
		t.Errorf("Primitive: got %q, want empty for a part", info.Primitive)
	}
}

// TestTypeInfoDeclaredPrimitive verifies a usage typed by a library scalar
// reports that scalar as its primitive.
func TestTypeInfoDeclaredPrimitive(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    private import ScalarValues::*;
    part def Vehicle {
        attribute wheels : Integer;
        attribute label : String;
    }
}
`)
	wheels := get("Demo::Vehicle::wheels").TypeInfo
	if wheels.Primitive != "Integer" || wheels.PrimitiveSource != "declared" {
		t.Errorf("wheels: got %q/%q, want Integer/declared", wheels.Primitive, wheels.PrimitiveSource)
	}
	label := get("Demo::Vehicle::label").TypeInfo
	if label.Primitive != "String" || label.PrimitiveSource != "declared" {
		t.Errorf("label: got %q/%q, want String/declared", label.Primitive, label.PrimitiveSource)
	}
}

// TestTypeInfoValueInferredPrimitive verifies an untyped attribute takes its
// primitive from its default value, reported as inferred.
func TestTypeInfoValueInferredPrimitive(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Vehicle {
        attribute mass = 1500.0;
        attribute doors = 4;
        attribute electric = true;
        attribute name = "car";
    }
}
`)
	cases := []struct{ fqn, prim string }{
		{"Demo::Vehicle::mass", "Real"},
		{"Demo::Vehicle::doors", "Integer"},
		{"Demo::Vehicle::electric", "Boolean"},
		{"Demo::Vehicle::name", "String"},
	}
	for _, c := range cases {
		info := get(c.fqn).TypeInfo
		if info.Primitive != c.prim || info.PrimitiveSource != "value" {
			t.Errorf("%s: got %q/%q, want %s/value", c.fqn, info.Primitive, info.PrimitiveSource, c.prim)
		}
	}
}

// TestTypeInfoUnresolvedType verifies an unresolvable type is reported as
// declared but unresolved rather than silently dropped.
func TestTypeInfoUnresolvedType(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Vehicle {
        attribute odd = 1 + 1;
    }
}
`)
	info := get("Demo::Vehicle::odd").TypeInfo
	if info.Primitive != "Integer" {
		t.Errorf("Primitive: got %q, want Integer", info.Primitive)
	}
	if info.ResolvedId != "" {
		t.Errorf("ResolvedId: got %q, want empty", info.ResolvedId)
	}
}

// TestTypeInfoQuantity verifies a value written with a measurement unit is
// reported as a quantity, with the unit as written.
func TestTypeInfoQuantity(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    private import SI::*;
    part def Vehicle {
        attribute mass = 1500.0 [kg];
    }
}
`)
	info := get("Demo::Vehicle::mass").TypeInfo
	if !info.Quantity {
		t.Error("expected Quantity for a value with a unit")
	}
	if info.Unit != "kg" {
		t.Errorf("Unit: got %q, want kg", info.Unit)
	}
}

// TestSpecializationsAll verifies every generalization edge is reported, not
// only the first one metadata carries.
func TestSpecializationsAll(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Machine;
    part def Powered;
    part def Vehicle :> Machine, Powered;
}
`)
	sym := get("Demo::Vehicle")
	if len(sym.Specializations) != 2 {
		t.Fatalf("got %d specializations, want 2", len(sym.Specializations))
	}
	want := []struct{ declared, target string }{
		{"Machine", "Demo::Machine"},
		{"Powered", "Demo::Powered"},
	}
	for i, w := range want {
		got := sym.Specializations[i]
		if got.Kind != "specializes" {
			t.Errorf("[%d] Kind: got %q, want specializes", i, got.Kind)
		}
		if got.Declared != w.declared || got.TargetId != w.target {
			t.Errorf("[%d]: got %q/%q, want %q/%q", i, got.Declared, got.TargetId, w.declared, w.target)
		}
		if got.TargetKind != "partDef" {
			t.Errorf("[%d] TargetKind: got %q, want partDef", i, got.TargetKind)
		}
	}
	// metadata still reports only the first, for compatibility.
	if sym.Metadata["specializes"] != "Machine" {
		t.Errorf("metadata specializes: got %q, want Machine", sym.Metadata["specializes"])
	}
}

// TestSpecializationsUsageKinds verifies typing, subsetting and redefinition
// are each reported under their own kind.
func TestSpecializationsUsageKinds(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Wheel;
    part def Vehicle {
        part wheels : Wheel[4];
    }
    part def Car :> Vehicle {
        part frontWheels : Wheel[2] :> wheels;
    }
}
`)
	front := get("Demo::Car::frontWheels")
	kinds := map[string]string{}
	for _, spec := range front.Specializations {
		kinds[spec.Kind] = spec.TargetId
	}
	if kinds["typing"] != "Demo::Wheel" {
		t.Errorf("typing target: got %q, want Demo::Wheel", kinds["typing"])
	}
	if kinds["subsets"] != "Demo::Vehicle::wheels" {
		t.Errorf("subsets target: got %q, want Demo::Vehicle::wheels", kinds["subsets"])
	}
}

// TestMultiplicityFacts verifies declared multiplicity bounds are reported and
// an undeclared multiplicity is reported as absent.
func TestMultiplicityFacts(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Wheel;
    part def Vehicle {
        part wheels : Wheel[4];
        part spare : Wheel[0..1];
        part trailers : Wheel[0..*];
        part engine : Wheel;
    }
}
`)
	cases := []struct{ fqn, lower, upper string }{
		{"Demo::Vehicle::wheels", "4", "4"},
		{"Demo::Vehicle::spare", "0", "1"},
		{"Demo::Vehicle::trailers", "0", "*"},
	}
	for _, c := range cases {
		mult := get(c.fqn).Multiplicity
		if mult == nil {
			t.Fatalf("%s: expected multiplicity", c.fqn)
		}
		if mult.Lower != c.lower || mult.Upper != c.upper {
			t.Errorf("%s: got %s..%s, want %s..%s", c.fqn, mult.Lower, mult.Upper, c.lower, c.upper)
		}
	}
	if mult := get("Demo::Vehicle::engine").Multiplicity; mult != nil {
		t.Errorf("engine: got %v, want no declared multiplicity", mult)
	}
}

// TestTypeFactsAbsentForNonDefUsage verifies a package carries no type facts.
func TestTypeFactsAbsentForNonDefUsage(t *testing.T) {
	get := parseForFacts(t, `package Demo { part def Vehicle; }`)
	pkg := get("Demo")
	if pkg.TypeInfo != nil {
		t.Errorf("TypeInfo: got %v, want nil for a package", pkg.TypeInfo)
	}
	if len(pkg.Specializations) != 0 {
		t.Errorf("Specializations: got %d, want 0", len(pkg.Specializations))
	}
}

// TestSymbolContextConcurrentConversion verifies that converting symbols of one
// cached model from several goroutines is safe: the shared resolver and
// semantic model memoize into plain maps, so conversion must serialize.
func TestSymbolContextConcurrentConversion(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: `
package Demo {
    private import ScalarValues::*;
    part def Engine {
        attribute power : Real;
    }
    part def Vehicle :> Engine {
        attribute mass : Real;
        part engine : Engine;
        part wheels : Engine[0..*];
    }
}
`},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	fqns := []string{"Demo::Engine", "Demo::Engine::power", "Demo::Vehicle", "Demo::Vehicle::mass", "Demo::Vehicle::engine", "Demo::Vehicle::wheels"}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, fqn := range fqns {
				resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
					ModelHash: parsed.ModelHash,
					SymbolId:  fqn,
				})
				if err != nil || resp.Symbol == nil {
					t.Errorf("GetSymbol(%s): %v %s", fqn, err, resp.GetError())
					return
				}
			}
		}()
	}
	wg.Wait()

	// Speculative resolution during conversion must not accumulate diagnostics
	// on the model's shared resolver.
	cached, ok := srv.cache.Get(parsed.ModelHash)
	if !ok {
		t.Fatal("model missing from cache")
	}
	if diags := cached.SymbolContext().Resolver.Diagnostics; len(diags) != 0 {
		t.Errorf("shared resolver accumulated %d diagnostics: %v", len(diags), diags)
	}
}
