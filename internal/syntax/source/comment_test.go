package source

import (
	"strings"
	"testing"
)

// The cases follow KerML 1.1 §8.2.3.3.2 Note 1; the pinned pilot's
// ElementUtil.processCommentBody agrees with each except blank_first_lines.
func TestCommentBody(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"spaced", "/* hello */", "hello "},
		{"tight", "/*hello*/", "hello"},
		{"empty", "/**/", ""},
		{"blank", "/* */", ""},
		{"opener_line_dropped", "/*\n * a\n * b\n */", "a\nb\n"},
		{"opener_text_kept", "/* a\n * b\n */", "a\nb\n"},
		{"trailing_space_kept", "/* a  \n *   b\n *\n * c*/", "a  \n  b\n\nc"},
		{"doc_opener_star_is_text", "/** x */", "* x "},
		{"one_space_after_margin", "/*\n *   x*/", "  x"},
		{"no_margin", "/* a\n   b */", "a\nb "},
		{"tab_after_margin_kept", "/* a\n *\tb */", "a\n\tb "},
		{"second_star_kept", "/* a\n **b */", "a\n*b "},
		{"crlf", "/*x\r\n * y*/", "x\ny"},
		{"bare_cr", "/* a\r * b*/", "a\nb"},
		{"mixed_terminators", "/* a\r\n * b\r * c\n * d*/", "a\nb\nc\nd"},
		{"leading_tab_dropped", "/*\tx\t*/", "x\t"},
		{"whitespace_first_line", "/*  \n  x  \n  */", "x  \n"},
		{"blank_first_lines", "/*\n\n * a\n*/", "\na\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CommentBody(tc.raw); got != tc.want {
				t.Errorf("CommentBody(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestCommentText(t *testing.T) {
	cases := []struct{ body, want string }{
		{"", "/* */"},
		{"hello", "/* hello*/"},
		{"hello ", "/* hello */"},
		{"a\nb", "/* a\n\t * b*/"},
		{"a\n\nb\n", "/* a\n\t *\n\t * b\n\t */"},
		{"  lead", "/*\n\t *   lead*/"},
		{"\nafter", "/*\n\t *\n\t * after*/"},
	}
	for _, tc := range cases {
		got, ok := CommentText(tc.body, "\t")
		if !ok || got != tc.want {
			t.Errorf("CommentText(%q) = %q, %v; want %q", tc.body, got, ok, tc.want)
		}
	}
	for _, body := range []string{"a */ b", "a\r\nb", "*/"} {
		if got, ok := CommentText(body, ""); ok {
			t.Errorf("CommentText(%q) = %q, want refused", body, got)
		}
	}
}

// TestCommentTextRoundTrip reads back every body CommentText writes.
func TestCommentTextRoundTrip(t *testing.T) {
	bodies := []string{
		"", " ", "\n", "\n\n", "x", " x", "x ", "*x", "* x", "x*", "x/", "/", "*", "**",
		"\tx\t", "a\n b\n  c", "a\n*b\n* c", "a\n \n", "line one\nline two ",
		"\f", "a\n\f b", "//note", "/* nested",
	}
	for _, body := range bodies {
		for _, indent := range []string{"", "    ", "\t"} {
			text, ok := CommentText(body, indent)
			if !ok {
				t.Fatalf("CommentText(%q) refused", body)
			}
			if strings.Index(text[2:], "*/") != len(text)-4 {
				t.Fatalf("CommentText(%q) = %q closes early", body, text)
			}
			if got := CommentBody(text); got != body {
				t.Errorf("CommentBody(CommentText(%q, %q) = %q) = %q", body, indent, text, got)
			}
		}
	}
}

func FuzzCommentText(f *testing.F) {
	for _, seed := range []string{"", "a", " a\n b ", "*\n*", "x\n\n"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		text, ok := CommentText(body, "  ")
		if !ok {
			return
		}
		if got := CommentBody(text); got != body {
			t.Errorf("CommentBody(%q) = %q, want %q", text, got, body)
		}
	})
}

func TestCommentProseLineTerminators(t *testing.T) {
	for _, raw := range []string{"/* a\n * b */", "/* a\r\n * b */", "/* a\r * b */"} {
		if got := CommentProse(raw); got != "a\nb" {
			t.Errorf("CommentProse(%q) = %q, want %q", raw, got, "a\nb")
		}
	}
}
