package repl

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// runCommandCase runs the last of the " ;; "-separated lines of a case with run,
// in a fresh session holding the model and the lines before it.
func runCommandCase(t *testing.T, model, lines string, run func(*Session, string) ([]string, bool, error)) ([]string, error) {
	t.Helper()
	s := NewSession()
	s.Submit(model)
	steps := strings.Split(lines, " ;; ")
	for _, step := range steps[:len(steps)-1] {
		if _, _, err := s.RunMeta(step); err != nil {
			t.Fatalf("%s: %s = %v", lines, step, err)
		}
	}
	out, _, err := run(s, steps[len(steps)-1])
	return out, err
}

func runCommandCases(t *testing.T, name string) (model string, cases []string) {
	t.Helper()
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join("testdata", "run_command", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return read("model.sysml"), strings.Split(strings.TrimSpace(read(name)), "\n")
}

func TestRunCommandFailsAUsageAnswer(t *testing.T) {
	model, cases := runCommandCases(t, "usage.txt")
	for _, lines := range cases {
		printed, err := runCommandCase(t, model, lines, (*Session).RunMeta)
		if err != nil {
			t.Errorf("%s: RunMeta = %v, want its usage printed", lines, err)
			continue
		}
		out, err := runCommandCase(t, model, lines, (*Session).RunCommand)
		var usage *UsageError
		if !errors.As(err, &usage) || len(out) != 0 || !slices.Equal(usage.Lines, printed) {
			t.Errorf("%s: RunCommand = %q, %v; want a UsageError holding %q", lines, out, err, printed)
		}
	}
}

// A command that ran, or failed otherwise, is answered as RunMeta answers it,
// whatever usage text its output quotes from the model.
func TestRunCommandAnswersAsRunMetaOtherwise(t *testing.T) {
	model, cases := runCommandCases(t, "ran.txt")
	for _, lines := range cases {
		want, wantErr := runCommandCase(t, model, lines, (*Session).RunMeta)
		got, gotErr := runCommandCase(t, model, lines, (*Session).RunCommand)
		if !slices.Equal(got, want) || (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
			t.Errorf("%s: RunCommand = %q, %v; RunMeta = %q, %v", lines, got, gotErr, want, wantErr)
		}
	}
}
