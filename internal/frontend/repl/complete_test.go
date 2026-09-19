package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// contains reports whether candidates holds want.
func contains(candidates []string, want string) bool {
	for _, c := range candidates {
		if c == want {
			return true
		}
	}
	return false
}

// TestCompleteMetaCommands covers completing a command at the start of a line.
func TestCompleteMetaCommands(t *testing.T) {
	tests := []struct {
		line    string
		wants   []string
		rejects []string
	}{
		{line: "%", wants: []string{"%help", "%eval", "%search", "%builtins"}},
		{line: "%ev", wants: []string{"%eval"}, rejects: []string{"%help"}},
		{line: "%se", wants: []string{"%search"}, rejects: []string{"%eval"}},
		{line: "  %bui", wants: []string{"%builtins"}},
		{line: "%zzzz", rejects: []string{"%eval", "%help"}},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := NewSession().Complete(tt.line, len(tt.line))
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("completing %q: want %q in %v", tt.line, want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("completing %q: did not want %q in %v", tt.line, bad, got.Candidates)
				}
			}
			if want := strings.TrimLeft(tt.line, " \t"); got.Prefix != want {
				t.Errorf("completing %q: prefix = %q, want %q", tt.line, got.Prefix, want)
			}
		})
	}
}

// TestCompleteNames covers completing declared, builtin and library names.
func TestCompleteNames(t *testing.T) {
	s := NewSession()
	if res := s.Submit("part def Wheel { attribute diameter = 1.0; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
	}

	tests := []struct {
		name    string
		line    string
		wants   []string
		rejects []string
	}{
		{name: "declared name", line: "%eval Whe", wants: []string{"Wheel"}},
		{name: "declared member", line: "%eval diam", wants: []string{"diameter"}},
		{name: "builtin function", line: "%eval sqr", wants: []string{"sqrt"}},
		{name: "library namespace", line: "%eval ScalarVal", wants: []string{"ScalarValues"}},
		{
			name:    "one qualified segment at a time",
			line:    "%eval ScalarValues::",
			wants:   []string{"ScalarValues::Integer"},
			rejects: []string{"ScalarValues::Integer::"},
		},
		{
			// A completed namespace still answers with its members: offering
			// the typed word back inserts nothing.
			name:    "a whole namespace offers its members",
			line:    "%eval ScalarValues",
			wants:   []string{"ScalarValues::Integer"},
			rejects: []string{"ScalarValues", "ScalarValues::Integer::"},
		},
		{name: "inside an expression", line: "%eval 1 + Whe", wants: []string{"Wheel"}},
		{name: "no match", line: "%eval zzzznotaname", rejects: []string{"Wheel", "sqrt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.Complete(tt.line, len(tt.line))
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("completing %q: want %q in %v", tt.line, want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("completing %q: did not want %q in %v", tt.line, bad, got.Candidates)
				}
			}
			for _, c := range got.Candidates {
				if !strings.HasPrefix(c, got.Prefix) {
					t.Errorf("candidate %q does not extend prefix %q", c, got.Prefix)
				}
			}
			if len(got.Candidates) > completionLimit {
				t.Errorf("got %d candidates, want at most %d", len(got.Candidates), completionLimit)
			}
		})
	}
}

// TestCompletePaths covers the file paths %load and %save complete to.
func TestCompletePaths(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"model.sysml", "other.sysml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("part def A;\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tests := []struct {
		name    string
		line    string
		wants   []string
		rejects []string
	}{
		{
			name:  "every entry of a directory",
			line:  "%load " + dir + "/",
			wants: []string{dir + "/model.sysml", dir + "/other.sysml", dir + "/nested/"},
		},
		{
			name:    "narrowed by what is typed",
			line:    "%load " + dir + "/mod",
			wants:   []string{dir + "/model.sysml"},
			rejects: []string{dir + "/other.sysml"},
		},
		{
			name:  "%save completes paths too",
			line:  "%save " + dir + "/oth",
			wants: []string{dir + "/other.sysml"},
		},
		{
			name:    "missing directory offers nothing",
			line:    "%load " + dir + "/nowhere/x",
			rejects: []string{dir + "/model.sysml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSession().Complete(tt.line, len(tt.line))
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("completing %q: want %q in %v", tt.line, want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("completing %q: did not want %q in %v", tt.line, bad, got.Candidates)
				}
			}
		})
	}
}

// TestNameWord checks the word under the cursor is taken whole, including one
// written with letters outside ASCII, so what is inserted extends what is typed.
func TestNameWord(t *testing.T) {
	tests := []struct {
		head, want string
	}{
		{"", ""},
		{"%eval sqr", "sqr"},
		{"%eval ISQ::ma", "ISQ::ma"},
		{"attribute x : Sca", "Sca"},
		{"%eval 1 + señ", "señ"},
		{"%eval señor::mas", "señor::mas"},
		{"%eval (Δv", "Δv"},
	}
	for _, tt := range tests {
		if got := nameWord(tt.head); got != tt.want {
			t.Errorf("nameWord(%q) = %q, want %q", tt.head, got, tt.want)
		}
	}
}

// TestCompleteConcurrentWithSubmit covers Tab arriving while the session is
// busy: readline completes from its own input goroutine, both while the loop
// evaluates a line and while the files named on the command line are loading.
func TestCompleteConcurrentWithSubmit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "startup.sysml")
	if err := os.WriteFile(path, []byte("part def Startup { attribute a = 1.0; }\n"), 0o600); err != nil {
		t.Fatalf("write startup file: %v", err)
	}

	for _, tt := range []struct {
		name string
		busy func(s *Session, i int)
	}{
		{name: "submitting", busy: func(s *Session, i int) {
			s.Submit(fmt.Sprintf("part def P%d { attribute a = 1.0; }", i))
		}},
		{name: "loading files", busy: func(s *Session, _ int) {
			if _, err := s.LoadPaths([]string{dir}); err != nil {
				t.Errorf("LoadPaths: %v", err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				for i := 0; i < 20; i++ {
					tt.busy(s, i)
				}
			}()
			go func() {
				defer wg.Done()
				for i := 0; i < 20; i++ {
					s.Complete("%eval ScalarValues::In", len("%eval ScalarValues::In"))
				}
			}()
			wg.Wait()
		})
	}
}

// TestCompletionBoundKeepsInsertion checks bounding a large answer never offers
// a longer shared prefix than the whole match set has: the prompt inserts what
// the candidates share, so dropping matches must not insert letters they lack.
func TestCompletionBoundKeepsInsertion(t *testing.T) {
	// Alphabetically first past the bound, so truncation alone would share "Ax".
	crowded := make([]string, 0, completionLimit+2)
	for i := 0; i < completionLimit+1; i++ {
		crowded = append(crowded, fmt.Sprintf("Ax%04d", i))
	}

	tests := []struct {
		name       string
		word       string
		candidates []string
		wantShared string
		wantCount  int
	}{
		{name: "under the bound", word: "A", candidates: []string{"Ab", "Ac"}, wantShared: "A", wantCount: 2},
		{
			name:       "bound offers nothing rather than the wrong text",
			word:       "A",
			candidates: append(append([]string(nil), crowded...), "Azz"),
			wantCount:  0, // every match shares only the typed word
		},
		{
			name:       "shared prefix extends the typed word",
			word:       "A",
			candidates: append(append([]string(nil), crowded...), "Ax1"),
			wantShared: "Ax",
			wantCount:  1,
		},
		{
			name:       "bounding is safe when the prefix is unchanged",
			word:       "Ax",
			candidates: crowded,
			wantShared: "Ax0",
			wantCount:  completionLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := completion(tt.word, tt.candidates)
			if len(got.Candidates) != tt.wantCount {
				t.Fatalf("offered %d candidates, want %d", len(got.Candidates), tt.wantCount)
			}
			if shared := sharedPrefix(got.Candidates); len(got.Candidates) > 0 && shared != tt.wantShared {
				t.Fatalf("shared prefix = %q, want %q", shared, tt.wantShared)
			}
		})
	}
}

// TestSharedPrefixKeepsWholeCharacters checks the shared prefix of names that
// differ inside a multi-byte character stays valid text the prompt can insert.
func TestSharedPrefixKeepsWholeCharacters(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		want       string
	}{
		{name: "differ inside a character", candidates: []string{"masé", "masê"}, want: "mas"},
		{name: "share a whole character", candidates: []string{"señor", "señora"}, want: "señor"},
		{name: "differ at the first character", candidates: []string{"Δv", "Ωv"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sharedPrefix(tt.candidates)
			if got != tt.want {
				t.Fatalf("sharedPrefix = %q, want %q", got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("sharedPrefix %q is not valid UTF-8", got)
			}
		})
	}
}

// TestSplitPath checks a typed path is split at the last separator the platform
// reads as one, so a path written with backslashes completes on Windows.
func TestSplitPath(t *testing.T) {
	tests := []struct {
		word, dir, base string
	}{
		{"", "", ""},
		{"model.sysml", "", "model.sysml"},
		{"/tmp/models/mod", "/tmp/models/", "mod"},
		{"/tmp/models/", "/tmp/models/", ""},
	}
	for _, tt := range tests {
		if dir, base := splitPath(tt.word); dir != tt.dir || base != tt.base {
			t.Errorf("splitPath(%q) = %q, %q; want %q, %q", tt.word, dir, base, tt.dir, tt.base)
		}
	}

	// A backslash separates only where the platform says it does: elsewhere it
	// is an ordinary character in a file name.
	const win = `C:\models\ro`
	wantDir, wantBase := "", win
	if os.IsPathSeparator('\\') {
		wantDir, wantBase = `C:\models\`, "ro"
	}
	if dir, base := splitPath(win); dir != wantDir || base != wantBase {
		t.Errorf("splitPath(%q) = %q, %q; want %q, %q", win, dir, base, wantDir, wantBase)
	}
}

// TestCompletePosition checks completion answers about the word at the cursor,
// not the end of the line.
func TestCompletePosition(t *testing.T) {
	s := NewSession()
	line := "%eval sqr + 1"
	got := s.Complete(line, len("%eval sqr"))
	if !contains(got.Candidates, "sqrt") {
		t.Errorf("want sqrt in %v", got.Candidates)
	}
	if got.Prefix != "sqr" {
		t.Errorf("prefix = %q, want %q", got.Prefix, "sqr")
	}
	// An out-of-range position is answered about the whole line.
	if out := s.Complete(line, len(line)+10); out.Prefix != "1" {
		t.Errorf("out-of-range position: prefix = %q, want %q", out.Prefix, "1")
	}
	if out := s.Complete(line, -1); out.Prefix != "1" {
		t.Errorf("negative position: prefix = %q, want %q", out.Prefix, "1")
	}
}
