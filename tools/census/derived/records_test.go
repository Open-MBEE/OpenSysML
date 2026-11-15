package derived

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordsRoundTripThroughTheCommittedLayout(t *testing.T) {
	report := &Report{Roots: []RootReport{
		{Name: "training", Dir: "a", Files: 2, Rules: map[string]RuleCounts{
			"Usage::nestedPart": {Ours: Counts{Carriers: 5, Value: 3, Empty: 2}, Pilot: Counts{Carriers: 5, Value: 4, Empty: 1}},
			"Feature::type":     {Ours: Counts{Carriers: 1, NotDerived: 1}, Pilot: Counts{Carriers: 1, Value: 1}},
		}},
		{Name: "examples", Dir: "b", Files: 1, Rules: map[string]RuleCounts{
			"Feature::type": {Ours: Counts{Carriers: 7, Value: 7}, Pilot: Counts{Carriers: 7, Value: 7}},
		}},
	}}
	encoded := string(encodeRecords(report))
	lines := strings.Split(strings.TrimSpace(encoded), "\n")
	var records []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") {
			records = append(records, line)
		}
	}
	want := []string{
		"Feature::type\ttraining\t1\t0\t0\t1\t1\t1\t0\t0",
		"Feature::type\texamples\t7\t7\t0\t0\t7\t7\t0\t0",
		"Usage::nestedPart\ttraining\t5\t3\t2\t0\t5\t4\t1\t0",
	}
	if strings.Join(records, "\n") != strings.Join(want, "\n") {
		t.Fatalf("records:\n%s\nwant:\n%s", strings.Join(records, "\n"), strings.Join(want, "\n"))
	}
	head, err := committedHeader(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(head), `"rules"`) || strings.Contains(string(head), `"totals"`) || strings.Contains(string(head), `"summary"`) {
		t.Errorf("header carries what the records or the reader derive:\n%s", head)
	}
	decoded := Report{Roots: []RootReport{{Name: "training"}, {Name: "examples"}}}
	if err := decodeRecords([]byte(encoded), &decoded); err != nil {
		t.Fatal(err)
	}
	for i, root := range report.Roots {
		for name, counts := range root.Rules {
			if decoded.Roots[i].Rules[name] != counts {
				t.Errorf("%s %s decoded as %+v, want %+v", root.Name, name, decoded.Roots[i].Rules[name], counts)
			}
		}
		if len(decoded.Roots[i].Rules) != len(root.Rules) {
			t.Errorf("%s decoded %d rules, want %d", root.Name, len(decoded.Roots[i].Rules), len(root.Rules))
		}
	}
	if err := decodeRecords([]byte("Feature::type\tunknown\t1\t0\t0\t0\t1\t0\t0\t0\n"), &decoded); err == nil {
		t.Error("a record naming a root the header lacks decoded")
	}
	if err := decodeRecords([]byte("Feature::type\ttraining\t1\n"), &decoded); err == nil {
		t.Error("a short record decoded")
	}
}

func TestRecordsReproduceNamesTheRecordsThatMoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.tsv")
	committed := "# header\nA::x\ttraining\t1\t1\t0\t0\t1\t1\t0\t0\nB::y\ttraining\t2\t2\t0\t0\t2\t2\t0\t0\nC::z\ttraining\t3\t3\t0\t0\t3\t3\t0\t0\n"
	if err := writeFile(path, committed); err != nil {
		t.Fatal(err)
	}
	if err := recordsReproduce(path, []byte(committed)); err != nil {
		t.Fatalf("identical records do not reproduce: %v", err)
	}
	fresh := "# header\nA::x\ttraining\t1\t1\t0\t0\t1\t1\t0\t0\nB::y\ttraining\t2\t1\t1\t0\t2\t2\t0\t0\nD::w\ttraining\t4\t4\t0\t0\t4\t4\t0\t0\n"
	err := recordsReproduce(path, []byte(fresh))
	if err == nil {
		t.Fatal("moved records reproduce")
	}
	for _, want := range []string{"B::y\ttraining\t2\t1\t1\t0", "D::w\ttraining", "gone: C::z\ttraining", refreshCommand} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "A::x") {
		t.Errorf("error names an unmoved record:\n%v", err)
	}
}
