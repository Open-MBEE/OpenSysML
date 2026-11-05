package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func commentBodies(t *testing.T, src string, members []ast.Node) []string {
	t.Helper()
	var bodies []string
	for _, m := range members {
		if c, ok := memberNode(m).(*ast.Comment); ok {
			bodies = append(bodies, src[c.BodySpan.Offset:c.BodySpan.End()])
		}
	}
	return bodies
}

// A /* */ written where a member may appear is an anonymous Comment member of
// the enclosing namespace (KerML 1.1 §8.2.3.3.2: `Comment = ('comment'
// Identification ('about' …)?)? ('locale' …)? body = REGULAR_COMMENT`), before
// the first member, between members, before the closing brace and at the ends
// of the file alike; one inside a declaration head stays trivia.
func TestBareCommentsAtMemberPositionsAreCommentMembers(t *testing.T) {
	src := "/* head */\npackage P {\n\t/* first */\n\tpart /* in head */ def V {\n\t\tpart x;\n\t\t/* last */\n\t}\n\t/* between */\n\tpart y : V;\n}\n/* eof */\n"
	p := New(source.New("t.sysml", []byte(src)))
	root := p.ParseFile()
	// The one comment inside a declaration head is where the grammar admits none.
	if len(p.Diagnostics) != 0 || len(p.Warnings) != 1 {
		t.Fatalf("diagnostics %v warnings %v, want one misplaced-comment warning", p.Diagnostics, p.Warnings)
	}
	if got := commentBodies(t, src, root.Members); len(got) != 2 || got[0] != "/* head */" || got[1] != "/* eof */" {
		t.Errorf("root comments = %q, want the head and eof comments", got)
	}
	pkg := root.Members[1].(*ast.Membership).Member.(*ast.Package)
	if got := commentBodies(t, src, pkg.Members); len(got) != 2 || got[0] != "/* first */" || got[1] != "/* between */" {
		t.Errorf("package comments = %q, want first and between", got)
	}
	def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
	if got := commentBodies(t, src, def.Members); len(got) != 1 || got[0] != "/* last */" {
		t.Errorf("definition comments = %q, want the one before its closing brace", got)
	}
	var headTrivia int
	for _, tr := range pkg.Members[1].LeadingTrivia() {
		if tr.Kind == ast.TriviaComment {
			headTrivia++
		}
	}
	if headTrivia != 0 {
		t.Errorf("the definition carries %d comment trivia; the head comment is read by the next member", headTrivia)
	}
}

// The body of `doc` or `comment` is the comment written right after its head;
// a second comment before the next member is a member of its own, not the body.
func TestCommentBodyIsTheFirstCommentAfterTheHead(t *testing.T) {
	for _, c := range []struct{ src, body, member string }{
		{"package P {\n\tdoc /* d */\n\t/* bare */\n\tpart x;\n}\n", "/* d */", "/* bare */"},
		{"package P {\n\tcomment /* c */\n\t/* bare */\n\tpart x;\n}\n", "/* c */", "/* bare */"},
		{"package P {\n\tcomment C /* c */ /* bare */ part x;\n}\n", "/* c */", "/* bare */"},
	} {
		p := New(source.New("t.sysml", []byte(c.src)))
		root := p.ParseFile()
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		var body string
		switch d := pkg.Members[0].(*ast.Membership).Member.(type) {
		case *ast.Documentation:
			body = c.src[d.BodySpan.Offset:d.BodySpan.End()]
		case *ast.Comment:
			body = c.src[d.BodySpan.Offset:d.BodySpan.End()]
		}
		if body != c.body {
			t.Errorf("%q: body %q, want %q", c.src, body, c.body)
		}
		if got := commentBodies(t, c.src, pkg.Members[1:]); len(got) != 1 || got[0] != c.member {
			t.Errorf("%q: further comment members %q, want %q", c.src, got, c.member)
		}
	}
}

// A comment before a member that continues the one before it (`then x;`) opens
// no member position and stays trivia; an unterminated bare comment is reported
// without a panic.
func TestBareCommentsOutsideMemberPositionsStayTrivia(t *testing.T) {
	src := "action def A {\n\taction a;\n\t/* before then */\n\tthen done;\n}\n"
	p := New(source.New("t.sysml", []byte(src)))
	root := p.ParseFile()
	def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if got := commentBodies(t, src, def.Members); len(got) != 0 {
		t.Errorf("comments = %q, want none: the comment precedes an attached member", got)
	}
	if len(p.Warnings) != 1 {
		t.Errorf("warnings = %v, want the misplaced-comment warning", p.Warnings)
	}
	p = New(source.New("t.sysml", []byte("package P {\n\t/* open\n\tpart x;\n}\n")))
	p.ParseFile()
	if len(p.Diagnostics) == 0 {
		t.Error("an unterminated comment is not reported")
	}
}
