package migrate

import "testing"

// TestCommentTextStripsStyleAndScript checks a comment body's style and
// script elements are removed with their contents before tag stripping, and
// that the widened trigger fires for style, img, div and span markup.
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
	} {
		if got := commentText(tc.body); got != tc.want {
			t.Errorf("commentText(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}
