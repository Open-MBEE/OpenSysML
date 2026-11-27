package migrate

import "testing"

// A note names a keyword, a UML metaclass or a diagram kind with the article it
// is said with, whatever its case: "a use case", "an Activity".
func TestNoteArticleFollowsHowTheWordIsSaid(t *testing.T) {
	for word, want := range map[string]string{
		"use case":         "a use case",
		"usage":            "a usage",
		"UseCase":          "a UseCase",
		"Use Case Diagram": "a Use Case Diagram",
		"item":             "an item",
		"attribute def":    "an attribute def",
		"Activity":         "an Activity",
		"OpaqueExpression": "an OpaqueExpression",
		"part":             "a part",
		"SignalEvent":      "a SignalEvent",
	} {
		if got := kwArticle(word); got != want {
			t.Errorf("kwArticle(%q) = %q, want %q", word, got, want)
		}
	}
}
