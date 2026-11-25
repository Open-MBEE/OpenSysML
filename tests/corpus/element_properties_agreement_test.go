package corpus

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/corpussubset"
)

// updateAgreementRatchet re-records the counts under testdata/element_properties_agreement/,
// keeping each row's clause key.
var updateAgreementRatchet = flag.Bool("update-agreement-ratchet", false, "re-record the counts of tests/corpus/testdata/element_properties_agreement/<kind>.txt, keeping each row's clause key")

// agreementRatchetDir holds clauses.txt (key, side, clause: the adjudications) and one
// <kind>.txt per kind of disagreement (class, count, clause key).
const agreementRatchetDir = "testdata/element_properties_agreement"

// agreementGate names the ratchet in the banner a subset prints.
const agreementGate = "element-properties agreement ratchet"

var agreementKinds = []string{"unresolved-id", "metaclass", "value", "exported-not-answered", "answered-not-exported"}

// agreementRow is one class of disagreement between the api-json export and GetElementProperties:
// a metamodel property of a metaclass (or its @id / @type) and the kind of disagreement.
type agreementRow struct{ class, kind string }

// agreementRatchet is the recorded count of a class with the adjudication it rests on:
// the clause's key, the side that is correct (symbol or export) and the KerML/SysML clause.
type agreementRatchet struct {
	count             int
	key, side, clause string
}

type agreementClause struct{ side, text string }

// agreementCounts collects the sweep's disagreements per class, with the first example of each.
type agreementCounts struct {
	mu       sync.Mutex
	counts   map[agreementRow]int
	examples map[agreementRow]string
	agreed   int
}

func (c *agreementCounts) note(row agreementRow, example string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts == nil {
		c.counts = map[agreementRow]int{}
		c.examples = map[agreementRow]string{}
	}
	c.counts[row]++
	if _, ok := c.examples[row]; !ok {
		c.examples[row] = example
	}
}

func (c *agreementCounts) agree() {
	c.mu.Lock()
	c.agreed++
	c.mu.Unlock()
}

func agreementClauses(t *testing.T) map[string]agreementClause {
	t.Helper()
	path := filepath.Join(agreementRatchetDir, "clauses.txt")
	out := map[string]agreementClause{}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("%s: want key, side, clause; got %q", path, line)
		}
		if fields[1] != "symbol" && fields[1] != "export" {
			t.Fatalf("%s: %q: the side is symbol or export", path, line)
		}
		if _, dup := out[fields[0]]; dup {
			t.Fatalf("%s: clause %s twice", path, fields[0])
		}
		out[fields[0]] = agreementClause{fields[1], fields[2]}
	}
	return out
}

func agreementRatchetRows(t *testing.T) map[agreementRow]agreementRatchet {
	t.Helper()
	clauses := agreementClauses(t)
	out := map[agreementRow]agreementRatchet{}
	for _, kind := range agreementKinds {
		path := filepath.Join(agreementRatchetDir, kind+".txt")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) != 3 {
				t.Fatalf("%s: want class, count, clause key; got %q", path, line)
			}
			count, err := strconv.Atoi(fields[1])
			if err != nil {
				t.Fatalf("%s: %q: %v", path, line, err)
			}
			row := agreementRow{fields[0], kind}
			if _, dup := out[row]; dup {
				t.Fatalf("%s: %s twice", path, fields[0])
			}
			recorded := agreementRatchet{count: count, key: fields[2], side: "?", clause: "?"}
			if clause, ok := clauses[fields[2]]; ok {
				recorded.side, recorded.clause = clause.side, clause.text
			} else if fields[2] != "?" {
				t.Fatalf("%s: %s: no clause %s in clauses.txt", path, fields[0], fields[2])
			}
			out[row] = recorded
		}
	}
	return out
}

func writeAgreementRatchet(t *testing.T, recorded map[agreementRow]agreementRatchet, counts map[agreementRow]int) {
	t.Helper()
	byKind := map[string][]agreementRow{}
	for row := range counts {
		byKind[row.kind] = append(byKind[row.kind], row)
	}
	for _, kind := range agreementKinds {
		rows := byKind[kind]
		sort.Slice(rows, func(i, j int) bool { return rows[i].class < rows[j].class })
		var b strings.Builder
		fmt.Fprintf(&b, "# %s disagreements between the api-json export and GetElementProperties over the\n", kind)
		b.WriteString("# committed examples, counted per metamodel property (Metaclass::property, the\n")
		b.WriteString("# export's metaclass). Tab-separated: class, count, key of the clause in clauses.txt\n")
		b.WriteString("# (the side that is correct and the KerML/SysML clause it rests on).\n")
		b.WriteString("# TestExamplesSweep fails on a row not here, on a count above the one here, on a row\n")
		b.WriteString("# here the run no longer produces, and on a ? key; a count that fell is re-recorded\n")
		b.WriteString("# with -update-agreement-ratchet.\n")
		for _, row := range rows {
			key := "?"
			if prior, ok := recorded[row]; ok {
				key = prior.key
			}
			fmt.Fprintf(&b, "%s\t%d\t%s\n", row.class, counts[row], key)
		}
		if err := os.WriteFile(filepath.Join(agreementRatchetDir, kind+".txt"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// sweptAgreement holds each sweep part's disagreements until every part has run: the recorded
// counts are over the whole sweep, so only the parts together can be held to them exactly.
var sweptAgreement struct {
	mu       sync.Mutex
	parts    map[string]*agreementCounts
	partial  bool
	recorded bool
}

// TestMain fails a re-recording the run did not complete, which would otherwise pass having written nothing.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && *updateAgreementRatchet && !sweptAgreement.recorded {
		fmt.Fprintf(os.Stderr, "FAIL: -update-agreement-ratchet wrote nothing: re-recording %s needs every sweep part in one run (-run TestExamplesSweep)\n", agreementRatchetDir)
		code = 1
	}
	os.Exit(code)
}

// checkAgreementRatchet holds a sweep part to the recorded classes: none new, none above its count, each
// with a clause. The part that completes the whole sweep holds the summed counts to the record exactly.
func checkAgreementRatchet(t *testing.T, subset corpussubset.Subset, part sweepPart, complete bool, agreement *agreementCounts) {
	t.Helper()
	recorded := agreementRatchetRows(t)
	if *updateAgreementRatchet && !complete {
		t.Fatalf("re-recording %s needs the whole sweep with every corpus downloaded (%s=%q)", agreementRatchetDir, corpussubset.EnvVar, subset.Name())
	}
	whole := sweptAgreementWith(part, complete, agreement)
	if whole != nil && *updateAgreementRatchet {
		writeAgreementRatchet(t, recorded, whole.counts)
		sweptAgreement.recorded = true
		recorded = agreementRatchetRows(t)
	}
	unchecked := 0
	for row, count := range agreement.counts {
		prior, ok := recorded[row]
		switch {
		case !ok && !*updateAgreementRatchet:
			t.Errorf("%s %s: %d new disagreements, e.g. %s", row.class, row.kind, count, agreement.examples[row])
		case ok && count > prior.count:
			t.Errorf("%s %s: %d disagreements, %d recorded (%s is correct: %s); e.g. %s", row.class, row.kind, count, prior.count, prior.side, prior.clause, agreement.examples[row])
		}
	}
	for row, prior := range recorded {
		if prior.side == "?" || prior.clause == "?" {
			t.Errorf("%s/%s.txt: %s carries no clause key", agreementRatchetDir, row.kind, row.class)
		}
		if _, ok := agreement.counts[row]; !ok && !subset.Whole() {
			unchecked++
		}
	}
	announceUnchecked(t, agreementGate, subset, unchecked, len(recorded), "classes")
	t.Logf("%s: %d (element, property) pairs agree, %d classes of disagreement", part.name, agreement.agreed, len(agreement.counts))
	if whole == nil {
		return
	}
	fell := 0
	for row, prior := range recorded {
		switch count, ok := whole.counts[row]; {
		case !ok:
			t.Errorf("%s/%s.txt: %s no longer differs; remove the row", agreementRatchetDir, row.kind, row.class)
		case count > prior.count:
			t.Errorf("%s %s: %d disagreements over the whole sweep, %d recorded (%s is correct: %s); e.g. %s", row.class, row.kind, count, prior.count, prior.side, prior.clause, whole.examples[row])
		case count < prior.count:
			fell++
		}
	}
	if fell > 0 {
		t.Logf("%d classes fell below their recorded count; re-record with -update-agreement-ratchet", fell)
	}
	t.Logf("whole sweep: %d (element, property) pairs agree, %d classes of disagreement", whole.agreed, len(whole.counts))
}

// sweptAgreementWith adds a part's disagreements and returns the whole sweep's once every part has
// run complete in this process, nil until then.
func sweptAgreementWith(part sweepPart, complete bool, agreement *agreementCounts) *agreementCounts {
	sweptAgreement.mu.Lock()
	defer sweptAgreement.mu.Unlock()
	if sweptAgreement.parts == nil {
		sweptAgreement.parts = map[string]*agreementCounts{}
	}
	sweptAgreement.parts[part.name] = agreement
	sweptAgreement.partial = sweptAgreement.partial || !complete
	if sweptAgreement.partial || len(sweptAgreement.parts) != len(sweepParts) {
		return nil
	}
	whole := &agreementCounts{counts: map[agreementRow]int{}, examples: map[agreementRow]string{}}
	for _, p := range sweepParts {
		swept := sweptAgreement.parts[p.name]
		whole.agreed += swept.agreed
		for row, count := range swept.counts {
			whole.counts[row] += count
			if _, ok := whole.examples[row]; !ok {
				whole.examples[row] = swept.examples[row]
			}
		}
	}
	return whole
}

// metamodelProperty reports whether name is a property the metamodel declares on class or an ancestor,
// as opposed to a key the export writes for its own reader (`redefines`, `specializes`, `subsets`, `references`).
func metamodelProperty(class, name string) bool {
	for _, definition := range ontology.Properties() {
		if definition.Name == name && ontology.IsAncestorOrSelf(class, definition.DefiningClass) {
			return true
		}
	}
	return false
}
