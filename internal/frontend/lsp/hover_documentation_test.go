package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// The model of the hover report: every doc is owned by the element it is
// written in, and must not surface on the member declared after it.
const ownedDocumentationSrc = `package HoverRepro {
    attribute def MatingInterface {
        doc /* DOC_FOR_MATING_INTERFACE */
        attribute interfaceName {
            doc /* DOC_FOR_INTERFACE_NAME */
        }
    }
    attribute def ConnectorFamily {
        doc /* DOC_FOR_CONNECTOR_FAMILY */
        attribute familyName {
            doc /* DOC_FOR_FAMILY_NAME */
        }
    }
    attribute sample : ConnectorFamily;
}
`

// hoverProse returns what a Markdown hover shows after the signature block.
func hoverProse(t *testing.T, res *protocol.Hover) string {
	t.Helper()
	value := hoverMarkup(t, res).Value
	_, prose, _ := strings.Cut(value, "\n```")
	return strings.TrimSpace(prose)
}

func TestHoverShowsTheHoveredElementsOwnedDocumentation(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/owned.sysml").FsPath()
	src := ownedDocumentationSrc
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct{ at, want string }{
		{"MatingInterface {", "DOC_FOR_MATING_INTERFACE"},
		{"interfaceName {", "DOC_FOR_INTERFACE_NAME"},
		{"ConnectorFamily {", "DOC_FOR_CONNECTOR_FAMILY"},
		{"familyName {", "DOC_FOR_FAMILY_NAME"},
		{"ConnectorFamily;", "DOC_FOR_CONNECTOR_FAMILY"},
		{"sample", ""},
	} {
		res := hoverInSrc(t, s, name, src, strings.Index(src, tc.at))
		if got := hoverProse(t, res); got != tc.want {
			t.Errorf("hover at %q shows %q, want %q", tc.at, got, tc.want)
		}
	}
}

// A `comment` or `rep` body belongs to its annotation and is neither the next
// member's documentation nor the owner's; a named `comment about X` is no different.
func TestHoverDoesNotShowCommentOrRepBodies(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/bodies.sysml").FsPath()
	src := "package P {\n    part def Car {\n        doc /* A car. */\n        comment /* OWNED_COMMENT */\n        comment c about mass /* NAMED_COMMENT */\n        rep inOcl language \"ocl\" /* REP_BODY */\n        attribute mass;\n    }\n    part def Bus;\n}\n"
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct{ at, want string }{
		{"Car {", "A car."},
		{"mass;", ""},
		{"Bus;", ""},
	} {
		res := hoverInSrc(t, s, name, src, strings.Index(src, tc.at))
		if got := hoverProse(t, res); got != tc.want {
			t.Errorf("hover at %q shows %q, want %q", tc.at, got, tc.want)
		}
	}
}

// The notes written directly above a declaration follow its owned documentation.
func TestHoverShowsNotesAfterOwnedDocumentation(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/notes.sysml").FsPath()
	src := "package P {\n    // a line note\n    //* a block note */\n    part def Wheel {\n        doc /* A wheel. */\n    }\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel {"))
	if got, want := hoverProse(t, res), "A wheel.\n\na line note\n\na block note"; got != want {
		t.Errorf("hover shows %q, want %q", got, want)
	}
}

// A bare `/* */` above a declaration is a Comment owned by, and about, the
// enclosing namespace: it is not what documents the declaration after it.
func TestHoverOmitsTheBareCommentAboveADeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/bare.sysml").FsPath()
	src := "package P {\n    /* about P */\n    part def Wheel;\n    // a note\n    part def Axle;\n}\n"
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct{ at, want string }{
		{"Wheel;", ""},
		{"Axle;", "a note"},
	} {
		res := hoverInSrc(t, s, name, src, strings.Index(src, tc.at))
		if got := hoverProse(t, res); got != tc.want {
			t.Errorf("hover at %q shows %q, want %q", tc.at, got, tc.want)
		}
	}
}

// Without Markdown the owned documentation is plain prose, the notes as written.
func TestHoverPlainTextShowsOwnedDocumentationThenNotes(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/plain.sysml").FsPath()
	src := "package P {\n    // a note\n    part def Wheel {\n        doc /* A wheel. */\n    }\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel {"))
	if got, want := hoverMarkup(t, res).Value, "part def Wheel\n\nA wheel.\n// a note"; got != want {
		t.Errorf("hover = %q, want %q", got, want)
	}
}

// A standard-library element's hover, in its bundled document and from a reference,
// shows its own doc: `Anything` is the top-level type, not what Base's doc says.
func TestHoverShowsAStandardLibraryElementsOwnDocumentation(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	const file = "Kernel Libraries/Kernel Semantic Library/Base.kerml"
	doc := ws.LibraryDocument(file)
	if doc == nil {
		t.Fatalf("library document %q not bundled", file)
	}
	lib := string(doc.Content)
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: libraryURI(file)},
			Position:     offsetToPosition(doc.Content, strings.Index(lib, "classifier Anything")+len("classifier ")),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	wantAnything := "Anything is the top level generalized type in the language."
	if got := hoverProse(t, res); !strings.Contains(got, wantAnything) || strings.Contains(got, "Base") {
		t.Errorf("hover on the Anything declaration shows %q, want its own doc and not the package's", got)
	}

	name := uri.File("/tmp/uses-anything.sysml").FsPath()
	src := "package P { classifier Thing specializes Base::Anything; }"
	ws.Open(name, []byte(src), 1)
	res = hoverInSrc(t, s, name, src, strings.Index(src, "Anything"))
	if got := hoverProse(t, res); !strings.Contains(got, wantAnything) || strings.Contains(got, "Base is") {
		t.Errorf("hover on a reference to Anything shows %q, want its own doc", got)
	}
}
