package migrate

import "testing"

// TestCommentTextStripsStyleAndScript checks a comment body's style and
// script elements are removed with their contents before tag stripping, that
// the widened trigger fires for style, img, div and span markup, and that a
// plain-text body keeps its spacing where a marked-up one has it collapsed.
func TestCommentTextStripsStyleAndScript(t *testing.T) {
	for _, tc := range []struct {
		body string
		want string
	}{
		{`<html><head><style>p {padding:0px; margin:0px;}</style></head><body><p>Figure 1 caption</p></body></html>`, "Figure 1 caption"},
		{`<style>p {margin:0}</style>A figure`, "A figure"},
		{`<script>alert(1)</script>A figure`, "A figure"},
		{`<img src="x.png">A figure`, "A figure"},
		{`<div>A</div><div>B</div>`, "AB"},
		{`<span>A</span>`, "A"},
		{`plain text`, "plain text"},
		{"  two  spaces,\ta tab\n\tand indentation  ", "two  spaces,\ta tab\n\tand indentation"},
		{"<p>two  spaces,\ta tab</p>", "two spaces, a tab"},
	} {
		if got := commentText(tc.body); got != tc.want {
			t.Errorf("commentText(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}
