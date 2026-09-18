package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// The did-you-mean hint belongs to the diagnostic, so every surface that shows
// an unresolved reference shows it: the CLI, the REPL and the LSP all read these
// messages.
func TestUnresolvedReferenceSuggestsSpelling(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    string
		absent  string
		noError bool
	}{
		{
			name: "unimported library name suggests its qualified spelling",
			src:  "part def A { attribute x : Integer = 1; }",
			want: "unresolved reference: Integer — did you mean ScalarValues::Integer?",
		},
		{
			name:   "typo suggests the nearest name and nothing further out",
			src:    "part def A { attribute x : Intger; }",
			want:   "unresolved reference: Intger — did you mean ScalarValues::Integer?",
			absent: " or ",
		},
		{
			name: "the user's own declaration rules out a distant library name",
			src:  "part def Wheel;\npart w : Whel;",
			want: "unresolved reference: Whel — did you mean Wheel?",
		},
		{
			name:   "a name too short for a typo to be identifiable is not guessed",
			src:    "part w : Wh;",
			want:   "unresolved reference: Wh",
			absent: "did you mean",
		},
		{
			name:   "a name matching nothing is not guessed",
			src:    "part p : Zzzqqwwvv;",
			want:   "unresolved reference: Zzzqqwwvv",
			absent: "did you mean",
		},
		{
			name: "a typo of an imported library name is offered as written",
			src:  "package T { private import ScalarValues::*; part def A { attribute x : Intger; } }",
			want: "unresolved reference: Intger — did you mean Integer?",
		},
		{
			name:   "a misspelling is not sent to a name nested in another element",
			src:    "part w : Whel;",
			want:   "unresolved reference: Whel",
			absent: "did you mean",
		},
		{
			name: "a name spelled exactly right is still located",
			src:  "part def A { attribute x : when; }",
			want: "unresolved reference: when — did you mean SysML::Systems::TriggerKind::when?",
		},
		{
			name: "a name reported while another is being scored is still hinted",
			src:  "part def Wheel;\npart def Sensor;\npackage P { public import Q::*; part w : Whel; }\npackage Q { private import Sensoor; }",
			want: "unresolved reference: Sensoor — did you mean Sensor?",
		},
		{
			name:   "a qualified name is not second-guessed",
			src:    "part def A { attribute x : Nowhere::Integer; }",
			want:   "unresolved reference: Nowhere::Integer",
			absent: "did you mean",
		},
		{
			name:    "an imported library name resolves",
			src:     "package T { private import ScalarValues::*; part def A { attribute x : Integer = 1; } }",
			noError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := NewWorkspace()
			ws.Open("t.sysml", []byte(tc.src), 1)

			var errs []string
			for _, d := range ws.Diagnostics("t.sysml") {
				if d.Severity == diag.SeverityError {
					errs = append(errs, d.Message)
				}
			}

			if tc.noError {
				if len(errs) > 0 {
					t.Fatalf("unexpected error(s): %s", strings.Join(errs, "; "))
				}
				return
			}

			joined := strings.Join(errs, "; ")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("diagnostics %q do not contain %q", joined, tc.want)
			}
			if tc.absent != "" && strings.Contains(joined, tc.absent) {
				t.Errorf("diagnostics %q contain %q", joined, tc.absent)
			}
		})
	}
}
