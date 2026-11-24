package rdf

import "testing"

// A relative IRI reference resolves against @base as RFC 3986 section 5 does,
// not by appending it to the base.
func TestTurtleRelativeIRIResolvesAgainstBase(t *testing.T) {
	cases := []struct{ base, ref, want string }{
		{"http://example.org/a/b", "c", "http://example.org/a/c"},
		{"http://example.org/a/b/", "c", "http://example.org/a/b/c"},
		{"http://example.org/a/b", "../c", "http://example.org/c"},
		{"http://example.org/a/b", "/c", "http://example.org/c"},
		{"http://example.org/a/b", "#c", "http://example.org/a/b#c"},
		{"http://example.org/a/b", "", "http://example.org/a/b"},
		{"http://example.org/a/b", "#", "http://example.org/a/b#"},
		{"http://example.org/a/b#old", "#c", "http://example.org/a/b#c"},
		{"http://example.org/a/b?q", "", "http://example.org/a/b?q"},
		{"http://example.org/a/b?q", "?r", "http://example.org/a/b?r"},
		{"http://example.org/a/b?q", "c", "http://example.org/a/c"},
		{"http://example.org/a/b", "é/ü", "http://example.org/a/é/ü"},
		{"http://example.org/a/b", "./c/./d/..", "http://example.org/a/c/"},
		{"http://example.org/a/b", "../../../c", "http://example.org/c"},
		{"http://example.org", "c", "http://example.org/c"},
		{"file:///a/b", "c", "file:///a/c"},
		{"http://example.org/a/b", "//other.example/x/../y", "http://other.example/y"},
		{"http://example.org/a/b", "//other.example", "http://other.example"},
		{"http://example.org/a/b", "//other.example/", "http://other.example/"},
		{"http://example.org/a/b", "//", "http://"},
		{"http://example.org/a/b", "///x", "http:///x"},
		{"models?next:/guide", "c", "models?next:/guidec"},
		{"a/b:/c", "d", "a/b:/cd"},
		{"urn:x:", "c", "urn:x:c"},
	}
	for _, tc := range cases {
		g, err := ParseTurtle([]byte("@base <" + tc.base + "> .\n<" + tc.ref + "> <http://example.org/p> <http://example.org/o> .\n"))
		if err != nil {
			t.Fatalf("base %s, reference %q: %v", tc.base, tc.ref, err)
		}
		if !g.HasProperty(IRI(tc.want), "http://example.org/p") {
			t.Errorf("base %s, reference %q: no subject %s", tc.base, tc.ref, tc.want)
		}
	}
}
