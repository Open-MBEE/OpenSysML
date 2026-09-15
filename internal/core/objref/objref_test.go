package objref

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// TestParseReferences: the spellings a session reaches an object under parse to
// the id and segments a walker follows, indexes counting from 1.
func TestParseReferences(t *testing.T) {
	cases := []struct {
		text     string
		id       int64
		segments string // "name[index]" per segment, "." before a dotted one
		head     int
	}{
		{"#2", 2, "", 0},
		{"#2.wheels[2]", 2, "wheels[2]", 0},
		{"#2::engine.power", 2, "engine .power", 1},
		{"car", 0, "car", 1},
		{"Garage::car", 0, "Garage car", 2},
		{"Garage::car.wheels[2]", 0, "Garage car .wheels[2]", 2},
		{"car.engine.power", 0, "car .engine .power", 1},
		{"'My Garage'::car", 0, "My Garage car", 2},
		{"#7.'odd wheel'[1]", 7, "odd wheel[1]", 0},
		{"car.'odd wheel'[1]", 0, "car .odd wheel[1]", 1},
	}
	for _, tc := range cases {
		ref, err := Parse(tc.text)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.text, err)
			continue
		}
		if ref.ID != tc.id {
			t.Errorf("Parse(%q).ID = %d, want %d", tc.text, ref.ID, tc.id)
		}
		var got []string
		for _, seg := range ref.Segments {
			s := seg.Name
			if seg.Dotted {
				s = "." + s
			}
			if seg.Index > 0 {
				s += "[" + strconv.Itoa(seg.Index) + "]"
			}
			got = append(got, s)
		}
		if joined := strings.Join(got, " "); joined != tc.segments {
			t.Errorf("Parse(%q) segments = %q, want %q", tc.text, joined, tc.segments)
		}
		if head := Head(ref.Segments); head != tc.head {
			t.Errorf("Head(%q) = %d, want %d", tc.text, head, tc.head)
		}
	}
}

// TestParseRejectsMalformedReferences: text no session reads as a reference is
// refused with a RefError saying what was wrong.
func TestParseRejectsMalformedReferences(t *testing.T) {
	cases := map[string]string{
		"":              "nothing was named",
		"#":             "written #<id>",
		"#0":            "not an object id",
		"#-1":           "written #<id>",
		"#2wheels":      "written after . or ::",
		"car..engine":   "",
		"car.":          "no feature after it",
		"car[1]":        "takes no index",
		"car.wheels[0]": "counted from 1",
		"car.wheels[x]": "",
		"car wheels":    "cannot follow car",
	}
	for text, detail := range cases {
		_, err := Parse(text)
		if err == nil {
			t.Errorf("Parse(%q) accepted, want a RefError", text)
			continue
		}
		var refErr *RefError
		if !errors.As(err, &refErr) {
			t.Errorf("Parse(%q) = %T %v, want *RefError", text, err, err)
			continue
		}
		if !strings.Contains(err.Error(), "not an object reference") || !strings.Contains(err.Error(), detail) {
			t.Errorf("Parse(%q) = %q, want it to say the text is not an object reference and %q", text, err.Error(), detail)
		}
	}
}

// TestClassifiers: the predicates the surfaces decide a binding's shape by.
func TestClassifiers(t *testing.T) {
	for _, text := range []string{"#1", "#42"} {
		if !IsID(text) {
			t.Errorf("IsID(%q) = false", text)
		}
	}
	for _, text := range []string{"#", "#x", "1", "car"} {
		if IsID(text) {
			t.Errorf("IsID(%q) = true", text)
		}
	}
	if !LooksLikePath("2.5") {
		t.Error("LooksLikePath(\"2.5\") = false: numbers are for callers to rule out first")
	}
	for _, text := range []string{"car.wheels[2]", "#1.engine", "car.engine"} {
		if !LooksLikePath(text) {
			t.Errorf("LooksLikePath(%q) = false", text)
		}
	}
	for _, text := range []string{"car", "Garage::car", "1500", "'a.b'"} {
		if LooksLikePath(text) {
			t.Errorf("LooksLikePath(%q) = true", text)
		}
	}
	ref, err := Parse("Garage::car.wheels[2]")
	if err != nil {
		t.Fatal(err)
	}
	if got := DeclaredRun(ref.Segments[:2]); got != "Garage::car" {
		t.Errorf("DeclaredRun = %q, want Garage::car", got)
	}
	if got := DeclaredRun(ref.Segments); got != "" {
		t.Errorf("DeclaredRun through a dotted segment = %q, want none", got)
	}
	if got := JoinTyped(ref.Segments[:2]); got != "Garage::car" {
		t.Errorf("JoinTyped = %q, want Garage::car", got)
	}
}
