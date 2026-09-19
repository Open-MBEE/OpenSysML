package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// queryTypesOf reports the `@type` every named element of a source is projected
// with, keyed by qualified name.
func queryTypesOf(t *testing.T, source string) map[string]string {
	t.Helper()
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: source},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{Select: []string{QueryPropType}},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	types := map[string]string{}
	for _, element := range resp.Elements {
		types[element.Id] = element.Type
	}
	return types
}

// assertTypes checks the `@type` of each named element against what the
// metamodel calls that construct.
func assertTypes(t *testing.T, types map[string]string, want map[string]string) {
	t.Helper()
	for id, wantType := range want {
		got, ok := types[id]
		if !ok {
			t.Errorf("%s was not selected; selected %v", id, types)
			continue
		}
		if got != wantType {
			t.Errorf("%s @type = %q, want %q", id, got, wantType)
		}
	}
}

// TestQueryTypeOfConnectorEnds verifies a named end reports the metaclass the
// connector that declares it gives it: a ReferenceUsage on a connection
// (SysML.xtext ConnectorEnd), a PortUsage on an interface (InterfaceEnd).
func TestQueryTypeOfConnectorEnds(t *testing.T) {
	types := queryTypesOf(t, `
package Ends {
	port def P;
	part def Node {
		port p : P;
	}
	part a : Node;
	part b : Node;
	connection c connect source references a to target references b;
	interface i connect client references a.p to server references b.p;
}
`)
	assertTypes(t, types, map[string]string{
		"Ends::c::source": "ReferenceUsage",
		"Ends::c::target": "ReferenceUsage",
		"Ends::i::client": "PortUsage",
		"Ends::i::server": "PortUsage",
	})
}

// TestQueryTypeOfKerMLTypes verifies a KerML type declaration reports its own
// metaclass rather than nothing: symbols.SymbolKerMLType spans all of them.
func TestQueryTypeOfKerMLTypes(t *testing.T) {
	types := queryTypesOf(t, `
package Kerml {
	class C;
	struct S;
	assoc A;
	behavior B;
	predicate Pr;
}
`)
	assertTypes(t, types, map[string]string{
		"Kerml::C":  "Class",
		"Kerml::S":  "Structure",
		"Kerml::A":  "Association",
		"Kerml::B":  "Behavior",
		"Kerml::Pr": "Predicate",
	})
}

// TestQueryTypeOfSatisfyRequirement verifies a satisfy usage reports the
// metamodel's SatisfyRequirementUsage (SysML.xtext SatisfyRequirementUsage).
func TestQueryTypeOfSatisfyRequirement(t *testing.T) {
	types := queryTypesOf(t, `
package Sat {
	requirement def R;
	part p {
		satisfy requirement s : R;
	}
}
`)
	assertTypes(t, types, map[string]string{"Sat::p::s": "SatisfyRequirementUsage"})
}

// TestMetamodelTypeNameCoversEveryKindDeclared verifies every symbol kind a
// declaration can have has a name, the two kinds refined from the declaration
// included — the old totality test stopped at SymbolConnectorEnd.
func TestMetamodelTypeNameCoversEveryKindDeclared(t *testing.T) {
	refined := map[symbols.SymbolKind]bool{symbols.SymbolKerMLType: true}
	for kind := symbols.SymbolPackage; kind <= symbols.SymbolSatisfyRequirementUsage; kind++ {
		if refined[kind] {
			continue
		}
		if corequery.MetamodelTypeName(kind) == "" {
			t.Errorf("symbol kind %q has no metamodel type name", kind)
		}
	}
	if corequery.MetamodelTypeName(symbols.SymbolUnknown) != "" {
		t.Error("an unclassified declaration must have no metamodel type name")
	}
	// A KerML type is named from its keyword, so the kind alone names nothing.
	if corequery.MetamodelTypeName(symbols.SymbolKerMLType) != "" {
		t.Error("a KerML type kind must be named from its declaration, not its kind")
	}
}
