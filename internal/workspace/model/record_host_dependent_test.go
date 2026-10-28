package model

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A document reported on the host's markup converter is not recorded: the
// verdict would not hold on a host that has the converter, or lacks it.
func TestInterfaceRecordRefusesHostDependentDiagnostics(t *testing.T) {
	t.Setenv(markup.AsciiDocEnv, filepath.Join(t.TempDir(), "missing-converter"))
	content := []byte(`package Prose {
	private import DocumentQueries::*;
	part def Report :> Document {
		attribute redefines title = "Report";
		rep language "asciidoc" /* A body. */
	}
}`)
	ws := NewWorkspace()
	ws.Open("prose.sysml", content, 1)
	var warned bool
	for _, d := range ws.Diagnostics("prose.sysml") {
		warned = warned || d.Code == markupToolMissingCode
	}
	if !warned {
		t.Fatal("the document reports no missing converter: the fixture is vacuous")
	}
	if _, err := ws.InterfaceRecord("prose.sysml"); !errors.Is(err, libs.ErrUnrecordable) {
		t.Fatalf("InterfaceRecord = %v, want %v naming the converter diagnostic", err, libs.ErrUnrecordable)
	}
}
