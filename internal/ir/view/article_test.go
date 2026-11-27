package view

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A notice names an anonymous element by its kind, with the article the kind is
// said with: a use case is written with a vowel and said with a consonant.
func TestNoticeArticleFollowsHowTheKindIsSaid(t *testing.T) {
	r := &Renderer{}
	for kind, want := range map[string]string{
		"use case":         "a use case",
		"include use case": "an include use case",
		"event occurrence": "an event occurrence",
		"flow":             "a flow",
	} {
		if got := r.subject(&symbols.Symbol{}, kind); got != want {
			t.Errorf("subject of an anonymous %s = %q, want %q", kind, got, want)
		}
	}
	for _, kind := range []Kind{KindTree, KindState, KindCase, KindMixed, KindTextual, KindTable, KindMatrix,
		KindSequence, KindTimeline, KindGeometry, KindRequirement, KindDefinition, KindPackage} {
		if got := kind.article(); got != "a" {
			t.Errorf("%s rendering reads with %q, want %q", kind, got, "a")
		}
	}
	for _, kind := range []Kind{KindInterconnection, KindAction} {
		if got := kind.article(); got != "an" {
			t.Errorf("%s rendering reads with %q, want %q", kind, got, "an")
		}
	}
}
