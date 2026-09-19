package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// TestHistoryPath covers where the prompt keeps its history, including the
// unwritable case that leaves it in memory.
func TestHistoryPath(t *testing.T) {
	state := t.TempDir()
	home := t.TempDir()
	blocked := filepath.Join(t.TempDir(), "denied")
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tests := []struct {
		name  string
		state string
		home  string
		want  string
	}{
		{
			name:  "XDG_STATE_HOME",
			state: state,
			home:  home,
			want:  filepath.Join(state, "sysml", "history"),
		},
		{
			name: "home fallback when unset",
			home: home,
			want: filepath.Join(home, ".sysml_history"),
		},
		{
			name:  "home fallback when the state directory is unwritable",
			state: blocked,
			home:  home,
			want:  filepath.Join(home, ".sysml_history"),
		},
		{
			name:  "in memory when neither is writable",
			state: blocked,
			home:  blocked,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", tt.state)
			if tt.state == "" {
				if err := os.Unsetenv("XDG_STATE_HOME"); err != nil {
					t.Fatalf("unset: %v", err)
				}
			}
			t.Setenv("HOME", tt.home)
			got := historyPath()
			if got != tt.want {
				t.Fatalf("historyPath() = %q, want %q", got, tt.want)
			}
			if got == "" {
				return
			}
			if _, err := os.Stat(got); err != nil {
				t.Errorf("history file not created: %v", err)
			}
		})
	}
}

// TestSessionCompleterDo checks the adapter answers readline with the remainder
// of each candidate and the length of the word already typed.
func TestSessionCompleterDo(t *testing.T) {
	c := &sessionCompleter{sess: repl.NewSession()}

	tests := []struct {
		name      string
		line      string
		pos       int
		want      string
		wantLen   int
		wantCount int
	}{
		{name: "command remainder", line: "%eva", pos: 4, want: "l", wantLen: 4},
		{name: "at the cursor", line: "%eva x", pos: 4, want: "l", wantLen: 4},
		{name: "no match", line: "%zzzz", pos: 5, wantLen: 5, wantCount: 0},
		{name: "position past the line", line: "%eva", pos: 99, want: "l", wantLen: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, length := c.Do([]rune(tt.line), tt.pos)
			if length != tt.wantLen {
				t.Errorf("length = %d, want %d", length, tt.wantLen)
			}
			if tt.want == "" {
				if len(got) != tt.wantCount {
					t.Errorf("got %d candidates, want %d", len(got), tt.wantCount)
				}
				return
			}
			var texts []string
			for _, cand := range got {
				texts = append(texts, string(cand))
			}
			found := false
			for _, text := range texts {
				if text == tt.want {
					found = true
				}
			}
			if !found {
				t.Errorf("want %q in %v", tt.want, strings.Join(texts, ","))
			}
		})
	}
}
