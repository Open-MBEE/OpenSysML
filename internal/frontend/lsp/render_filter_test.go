package lsp

import (
	"context"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Two copies of a library in the workspace declare each metaclass twice; a view
// filtered by one still draws the elements of that metaclass.
func TestRenderFiltersByAMetaclassDeclaredByTwoDocuments(t *testing.T) {
	s := initializedServer(t, map[string]any{CrossDocumentCapability: true})
	open := func(name, src string) uri.URI {
		docURI := tempDocURI(t, name)
		if err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: docURI, LanguageID: "sysml", Version: 1, Text: src},
		}); err != nil {
			t.Fatalf("DidOpen(%s) err = %v", name, err)
		}
		return docURI
	}
	const metaclassCopy = `package SysML { package Systems { metadata def PartUsage; } }`
	open("lib_one.sysml", metaclassCopy)
	open("lib_two.sysml", metaclassCopy)
	docURI := open("interconnection.sysml", `
		package Model {
			part abcd {
				part a { port a; }
				part b { port b; }
				action d;
				interface i connect (a.a, b.b);
				part c;
				port p;
			}
			private import StandardViewDefinitions::*;
			view filtered : InterconnectionView { expose abcd; filter @PartUsage; }
		}
	`)
	res := render(t, s, docURI, "Model::filtered")
	var names []string
	for _, node := range res.Nodes {
		names = append(names, node.Name)
	}
	slices.Sort(names)
	if want := []string{"Model::abcd", "a", "b", "c"}; !slices.Equal(names, want) {
		t.Fatalf("render(Model::filtered) nodes = %v (notices %v), want %v", names, res.Notices, want)
	}
	for _, notice := range res.Notices {
		if strings.Contains(notice, "rejected by its filter") {
			t.Fatalf("render(Model::filtered) notices = %v, want none claiming the filter rejects everything", res.Notices)
		}
	}
}
