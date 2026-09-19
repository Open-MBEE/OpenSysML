package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const requirementShortNameSrc = `package P {
	part def T;
	constraint def C;
	requirement def R {
		subject <s> x : T;
		assume constraint <a> ac : C;
		require constraint <r> rc : C;
		subject <t> : T;
	}
	requirement def R2 :> R {
		subject :>> s;
		assume constraint :>> a;
		require constraint :>> r;
		subject :>> t;
	}
	part p : R::t;
}`

func openRequirementShortNameDoc(t *testing.T) (*Server, *model.Workspace, string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/requirement_short_names.sysml").Filename()
	ws.Open(name, []byte(requirementShortNameSrc), 1)
	return s, ws, name
}

func requirementShortNamePos(t *testing.T, anchor string, skip int) protocol.Position {
	t.Helper()
	off := strings.Index(requirementShortNameSrc, anchor)
	if off < 0 {
		t.Fatalf("anchor %q not found", anchor)
	}
	return offsetToPosition([]byte(requirementShortNameSrc), off+skip)
}

// The short name of a subject, assume or require member is a name a reference
// jumps from, as `part <p>` is: `:>> s` lands on the subject that declares <s>.
func TestDefinitionRequirementMemberShortName(t *testing.T) {
	s, _, name := openRequirementShortNameDoc(t)
	cases := []struct {
		anchor string
		line   uint32
	}{
		{":>> s;", 4},
		{":>> a;", 5},
		{":>> r;", 6},
		{":>> t;", 7},
		{"R::t;", 7},
	}
	for _, tc := range cases {
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     requirementShortNamePos(t, tc.anchor, len(tc.anchor)-2),
			},
		})
		if err != nil {
			t.Fatalf("%s: Definition err = %v", tc.anchor, err)
		}
		if len(locs) != 1 || locs[0].Range.Start.Line != tc.line {
			t.Errorf("%s: locations = %+v, want one on line %d", tc.anchor, locs, tc.line)
		}
	}
}

// Hovering a reference written with the short name describes the member it
// names; hovering the short name in the declaration describes that member.
func TestHoverRequirementMemberShortName(t *testing.T) {
	s, _, name := openRequirementShortNameDoc(t)
	hover := func(anchor string, skip int) string {
		res, err := s.Hover(context.Background(), &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     requirementShortNamePos(t, anchor, skip),
			},
		})
		if err != nil {
			t.Fatalf("%s: Hover err = %v", anchor, err)
		}
		if res == nil {
			t.Fatalf("%s: no hover", anchor)
		}
		return res.Contents.Value
	}
	for _, tc := range []struct{ anchor, want string }{
		{":>> s;", "subject x"},
		{":>> a;", "assume constraint ac"},
		{":>> r;", "require constraint rc"},
		{":>> t;", "subject t"},
	} {
		if got := hover(tc.anchor, len(tc.anchor)-2); !strings.Contains(got, tc.want) {
			t.Errorf("hover at %s = %q, want %q", tc.anchor, got, tc.want)
		}
	}
	if got := hover("<s> x", 1); !strings.Contains(got, "subject x") {
		t.Errorf("hover on <s> = %q, want the subject x", got)
	}
	if got := hover("<t> :", 1); !strings.Contains(got, "subject t") {
		t.Errorf("hover on <t> = %q, want the subject t", got)
	}
}

// Document symbols list a member under its name, or under its short name when
// that is the only name it declares.
func TestDocumentSymbolRequirementMemberShortNames(t *testing.T) {
	s, _, name := openRequirementShortNameDoc(t)
	res, err := s.DocumentSymbol(context.Background(), &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("DocumentSymbol err = %v", err)
	}
	pkg := res[0].(protocol.DocumentSymbol)
	var names []string
	for _, child := range pkg.Children {
		if child.Name != "R" {
			continue
		}
		for _, member := range child.Children {
			names = append(names, member.Name)
		}
	}
	if got := strings.Join(names, " "); got != "x ac rc t" {
		t.Errorf("R members = %q, want %q", got, "x ac rc t")
	}
}

// Member completion offers the short names of a requirement's subject, assume
// and require members beside their names.
func TestCompletionOffersRequirementMemberShortNames(t *testing.T) {
	src := strings.Replace(requirementShortNameSrc, "part p : R::t;", "part p : R::t;\n\tpart q : R::", 1)
	items := completionAt(t, src, "part q : R::")
	for _, label := range []string{"s", "x", "a", "ac", "r", "rc", "t"} {
		if _, ok := items[label]; !ok {
			t.Errorf("completion of R:: missing %q; got %v", label, labelsOf(items))
		}
	}
}

// Renaming from the declaration of a short-name-only subject renames the short
// name and every reference to it, as for `part <p>;`. Renaming a subject's name
// leaves references written with its short name alone, as for `part <p> x`, and
// renaming from such a reference renames the short name instead.
func TestRenameRequirementMemberShortName(t *testing.T) {
	_, ws, name := openRequirementShortNameDoc(t)
	got, err := applyRename(t, ws, name, "t> : T", "target")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := strings.NewReplacer("<t> : T", "<target> : T", ":>> t;", ":>> target;", "R::t;", "R::target;").
		Replace(requirementShortNameSrc)
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}

	_, ws, name = openRequirementShortNameDoc(t)
	got, err = applyRename(t, ws, name, "x : T", "vehicle")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want = strings.NewReplacer("<s> x : T", "<s> vehicle : T").Replace(requirementShortNameSrc)
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}

	_, ws, name = openRequirementShortNameDoc(t)
	got, err = applyRename(t, ws, name, "s;", "veh")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want = strings.NewReplacer("<s> x : T", "<veh> x : T", ":>> s;", ":>> veh;").Replace(requirementShortNameSrc)
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}
