package export_test

import (
	"strings"
	"testing"
)

// A graph stored before bare comments were Comment elements carries such a
// comment only in the next member's text; the text states it, so it comes back.
func TestLegacyBareCommentComesBackFromTheText(t *testing.T) {
	src := "package P {\n    /* c */\n    part def A;\n    part def B;\n}\n"
	turtle := idTurtle(t, src)
	if !strings.Contains(string(turtle), "elmt:P___400\n") {
		t.Fatalf("the bare comment is not the Comment element the test removes:\n%s", turtle)
	}
	legacy := withoutMember(t, turtle, "elmt:P___400")
	legacy = editTurtle(t, legacy, `sysx:sourceText "    part def A;\n"`, `sysx:sourceText "    /* c */\n    part def A;\n"`)
	if strings.Contains(string(legacy), "P___400") {
		t.Fatalf("the legacy graph still declares the Comment:\n%s", legacy)
	}
	if back := toNotation(t, legacy); back != src {
		t.Errorf("the legacy graph's bare comment was lost:\n--- want ---\n%s--- got ---\n%s", src, back)
	}
	edited := editTurtle(t, legacy, `sysml:declaredName "B"`, `sysml:declaredName "C"`)
	want := "package P {\n    /* c */\n    part def A;\n    part def C;\n}\n"
	if back := toNotation(t, edited); back != want {
		t.Errorf("an edit elsewhere lost the legacy graph's bare comment:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}
