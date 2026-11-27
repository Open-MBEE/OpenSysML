package diag

import "testing"

func TestIndefiniteArticle(t *testing.T) {
	for phrase, want := range map[string]string{
		"":                 "a",
		"part":             "a",
		"#meta part":       "a",
		"attribute def":    "an",
		"analysis case":    "an",
		"enum":             "an",
		"item":             "an",
		"occurrence":       "an",
		"unknown":          "an",
		"uninterpreted":    "an",
		"use case":         "a",
		"use case def":     "a",
		"Use Case Diagram": "a",
		"UseCase":          "a",
		"usage":            "a",
		"Usage":            "a",
		"unit":             "a",
		"union":            "a",
		"unique feature":   "a",
		"utility":          "a",
		"one-of":           "a",
		"oneOf":            "a",
		"Activity":         "an",
		"OpaqueBehavior":   "an",
		"Interaction":      "an",
	} {
		if got := IndefiniteArticle(phrase); got != want {
			t.Errorf("IndefiniteArticle(%q) = %q, want %q", phrase, got, want)
		}
	}
}
