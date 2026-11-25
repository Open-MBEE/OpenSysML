package hideo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaselineRoundTrips(t *testing.T) {
	b := baselineOf(t, threeCases)
	path := filepath.Join(t.TempDir(), "baseline.tsv")
	if err := WriteBaseline(path, b); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Encode(), b.Encode()) || got.Pin != testPin {
		t.Errorf("read back\n%s\nwant\n%s", got.Encode(), b.Encode())
	}
	want := "A01-001\tpass\tpass\tpass\tpass\nA01-002\tunsupported\tunsupported\tunsupported\tunsupported\nB02-001\tfail\tfail\tfail\tfail\n"
	if !strings.HasSuffix(string(b.Encode()), want) {
		t.Errorf("encoded\n%s", b.Encode())
	}
}

func TestReadBaselineRefusesAMalformedFile(t *testing.T) {
	good := string(baselineOf(t, threeCases).Encode())
	tests := map[string]string{
		"unknown outcome": strings.Replace(good, "A01-001\tpass", "A01-001\tpassed", 1),
		"missing column":  strings.Replace(good, "B02-001\tfail\tfail", "B02-001\tfail", 1),
		"duplicate case":  good + "A01-001\tpass\tpass\tpass\tpass\n",
		"out of order":    good + "A00-001\tpass\tpass\tpass\tpass\n",
		"no commit":       strings.Replace(good, commitPrefix+testPin.Commit+"\n", "", 1),
		"no header":       strings.Replace(good, "case\t", "id\t", 1),
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "baseline.tsv")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadBaseline(path); err == nil {
				t.Error("accepted")
			}
		})
	}
}

func TestTableCountsPerMatrixAndSuite(t *testing.T) {
	fresh := uniform(threeCases)
	fresh[Suites[3]]["A01-001"] = Result{Outcome: Skip}
	b, err := Record(testPin, fresh)
	if err != nil {
		t.Fatal(err)
	}
	table := b.Table()
	for _, want := range []string{
		"| A01 | 2 | 1 / 0 / 1 | 1 / 0 / 1 | 1 / 0 / 1 | 0 / 0 / 1 (+1 skip) |\n",
		"| B02 | 1 | 0 / 1 / 0 | 0 / 1 / 0 | 0 / 1 / 0 | 0 / 1 / 0 |\n",
		"| **All** | 3 | 1 / 1 / 1 | 1 / 1 / 1 | 1 / 1 / 1 | 0 / 1 / 1 (+1 skip) |\n",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("table lacks %q:\n%s", want, table)
		}
	}
}

func TestDocTableIsCheckedAgainstTheBaseline(t *testing.T) {
	b := baselineOf(t, threeCases)
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("intro\n"+countsBegin+"stale\n"+countsEnd+"outro\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckDocTable(path, b); err == nil {
		t.Error("a stale table was accepted")
	}
	if err := WriteDocTable(path, b); err != nil {
		t.Fatal(err)
	}
	if err := CheckDocTable(path, b); err != nil {
		t.Error(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "intro\n" + countsBegin + b.Table() + countsEnd + "outro\n"; string(got) != want {
		t.Errorf("document\n%s\nwant\n%s", got, want)
	}
	if err := os.WriteFile(path, []byte("no markers\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteDocTable(path, b); err == nil {
		t.Error("a document without the block was accepted")
	}
}
