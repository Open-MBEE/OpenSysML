package main

import (
	"strings"
	"testing"
)

// The extractor is checked against hand-written grammar snippets, so the real
// grammars' figures rest on verified extraction rather than on inspection.
func TestParseGrammarProductions(t *testing.T) {
	const src = `
grammar org.example.Toy with org.example.Base hidden(WS, SL_COMMENT)
import "http://www.eclipse.org/emf/2002/Ecore" as ecore

Package returns SysML::Package :
	'package' Identification '{' Body '}'
;

@Override
fragment Identification returns SysML::Element :
	( '<' declaredShortName = Name '>' )? ( declaredName = Name )?
;

enum VisibilityKind returns SysML::VisibilityKind :
	public = 'public' | private = 'private'
;

terminal Name :
	('a'..'z' | 'A'..'Z')+
;
`
	grammar, err := ParseGrammar("Toy.xtext", src)
	if err != nil {
		t.Fatalf("ParseGrammar: %v", err)
	}
	if grammar.Declared != "org.example.Toy" || grammar.Extends != "org.example.Base" {
		t.Fatalf("grammar declaration: %q with %q", grammar.Declared, grammar.Extends)
	}
	want := []struct {
		name     string
		kind     Kind
		line     int
		returns  string
		override bool
		literals string
	}{
		{"Package", KindRule, 5, "SysML::Package", false, `"package" "{" "}"`},
		{"Identification", KindFragment, 10, "SysML::Element", true, `"<" ">"`},
		{"VisibilityKind", KindEnum, 14, "SysML::VisibilityKind", false, `"private" "public"`},
		{"Name", KindTerminal, 18, "", false, ""},
	}
	if len(grammar.Productions) != len(want) {
		t.Fatalf("got %d productions, want %d", len(grammar.Productions), len(want))
	}
	for i, expect := range want {
		got := grammar.Productions[i]
		if got.Grammar != "Toy.xtext" || got.Name != expect.name || got.Kind != expect.kind ||
			got.Line != expect.line || got.Returns != expect.returns || got.Override != expect.override {
			t.Errorf("production %d = %+v", i, got)
		}
		if literals := quoteAll(got.Literals()); expect.literals != "" && literals != expect.literals {
			t.Errorf("%s literals = %s, want %s", got.Name, literals, expect.literals)
		}
	}
	// A terminal's body is character classes, not notation, so it is not parsed.
	if grammar.Productions[3].Body != nil {
		t.Errorf("terminal Name has a parsed body")
	}
}

// Elements that consume no literal of their own must not contribute literals,
// and Xtext's syntactic predicates must not be mistaken for input.
func TestParseGrammarNonLiteralElements(t *testing.T) {
	const src = `
Item returns SysML::Item :
	{SysML::Item} => 'item' ownedMember += Member*
	( 'of' target = [SysML::Type|QualifiedName] )?
;
`
	grammar, err := ParseGrammar("Toy.xtext", src)
	if err != nil {
		t.Fatalf("ParseGrammar: %v", err)
	}
	if got, want := quoteAll(grammar.Productions[0].Literals()), `"item" "of"`; got != want {
		t.Fatalf("literals = %s, want %s", got, want)
	}
}

func TestParseGrammarErrors(t *testing.T) {
	for name, src := range map[string]string{
		"missing colon":     "Item 'item' ;",
		"missing semicolon": "Item : 'item'",
		"unterminated body": "Item : ( 'item' ;",
		"stray operator":    "Item : = 'item' ;",
	} {
		if _, err := ParseGrammar("Toy.xtext", src); err == nil {
			t.Errorf("%s: parsed without error", name)
		} else if !strings.HasPrefix(err.Error(), "Toy.xtext: ") {
			t.Errorf("%s: error lacks the grammar name: %v", name, err)
		}
	}
}
