package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// escapeWarnings parses a document and returns the text each
// nonstandard-notation warning spans, with its messages and the parse errors.
func escapeWarnings(t *testing.T, name, input string) (spans, messages, errs []string) {
	t.Helper()
	src := source.New(name, []byte(input))
	p := New(src)
	p.ParseFile()
	for _, w := range p.Warnings {
		if w.Code == codeNonstandardNotation {
			spans = append(spans, src.Text(w.Span))
			messages = append(messages, w.Message)
		}
	}
	for _, d := range p.Diagnostics {
		errs = append(errs, d.Message)
	}
	return spans, messages, errs
}

// `\r` is the pilot grammar's escape, not one of KerML §8.2.2.3 Table 4: it
// stays parsed and each spelling is warned of, in a name and a string alike.
func TestNonstandardEscapeIsWarnedOf(t *testing.T) {
	for _, tc := range []struct {
		name, file, input, terminal string
		want                        int
	}{
		{"name_sysml", "a.sysml", `package P { part def 'a\rb'; }`, "an unrestricted name", 1},
		{"name_kerml", "a.kerml", `package P { class 'a\rb'; }`, "an unrestricted name", 1},
		{"string_sysml", "a.sysml", `package P { attribute s = "a\rb"; }`, "a string", 1},
		{"string_kerml", "a.kerml", `package P { feature s = "a\rb"; }`, "a string", 1},
		{"each_spelling", "a.sysml", `package P { attribute s = "\r\n\r"; }`, "a string", 2},
		{"name_reference", "a.sysml", `package P { part def 'a\rb'; part p : 'a\rb'; }`, "an unrestricted name", 2},
		{"short_name", "a.sysml", `package P { part def <'a\rb'> D; }`, "an unrestricted name", 1},
		{"beside_table4_escapes", "a.sysml", `package P { part def 'a\t\rb\\'; }`, "an unrestricted name", 1},
		// The crossing-feature try-parse reads the name first and restores: one warning, not two or none.
		{"after_cross_feature_attempt", "a.sysml", `package P { interface def I { end <'a\rb'>; } }`, "an unrestricted name", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spans, messages, errs := escapeWarnings(t, tc.file, tc.input)
			if len(errs) != 0 {
				t.Fatalf("`\\r` must stay parsed, got errors %v", errs)
			}
			if len(spans) != tc.want {
				t.Fatalf("got %d warning(s) %v, want %d", len(spans), messages, tc.want)
			}
			for i, sp := range spans {
				if sp != `\r` {
					t.Errorf("warning spans %q, want the escape `\\r`", sp)
				}
				if !strings.Contains(messages[i], "`\\r` in "+tc.terminal) || !strings.Contains(messages[i], source.StandardEscapes()) {
					t.Errorf("message = %q, want it to name the escape, %s and Table 4's sequences", messages[i], tc.terminal)
				}
			}
		})
	}
}

// Every escape sequence of Table 4 is standard notation, and a carriage return
// that is not an escape — a raw one, or the `r` after an escaped backslash — is no finding.
func TestTable4EscapesStaySilent(t *testing.T) {
	for _, tc := range []struct{ name, file, input string }{
		{"name_sysml", "a.sysml", `package P { part def 'a\'\"\b\f\t\n\\b'; }`},
		{"name_kerml", "a.kerml", `package P { class 'a\'\"\b\f\t\n\\b'; }`},
		{"string_sysml", "a.sysml", `package P { attribute s = "a\'\"\b\f\t\n\\b"; }`},
		{"string_kerml", "a.kerml", `package P { feature s = "a\'\"\b\f\t\n\\b"; }`},
		{"escaped_backslash_then_r", "a.sysml", `package P { part def 'a\\r'; attribute s = "a\\r"; }`},
		{"raw_carriage_return", "a.sysml", "package P { attribute s = \"a\rb\";\r\n part def 'a\rb'; }"},
		{"outside_a_quoted_token", "a.sysml", `package P { doc /* a\rb */ part def r; } // \r`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spans, messages, errs := escapeWarnings(t, tc.file, tc.input)
			if len(errs) != 0 {
				t.Fatalf("parse errors %v", errs)
			}
			if len(spans) != 0 {
				t.Fatalf("got warning(s) %v, want none", messages)
			}
		})
	}
}

// An escape the lexer does not admit stays the error it was, and is not also a warning.
func TestInvalidEscapeIsNotANonstandardEscape(t *testing.T) {
	spans, _, errs := escapeWarnings(t, "a.sysml", `package P { attribute s = "a\qb\r"; }`)
	if len(errs) != 1 || !strings.Contains(errs[0], `invalid escape '\q' in a string`) {
		t.Fatalf("errors = %v, want the one invalid escape", errs)
	}
	if len(spans) != 1 || spans[0] != `\r` {
		t.Fatalf("warnings span %v, want `\\r` alone", spans)
	}
}
