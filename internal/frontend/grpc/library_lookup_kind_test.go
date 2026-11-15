package grpc

import (
	"context"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A standard-library element is read as the lookup kind states it: its normative id only as
// an id, on both RPCs, and its qualified name as a name.
func TestLibraryElementIsReadAsTheStatedLookupKind(t *testing.T) {
	r := parseForElementProperties(t, elementPropertiesModel)
	types := r.property("EP::Pair::x", "type")
	if len(types.Values) != 1 || types.Values[0].GetElement().GetQualifiedName() != "ScalarValues::Real" {
		t.Fatalf("EP::Pair::x.type = %+v", types)
	}
	id := types.Values[0].GetElement().Id
	const name = "ScalarValues::Real"
	unspecified, byName, byID := pb.LookupKind_LOOKUP_KIND_UNSPECIFIED, pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME, pb.LookupKind_LOOKUP_KIND_ELEMENT_ID
	for _, tc := range []struct {
		element string
		lookup  pb.LookupKind
		found   bool
	}{
		{id, byID, true}, {id, unspecified, true}, {id, byName, false},
		{name, byName, true}, {name, unspecified, true}, {name, byID, true},
	} {
		properties, err := r.srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{
			ModelHash: r.hash, ElementId: tc.element, Lookup: tc.lookup,
		})
		if err != nil {
			t.Fatalf("GetElementProperties(%s, %v): %v", tc.element, tc.lookup, err)
		}
		if found := properties.Error == ""; found != tc.found || found && properties.Element.Id != id {
			t.Errorf("GetElementProperties(%s, %v) = element %+v, error %q; want found %v", tc.element, tc.lookup, properties.Element, properties.Error, tc.found)
		}
		symbol, err := r.srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: r.hash, SymbolId: tc.element, Lookup: tc.lookup})
		if err != nil {
			t.Fatalf("GetSymbol(%s, %v): %v", tc.element, tc.lookup, err)
		}
		if found := symbol.Error == ""; found != tc.found || found && symbol.Symbol.Id != name {
			t.Errorf("GetSymbol(%s, %v) = symbol %+v, error %q; want found %v", tc.element, tc.lookup, symbol.Symbol, symbol.Error, tc.found)
		}
	}
}

// A library element's normative id names the library element, not a model declaration the
// model spells with the same qualified name.
func TestLibraryIDIsNotTheModelHomonym(t *testing.T) {
	r := parseForElementProperties(t, "package ScalarValues { part def Real { part own; } }\n")
	byID, byName := pb.LookupKind_LOOKUP_KIND_ELEMENT_ID, pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME
	read := func(id string, lookup pb.LookupKind) *pb.SymbolInfo {
		t.Helper()
		symbol, err := r.srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: r.hash, SymbolId: id, Lookup: lookup})
		if err != nil || symbol.Symbol == nil {
			t.Fatalf("GetSymbol(%s, %v) = %+v, %v", id, lookup, symbol, err)
		}
		return symbol.Symbol
	}
	declared := read("ScalarValues::Real", byName)
	if !slices.Contains(declared.ChildIds, "ScalarValues::Real::own") {
		t.Fatalf("ScalarValues::Real by name = %+v; want the model's declaration", declared)
	}
	types := parseForElementProperties(t, elementPropertiesModel).property("EP::Pair::x", "type")
	if len(types.Values) != 1 || types.Values[0].GetElement().GetQualifiedName() != "ScalarValues::Real" {
		t.Fatalf("EP::Pair::x.type = %+v", types)
	}
	libraryID := types.Values[0].GetElement().Id
	if library := read(libraryID, byID); library.Kind == declared.Kind || slices.Contains(library.ChildIds, "ScalarValues::Real::own") {
		t.Errorf("GetSymbol(%s, element id) = %+v; want the library's data type", libraryID, library)
	}
}
