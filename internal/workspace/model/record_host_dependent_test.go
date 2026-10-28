package model

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup/markuptest"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const proseWithAsciiDocBody = `package Prose {
	private import DocumentQueries::*;
	part def Report :> Document {
		attribute redefines title = "Report";
		part note : Paragraph {
			rep language "asciidoc" /* Mind the *gap*. */
		}
	}
}`

// A document with a body an external converter checks is not recorded,
// whether the converter was missing — the warning is this host's — or found
// and the body checked: the verdict would not hold on a host without it.
func TestInterfaceRecordRefusesExternallyCheckedBodies(t *testing.T) {
	t.Run("converter missing", func(t *testing.T) {
		t.Setenv(markup.AsciiDocEnv, filepath.Join(t.TempDir(), "missing-converter"))
		ws := NewWorkspace()
		ws.Open("prose.sysml", []byte(proseWithAsciiDocBody), 1)
		var warned bool
		for _, d := range ws.Diagnostics("prose.sysml") {
			warned = warned || d.Code == markupToolMissingCode
		}
		if !warned {
			t.Fatal("the document reports no missing converter: the fixture is vacuous")
		}
		if _, err := ws.InterfaceRecord("prose.sysml"); !errors.Is(err, libs.ErrUnrecordable) {
			t.Fatalf("InterfaceRecord = %v, want %v", err, libs.ErrUnrecordable)
		}
	})
	t.Run("converter found", func(t *testing.T) {
		converter, err := markuptest.RecordedConverter(t.TempDir(), "../../ir/docplan/markup/testdata")
		if err != nil {
			t.Skip(err)
		}
		t.Setenv(markup.AsciiDocEnv, converter)
		// The recorded converter answers the bodies of this fixture.
		content, err := os.ReadFile("../../doc/docrender/testdata/asciidoc_report.sysml")
		if err != nil {
			t.Fatal(err)
		}
		ws := NewWorkspace()
		ws.Open("prose.sysml", content, 1)
		if diags := ws.Diagnostics("prose.sysml"); len(diags) != 0 {
			t.Fatalf("the checked document reports %v, want nothing", diags)
		}
		if _, err := ws.InterfaceRecord("prose.sysml"); !errors.Is(err, libs.ErrUnrecordable) {
			t.Fatalf("InterfaceRecord = %v, want %v", err, libs.ErrUnrecordable)
		}
	})
}

// A body no document plan reads — one on an ordinary part — leaves the
// document recordable, converter or no converter.
func TestInterfaceRecordKeepsBodiesOutsideDocuments(t *testing.T) {
	t.Setenv(markup.AsciiDocEnv, filepath.Join(t.TempDir(), "missing-converter"))
	ws := NewWorkspace()
	ws.Open("engine.sysml", []byte(`package Model { part engine { rep language "asciidoc" /* engine notes */ } }`), 1)
	if diags := ws.Diagnostics("engine.sysml"); len(diags) != 0 {
		t.Fatalf("the fixture reports %v, want nothing", diags)
	}
	if _, err := ws.InterfaceRecord("engine.sysml"); err != nil {
		t.Fatalf("InterfaceRecord = %v, want the record", err)
	}
}

// A document declaring a document query definition is not recorded: a plan
// in another document evaluates the query's result expression from the tree.
func TestInterfaceRecordRefusesQueryDefinitions(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("queries.sysml", []byte(`package Queries {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	calc def Names :> Query {
		in root : Element;
		OwnedElements(source = root)
	}
}`), 1)
	if diags := ws.Diagnostics("queries.sysml"); len(diags) != 0 {
		t.Fatalf("the fixture reports %v, want nothing", diags)
	}
	if _, err := ws.InterfaceRecord("queries.sysml"); !errors.Is(err, libs.ErrUnrecordable) {
		t.Fatalf("InterfaceRecord = %v, want %v", err, libs.ErrUnrecordable)
	}
}
