package grpc

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/objref"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestSymbolToProto verifies exported SymbolToProtoIn API.
func TestSymbolToProto(t *testing.T) {
	sym := &symbols.Symbol{
		Name:       "TestPart",
		Kind:       symbols.SymbolPartDef,
		Visibility: ast.VisibilityPublic,
		DeclSpan:   source.Span{Offset: 10, Len: 10},
	}

	idx := symbols.NewIndex()

	proto := SymbolToProtoIn(sym, NewSymbolContext(idx))

	if proto.Name != "TestPart" {
		t.Errorf("expected name TestPart, got %s", proto.Name)
	}
	if proto.Kind != "partDef" {
		t.Errorf("expected kind partDef, got %s", proto.Kind)
	}
	if proto.Id == "" {
		t.Error("expected non-empty ID")
	}
	if proto.Metadata["visibility"] != "public" {
		t.Errorf("expected visibility public, got %s", proto.Metadata["visibility"])
	}
}

// TestDiagnosticToProto verifies DiagnosticToProto conversion.
func TestDiagnosticToProto(t *testing.T) {
	diag := diag.Diagnostic{
		Severity: diag.SeverityError,
		Message:  "test error",
		Span:     source.Span{Offset: 5, Len: 4}, // "Test" at position 5
	}

	sf := source.New("test.sysml", []byte("part Test { }"))
	proto := DiagnosticToProto(diag, sf)

	if proto.Severity != "error" {
		t.Errorf("expected severity error, got %s", proto.Severity)
	}
	if proto.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", proto.Message)
	}
	if proto.Span.File != "test.sysml" {
		t.Error("expected file test.sysml")
	}
	if proto.Span.StartLine != 1 {
		t.Errorf("expected StartLine 1, got %d", proto.Span.StartLine)
	}
}

// TestConvertSpan verifies source.Span → proto.Span conversion.
func TestConvertSpan(t *testing.T) {
	// Create a simple source file
	content := []byte("line1\nline2\nline3")
	sf := source.New("test.sysml", content)

	// Span covering "line2" (bytes 6-11)
	sp := source.Span{Offset: 6, Len: 5}

	pb := DiagnosticToProto(diag.Diagnostic{Span: sp}, sf).Span
	if pb.File != "test.sysml" {
		t.Errorf("File: got %q, want %q", pb.File, "test.sysml")
	}
	if pb.StartLine != 2 {
		t.Errorf("StartLine: got %d, want 2", pb.StartLine)
	}
	if pb.EndLine != 2 {
		t.Errorf("EndLine: got %d, want 2", pb.EndLine)
	}
	// Columns are 1-based byte columns
	if pb.StartCol != 1 {
		t.Errorf("StartCol: got %d, want 1", pb.StartCol)
	}
	if pb.EndCol != 6 {
		t.Errorf("EndCol: got %d, want 6", pb.EndCol)
	}
}

// TestConvertSymbolBasic verifies Symbol → SymbolInfo conversion for a simple part def.
func TestConvertSymbolBasic(t *testing.T) {
	// Parse real model to get symbol with proper structure
	content := `package Test { part def MyPart; }`
	sf := source.New("test.sysml", []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()

	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)

	// Lookup MyPart symbol
	syms := idx.LookupQualified("Test::MyPart")
	if len(syms) == 0 {
		t.Fatal("MyPart symbol not found")
	}

	pbSym := SymbolToProtoIn(syms[0], NewSymbolContext(idx))
	if pbSym.Id != "Test::MyPart" {
		t.Errorf("Id: got %q, want %q", pbSym.Id, "Test::MyPart")
	}
	if pbSym.Name != "MyPart" {
		t.Errorf("Name: got %q, want %q", pbSym.Name, "MyPart")
	}
	if pbSym.Kind != "partDef" {
		t.Errorf("Kind: got %q, want %q", pbSym.Kind, "partDef")
	}
}

// TestConvertSymbolWithChildren verifies child_ids population with FQNs.
func TestConvertSymbolWithChildren(t *testing.T) {
	// Parse model with nested symbols
	content := `package Parent { part def Child1; attribute def Child2; }`
	sf := source.New("test.sysml", []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()

	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)

	// Lookup Parent symbol
	parents := idx.LookupQualified("Parent")
	if len(parents) == 0 {
		t.Fatal("Parent symbol not found")
	}

	pbSym := SymbolToProtoIn(parents[0], NewSymbolContext(idx))
	if len(pbSym.ChildIds) != 2 {
		t.Fatalf("ChildIds: got %d, want 2", len(pbSym.ChildIds))
	}

	// Verify FQNs are present
	found := map[string]bool{}
	for _, id := range pbSym.ChildIds {
		found[id] = true
	}
	if !found["Parent::Child1"] {
		t.Errorf("Missing Parent::Child1 in ChildIds, got: %v", pbSym.ChildIds)
	}
	if !found["Parent::Child2"] {
		t.Errorf("Missing Parent::Child2 in ChildIds, got: %v", pbSym.ChildIds)
	}
}

// TestConvertSymbolMetadata verifies metadata extraction from Usage node.
func TestConvertSymbolMetadata(t *testing.T) {
	// Parse model with typed attribute
	content := `package MyPart { attribute mass : Real [1]; }`
	sf := source.New("test.sysml", []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()

	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)

	// Lookup mass symbol
	syms := idx.LookupQualified("MyPart::mass")
	if len(syms) == 0 {
		t.Fatal("mass symbol not found")
	}

	pbSym := SymbolToProtoIn(syms[0], NewSymbolContext(idx))

	// Check metadata (parser produces "1" for single value, not "1..1")
	if pbSym.Metadata["multiplicity"] != "1" {
		t.Errorf("Metadata[multiplicity]: got %q, want %q", pbSym.Metadata["multiplicity"], "1")
	}
	if pbSym.Metadata["type"] != "Real" {
		t.Errorf("Metadata[type]: got %q, want %q", pbSym.Metadata["type"], "Real")
	}
}

// TestSymbolIdIsFQN verifies Symbol.Id contains fully-qualified names for nested symbols.
func TestSymbolIdIsFQN(t *testing.T) {
	// Parse nested model
	content := `
package Vehicle {
  part def Engine {
    part combustionChamber;
  }
}
`
	sf := source.New("test.sysml", []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()

	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)

	// Verify Vehicle has correct FQN
	vehicles := idx.LookupQualified("Vehicle")
	if len(vehicles) == 0 {
		t.Fatal("Vehicle symbol not found")
	}
	vehiclePb := SymbolToProtoIn(vehicles[0], NewSymbolContext(idx))
	if vehiclePb.Id != "Vehicle" {
		t.Errorf("Vehicle Id: got %q, want %q", vehiclePb.Id, "Vehicle")
	}

	// Verify Engine has FQN "Vehicle::Engine"
	engines := idx.LookupQualified("Vehicle::Engine")
	if len(engines) == 0 {
		t.Fatal("Vehicle::Engine symbol not found")
	}
	enginePb := SymbolToProtoIn(engines[0], NewSymbolContext(idx))
	if enginePb.Id != "Vehicle::Engine" {
		t.Errorf("Engine Id: got %q, want %q", enginePb.Id, "Vehicle::Engine")
	}
	if enginePb.Name != "Engine" {
		t.Errorf("Engine Name: got %q, want %q", enginePb.Name, "Engine")
	}

	// Verify combustionChamber has FQN "Vehicle::Engine::combustionChamber"
	chambers := idx.LookupQualified("Vehicle::Engine::combustionChamber")
	if len(chambers) == 0 {
		t.Fatal("Vehicle::Engine::combustionChamber symbol not found")
	}
	chamberPb := SymbolToProtoIn(chambers[0], NewSymbolContext(idx))
	if chamberPb.Id != "Vehicle::Engine::combustionChamber" {
		t.Errorf("combustionChamber Id: got %q, want %q", chamberPb.Id, "Vehicle::Engine::combustionChamber")
	}
	if chamberPb.Name != "combustionChamber" {
		t.Errorf("combustionChamber Name: got %q, want %q", chamberPb.Name, "combustionChamber")
	}

	// Verify Engine.ChildIds contains FQN
	if len(enginePb.ChildIds) != 1 {
		t.Fatalf("Engine ChildIds count: got %d, want 1", len(enginePb.ChildIds))
	}
	if enginePb.ChildIds[0] != "Vehicle::Engine::combustionChamber" {
		t.Errorf("Engine ChildIds[0]: got %q, want %q", enginePb.ChildIds[0], "Vehicle::Engine::combustionChamber")
	}
}

// TestCollectionElementsHandlesSetAndSequence verifies a collection feature value is
// marshalled whichever collection kind the runtime left in it.
func TestCollectionElementsHandlesSetAndSequence(t *testing.T) {
	one := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	two := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}}

	seq := runtime.NewSequence()
	seq.Append(one)
	seq.Append(two)

	set := runtime.NewSet()
	set.Add(one)
	set.Add(two)

	for name, val := range map[string]runtime.Value{
		"sequence": runtime.NewSequenceValue(seq),
		"set":      runtime.NewSetValue(set),
	} {
		if got := len(objref.CollectionElements(val)); got != 2 {
			t.Errorf("%s: got %d elements, want 2", name, got)
		}
	}

	if got := objref.CollectionElements(runtime.Value{Kind: runtime.ValNull}); got != nil {
		t.Errorf("non-collection: got %v, want nil", got)
	}
}
