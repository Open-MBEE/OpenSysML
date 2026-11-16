package lsp

import (
	"strings"
	"testing"

	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const hoverMembersSrc = `package P {
    part def Base {
        attribute id : ScalarValues::String;
        attribute x;
        private attribute hidden;
        protected part sub[2];
    }
    part def Record :> Base {
        doc /* A record. */
        attribute name : ScalarValues::String;
        part items : Base[0..*];
        in port p : ~PD;
        attribute :>> x = 1;
        ref r2 : Base[1..n] ordered;
        attribute n = 3;
        part : Base[4];
        part def Nested;
        enum def Color { red; green; }
        comment c /* note */
        private attribute 'a secret';
    }
    port def PD { in attribute a; out attribute b; }
    part r : Record { attribute extra; }
    part c { port pp : ~PD; }
    alias RR for Record;
    part viaAlias : RR;
    part def Empty;
}
`

// hoverMembersServer opens hoverMembersSrc on a Markdown server given settings.
func hoverMembersServer(t *testing.T, settings map[string]any) (*Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	if settings != nil {
		changeConfiguration(t, s, map[string]any{"sysml": settings})
	}
	name := uri.File("/tmp/members.sysml").FsPath()
	ws.Open(name, []byte(hoverMembersSrc), 1)
	return s, name
}

func hoverMembersAt(t *testing.T, s *Server, name, at string) string {
	t.Helper()
	off := strings.Index(hoverMembersSrc, at)
	if off < 0 {
		t.Fatalf("no %q in the source", at)
	}
	return hoverMarkup(t, hoverInSrc(t, s, name, hoverMembersSrc, off)).Value
}

const recordOwnedMembers = "Members:\n\n```sysml\n" +
	"attribute name : ScalarValues::String\n" +
	"part items : Base[0..*]\n" +
	"in port p : ~PD\n" +
	"attribute x\n" +
	"attribute r2 : Base[1..n]\n" +
	"attribute n\n" +
	"part def Nested\n" +
	"enum def Color\n" +
	"private attribute 'a secret'\n```"

// A definition's hover lists each member it owns after its documentation, by
// kind, name, type and multiplicity; a reference to it shows the same.
func TestHoverSummarizesOwnedMembers(t *testing.T) {
	s, name := hoverMembersServer(t, nil)
	want := "```sysml\npart def Record\n```\n\nA record.\n\n" + recordOwnedMembers
	for _, at := range []string{"Record :> Base", "Record { attribute extra", "RR;"} {
		if got := hoverMembersAt(t, s, name, at); got != want {
			t.Errorf("hover at %q:\n%s\nwant:\n%s", at, got, want)
		}
	}
}

func TestHoverMemberSummaryCases(t *testing.T) {
	s, name := hoverMembersServer(t, nil)
	for _, tc := range []struct{ at, want string }{
		{"Empty;", "```sysml\npart def Empty\n```"},
		{"Color {", "```sysml\nenum def Color\n```\n\nMembers:\n\n```sysml\nenum red\nenum green\n```"},
		{"PD {", "```sysml\nport def PD\n```\n\nMembers:\n\n```sysml\nin attribute a\nout attribute b\n```"},
		{"r : Record", "```sysml\npart r\n```\n\nMembers:\n\n```sysml\nattribute extra\n```"},
		{"P {", "```sysml\npackage P\n```"},
		{"r2 :", "```sysml\nattribute r2\n```"},
	} {
		if got := hoverMembersAt(t, s, name, tc.at); got != tc.want {
			t.Errorf("hover at %q:\n%s\nwant:\n%s", tc.at, got, tc.want)
		}
	}
}

// The limit caps the listed members and says how many it left out; 0 leaves
// the summary out altogether.
func TestHoverMemberLimit(t *testing.T) {
	s, name := hoverMembersServer(t, map[string]any{"hoverMemberLimit": 2})
	want := "```sysml\npart def Record\n```\n\nA record.\n\nMembers:\n\n```sysml\n" +
		"attribute name : ScalarValues::String\npart items : Base[0..*]\n```\n\n… 7 more members"
	if got := hoverMembersAt(t, s, name, "Record :> Base"); got != want {
		t.Errorf("limit 2:\n%s\nwant:\n%s", got, want)
	}
	changeConfiguration(t, s, map[string]any{"sysml": map[string]any{"hoverMemberLimit": 8}})
	if got := hoverMembersAt(t, s, name, "Record :> Base"); !strings.HasSuffix(got, "enum def Color\n```\n\n… 1 more member") {
		t.Errorf("limit 8:\n%s", got)
	}
	changeConfiguration(t, s, map[string]any{"sysml": map[string]any{"hoverMemberLimit": 9}})
	if got := hoverMembersAt(t, s, name, "Record :> Base"); !strings.HasSuffix(got, recordOwnedMembers) {
		t.Errorf("limit 9, the member count:\n%s", got)
	}
	changeConfiguration(t, s, map[string]any{"sysml": map[string]any{"hoverMemberLimit": 0}})
	if got, want := hoverMembersAt(t, s, name, "Record :> Base"), "```sysml\npart def Record\n```\n\nA record."; got != want {
		t.Errorf("limit 0:\n%s\nwant:\n%s", got, want)
	}
}

// Inherited members follow the owned ones under the type each comes from: a
// redefined one and a private one are not inherited, a protected one is.
func TestHoverListsInheritedMembersByOrigin(t *testing.T) {
	s, name := hoverMembersServer(t, map[string]any{"hoverInheritedMembers": true, "hoverMemberLimit": 11})
	want := "```sysml\npart def Record\n```\n\nA record.\n\n" + recordOwnedMembers +
		"\n\nInherited from `P::Base`:\n\n```sysml\nattribute id : ScalarValues::String\nprotected part sub[2]\n```"
	got := hoverMembersAt(t, s, name, "Record :> Base")
	rest, ok := strings.CutPrefix(got, want)
	if !ok {
		t.Fatalf("inherited members:\n%s\nwant prefix:\n%s", got, want)
	}
	if !strings.HasPrefix(rest, "\n\n… ") || strings.Contains(rest, "```") {
		t.Errorf("after the limit: %q, want only the count of the library members left out", rest)
	}

	// A usage inherits its definition's members, and a conjugated port sees
	// its definition's directions reversed.
	got = hoverMembersAt(t, s, name, "r : Record")
	if !strings.HasPrefix(got, "```sysml\npart r\n```\n\nMembers:\n\n```sysml\nattribute extra\n```\n\nInherited from `P::Record`:\n\n```sysml\nattribute name : ScalarValues::String\n") {
		t.Errorf("usage with inherited members:\n%s", got)
	}
	got = hoverMembersAt(t, s, name, "pp : ~PD")
	if !strings.HasPrefix(got, "```sysml\nport pp\n```\n\nInherited from `P::PD`:\n\n```sysml\nout attribute a\nin attribute b\n```") {
		t.Errorf("conjugated port:\n%s", got)
	}
}

// A client without Markdown gets the same summary as plain text.
func TestHoverMemberSummaryAsPlainText(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	changeConfiguration(t, s, map[string]any{"hoverInheritedMembers": true, "hoverMemberLimit": 10})
	name := uri.File("/tmp/members.sysml").FsPath()
	ws.Open(name, []byte(hoverMembersSrc), 1)
	want := "part def Record\n\nA record.\n\nMembers:\n" +
		"attribute name : ScalarValues::String\npart items : Base[0..*]\nin port p : ~PD\nattribute x\n" +
		"attribute r2 : Base[1..n]\nattribute n\npart def Nested\nenum def Color\nprivate attribute 'a secret'\n\n" +
		"Inherited from P::Base:\nattribute id : ScalarValues::String\n\n… "
	if got := hoverMembersAt(t, s, name, "Record :> Base"); !strings.HasPrefix(got, want) {
		t.Errorf("plain hover:\n%s\nwant prefix:\n%s", got, want)
	}
}

func TestHoverMemberSettings(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	if s.hoverMemberLimit != defaultHoverMemberLimit || s.hoverInheritedMembers {
		t.Fatalf("defaults: limit %d, inherited %v", s.hoverMemberLimit, s.hoverInheritedMembers)
	}
	for _, tc := range []struct {
		name          string
		settings      map[string]any
		wantLimit     int
		wantInherited bool
	}{
		{"flat keys", map[string]any{"hoverMemberLimit": 3, "hoverInheritedMembers": true}, 3, true},
		{"dotted keys", map[string]any{"sysml.hoverMemberLimit": 4, "sysml.hoverInheritedMembers": false}, 4, false},
		{"nested section", map[string]any{"sysml": map[string]any{"hoverMemberLimit": 5, "hoverInheritedMembers": true}}, 5, true},
		{"unrelated payload", map[string]any{"sysml": map[string]any{"keepLineBreaks": true}}, 5, true},
		{"negative limit", map[string]any{"hoverMemberLimit": -1}, 5, true},
		{"fractional limit", map[string]any{"hoverMemberLimit": 2.5}, 5, true},
		{"mistyped settings", map[string]any{"hoverMemberLimit": "7", "hoverInheritedMembers": 1}, 5, true},
		{"zero limit", map[string]any{"hoverMemberLimit": 0}, 0, true},
	} {
		changeConfiguration(t, s, tc.settings)
		if s.hoverMemberLimit != tc.wantLimit || s.hoverInheritedMembers != tc.wantInherited {
			t.Errorf("%s: limit %d, inherited %v; want %d, %v", tc.name, s.hoverMemberLimit, s.hoverInheritedMembers, tc.wantLimit, tc.wantInherited)
		}
	}
}

// A notebook cell's hover summarizes members as a file's does: on a definition in its
// own cell and on a reference from a later one, inherited members from an earlier one.
func TestNotebookCellHoverSummarizesMembers(t *testing.T) {
	nb := openCells(t, []cellSpec{
		{"sysml", "package Lib { part def Base { attribute id[1]; } }\n"},
		{"markdown", ""},
		{"sysml", "part def Record :> Lib::Base {\n    doc /* A record. */\n    attribute name;\n    part items : Lib::Base[0..*];\n    attribute extra;\n}\n"},
		{"sysml", "%help\npart r : Record;\n"},
	})
	for _, cell := range []int{0, 2, 3} {
		nb.awaitDiagnostics(nb.cells[cell])
	}
	hoverText := func(cell, line, character int) string {
		t.Helper()
		hover, _ := nb.callOn("textDocument/hover", cell, line, character).(map[string]any)
		contents, _ := hover["contents"].(map[string]any)
		text, _ := contents["value"].(string)
		return text
	}
	owned := "part def Record\n\nA record.\n\nMembers:\nattribute name\npart items : Lib::Base[0..*]\nattribute extra"
	if got := hoverText(2, 0, 10); got != owned {
		t.Errorf("hover on the definition cell:\n%s\nwant:\n%s", got, owned)
	}
	if got := hoverText(3, 1, 10); got != owned {
		t.Errorf("hover on the reference in a later cell:\n%s\nwant:\n%s", got, owned)
	}

	nb.notify("workspace/didChangeConfiguration", `{"settings": {"sysml": {"hoverMemberLimit": 2}}}`)
	limited := "part def Record\n\nA record.\n\nMembers:\nattribute name\npart items : Lib::Base[0..*]\n\n… 1 more member"
	if got := hoverText(3, 1, 10); got != limited {
		t.Errorf("limited hover in a cell:\n%s\nwant:\n%s", got, limited)
	}

	nb.notify("workspace/didChangeConfiguration", `{"settings": {"sysml": {"hoverMemberLimit": 4, "hoverInheritedMembers": true}}}`)
	inherited := owned + "\n\nInherited from Lib::Base:\nattribute id[1]\n\n… "
	if got := hoverText(3, 1, 10); !strings.HasPrefix(got, inherited) {
		t.Errorf("inherited members in a cell:\n%s\nwant prefix:\n%s", got, inherited)
	}
}
