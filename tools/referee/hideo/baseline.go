package hideo

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
)

// BaselinePath is the committed baseline, relative to the repository root.
const BaselinePath = "docs/project/hideo-suite-baseline.tsv"

// Suites are the suite's OpenSysML readers the ratchet runs, in baseline column
// order: the element-properties API and the Symbol API, each from Python and Go.
var Suites = []string{"opensysml-py-props", "opensysml-go-props", "opensysml-py", "opensysml-go"}

// Outcome is the suite's verdict on one case.
type Outcome string

// The outcomes the suite writes.
const (
	Pass        Outcome = "pass"
	Fail        Outcome = "fail"
	Unsupported Outcome = "unsupported"
	Error       Outcome = "error"
	Skip        Outcome = "skip"
)

// Outcomes lists every outcome, in the order the figures are reported.
var Outcomes = []Outcome{Pass, Fail, Unsupported, Error, Skip}

// rank orders outcomes for the ratchet: a pass, then a case not attempted, then a case not met.
func (o Outcome) rank() int {
	switch o {
	case Pass:
		return 2
	case Unsupported, Skip:
		return 1
	default:
		return 0
	}
}

func parseOutcome(s string) (Outcome, error) {
	if o := Outcome(s); slices.Contains(Outcomes, o) {
		return o, nil
	}
	return "", fmt.Errorf("unknown outcome %q", s)
}

// Baseline is the committed record: the pin it was measured at and, per case
// id, one outcome per suite in the order of Suites.
type Baseline struct {
	Pin   Pin
	Cases map[string][]Outcome
}

// IDs lists the baseline's case ids in order.
func (b *Baseline) IDs() []string {
	ids := make([]string, 0, len(b.Cases))
	for id := range b.Cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

const (
	headerNote       = "# Per-case outcomes of the cross-MDK conformance suite; written by `go run -C tools ./cmd/hideo-suite -update`, never by hand."
	repositoryPrefix = "# repository\t"
	commitPrefix     = "# commit\t"
)

// Encode renders the baseline: three provenance lines, a header naming the
// suites, and one line per case in id order, so equal runs are byte-identical.
func (b *Baseline) Encode() []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s\n%s%s\n%s%s\n", headerNote, repositoryPrefix, b.Pin.Repository, commitPrefix, b.Pin.Commit)
	fmt.Fprintf(&buf, "case\t%s\n", strings.Join(Suites, "\t"))
	for _, id := range b.IDs() {
		buf.WriteString(id)
		for _, o := range b.Cases[id] {
			buf.WriteString("\t" + string(o))
		}
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// WriteBaseline records b as the committed baseline at path.
func WriteBaseline(path string, b *Baseline) error {
	return os.WriteFile(path, b.Encode(), 0o600)
}

// ReadBaseline decodes the committed baseline at path and refuses one that is
// not exactly what Encode writes, so a hand edit of its layout is caught.
func ReadBaseline(path string) (*Baseline, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- callers name a committed baseline in this repository
	if err != nil {
		return nil, err
	}
	b, err := decode(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !bytes.Equal(b.Encode(), content) {
		return nil, fmt.Errorf("%s is not in the recorded form (case order, spacing or header); re-record it with -update", path)
	}
	return b, nil
}

func decode(content []byte) (*Baseline, error) {
	b := &Baseline{Cases: make(map[string][]Outcome)}
	header := "case\t" + strings.Join(Suites, "\t")
	seenHeader := false
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		switch {
		case strings.HasPrefix(text, repositoryPrefix):
			b.Pin.Repository = strings.TrimPrefix(text, repositoryPrefix)
		case strings.HasPrefix(text, commitPrefix):
			b.Pin.Commit = strings.TrimPrefix(text, commitPrefix)
		case strings.HasPrefix(text, "#"):
		case text == header:
			seenHeader = true
		default:
			fields := strings.Split(text, "\t")
			if !seenHeader || len(fields) != len(Suites)+1 || fields[0] == "" {
				return nil, fmt.Errorf("line %d: expected the header %q, then a case id and %d outcomes", line, header, len(Suites))
			}
			if _, dup := b.Cases[fields[0]]; dup {
				return nil, fmt.Errorf("line %d: case %s recorded twice", line, fields[0])
			}
			outcomes := make([]Outcome, len(Suites))
			for i, f := range fields[1:] {
				o, err := parseOutcome(f)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", line, err)
				}
				outcomes[i] = o
			}
			b.Cases[fields[0]] = outcomes
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if b.Pin.Repository == "" || !commitRe.MatchString(b.Pin.Commit) {
		return nil, fmt.Errorf("no %q and %q lines naming the suite measured", strings.TrimSpace(repositoryPrefix), strings.TrimSpace(commitPrefix))
	}
	return b, nil
}
