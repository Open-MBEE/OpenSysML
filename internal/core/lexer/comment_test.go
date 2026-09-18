package lexer

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestCommentBody(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"block", "/* The mission shall return the crew. */", "The mission shall return the crew."},
		{"note", "//* a note */", "a note"},
		{"line", "// a line", "a line"},
		{"empty", "/* */", ""},
		{"starred_lines", "/*\n\t * first line\n\t * second line\n\t */", "first line\nsecond line"},
		{"indented_lines", "/* first\n\t\t   second\n\t*/", "first\nsecond"},
		{"doc_opener", "/** The mission shall return the crew. */", "The mission shall return the crew."},
		{"doc_opener_empty", "/***/", ""},
		{"doc_opener_starred_lines", "/**\n * first line\n * second line\n */", "first line\nsecond line"},
		{"doc_opener_first_line_text", "/** first\n * second\n */", "first\nsecond"},
		{"doc_opener_emphasis_after_margin", "/**\n * *important* note\n */", "*important* note"},
		{"doc_opener_tab", "/**\tnote */", "note"},
		{"glued_star_is_text", "/**bold* start */", "*bold* start"},
		{"note_doc_opener", "//** a note */", "a note"},
		{"emphasis_kept", "/* **bold** start */", "**bold** start"},
		{"single_emphasis_kept", "/* *important* note */", "*important* note"},
		{"single_emphasis_after_margin", "/*\n * *important* note\n */", "*important* note"},
		{"strong_after_margin", "/*\n * **bold** start\n * more\n */", "**bold** start\nmore"},
		{"bullets_kept_without_margin", "/* Items:\n\t* one\n\t* two\n\tDone.\n\t*/", "Items:\n* one\n* two\nDone."},
		{"uniform_stars_are_margin", "/* Items:\n * one\n * two\n */", "Items:\none\ntwo"},
		{"bullets_kept_behind_margin", "/*\n * Items:\n * * one\n * * two\n */", "Items:\n* one\n* two"},
		{"partial_margin_is_text", "/* first\n * second\n third\n */", "first\n* second\nthird"},
		{"emphasis_line_without_margin", "/* first\n *second*\n */", "first\n*second*"},
		{"blank_margin_line", "/*\n * one\n *\n * two\n */", "one\n\ntwo"},
		{"line_comment_lines", "// one\n// two", "one\ntwo"},
		{"line_comment_bullets", "// * one\n// * two", "* one\n* two"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := source.CommentBody(tc.raw); got != tc.want {
				t.Errorf("source.CommentBody(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
