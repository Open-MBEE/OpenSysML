package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// enumerationBodyDiags returns the enumeration-body-member findings of src.
func enumerationBodyDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	return only(analyzeSrc(t, src), "enumeration-body-member")
}

// Every declaration the general body grammar admits but EnumerationBody does
// not is reported, at any nesting depth, as a non-blocking notation error whose
// message names the member and the enumeration.
func TestEnumerationBodyRejectsNonEnumeratedDeclarations(t *testing.T) {
	const src = `package P {
		enum def E {
			attribute def Nested;
			a;
		}
		part def Holder {
			part h {
				enum def F {
					part def Q;
					enum def Inner;
					private import P::*;
					alias x for E::a;
					package Sub;
					b;
				}
			}
		}
	}`
	diags := enumerationBodyDiags(t, src)
	wantLines := []int{3, 9, 10, 11, 12, 13}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diagnostics %v, want %d", len(diags), diags, len(wantLines))
	}
	for i, d := range diags {
		if got := w8dLine(src, d.Span); got != wantLines[i] {
			t.Errorf("diagnostic %d on line %d, want %d: %s", i, got, wantLines[i], d.Message)
		}
		if d.Severity != SeverityError || !d.Notation || d.Blocking() || d.Source != "syntax" {
			t.Errorf("diagnostic %d = %+v, want a non-blocking syntax notation error", i, d)
		}
	}
	wantMessages := []string{
		"enumeration definition `E` cannot own attribute def `Nested`",
		"enumeration definition `F` cannot own part def `Q`",
		"enumeration definition `F` cannot own enum def `Inner`",
		"enumeration definition `F` cannot own this import",
		"enumeration definition `F` cannot own alias `x`",
		"enumeration definition `F` cannot own package `Sub`",
	}
	for i, want := range wantMessages {
		if !strings.Contains(diags[i].Message, want) || !strings.Contains(diags[i].Message, "holds only enumerated values and annotations") {
			t.Errorf("message %d = %q, want it to contain %q", i, diags[i].Message, want)
		}
	}
	if d := diags[0]; !strings.HasSuffix(d.Message, "move attribute def `Nested` out of `E`") {
		t.Errorf("message = %q, want the fix spelled out", d.Message)
	}
}

// A nested definition is reported once here and not again by the
// variation-membership constraint, which owns the non-enumerated usages.
func TestEnumerationBodyDoesNotDuplicateVariationMembership(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		enum def E {
			part def Q;
			attribute y : Integer;
			a;
		}
	}`
	diags := analyzeSrc(t, src)
	if got := only(diags, "enumeration-body-member"); len(got) != 1 || w8dLine(src, got[0].Span) != 4 {
		t.Fatalf("enumeration-body-member = %v, want one on line 4", got)
	}
	if got := only(diags, "variation-member-not-variant"); len(got) != 1 || w8dLine(src, got[0].Span) != 5 {
		t.Fatalf("variation-member-not-variant = %v, want one on line 5", got)
	}
}

// Enumerated values in every form and the annotating elements the grammar
// admits stay silent; so does a nested definition in a non-enumeration body.
func TestEnumerationBodyAdmitsValuesAndAnnotations(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		metadata def M;
		enum def E {
			doc /* d */
			comment /* c */
			locale "en" /* l */
			rep language "x" /* y */
			@M;
			metadata M;
			#M a;
			private b;
			enum c;
			d : E;
			= 1;
			<s>;
		}
		enum def Empty;
		enum def Braced {}
		attribute def AD { enum def Inner { a; } attribute def NestedIsFine; }
		variation part def V { part def Inner; variant part v : Inner; metadata M; }
	}`
	for _, code := range []string{"enumeration-body-member", "variation-member-not-variant"} {
		if diags := only(analyzeSrc(t, src), code); len(diags) != 0 {
			t.Fatalf("legal model reported %s: %v", code, diags)
		}
	}
}

// The parser itself carries the finding as a warning on the member, so a direct
// parser consumer sees it before any pass runs; the analysis escalates it.
func TestEnumerationBodyMemberIsAParserWarning(t *testing.T) {
	p := parser.New(source.New("<t>", []byte("package P { enum def E { part def Q; a; } }")))
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse errors = %v, want none: the member still reads", p.Diagnostics)
	}
	if len(p.Warnings) != 1 || p.Warnings[0].Code != CodeEnumerationBodyMember {
		t.Fatalf("parse warnings = %v, want one %s", p.Warnings, CodeEnumerationBodyMember)
	}
}

// A body the parser could not read is the parser's finding, not a second one here.
func TestEnumerationBodyLeavesParseErrorsAlone(t *testing.T) {
	const src = `package P {
		enum def E {
			a;
			+ ;
			b;
		}
	}`
	if diags := enumerationBodyDiags(t, src); len(diags) != 0 {
		t.Fatalf("parse-error member reported: %v", diags)
	}
}
