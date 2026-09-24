package filename

import (
	"errors"
	"strings"
	"testing"
)

// A name that fits is written as it is; one past Max bytes is cut and tagged
// with a hash of the whole so two long names sharing a prefix stay apart,
// and the cut splits neither a UTF-8 sequence nor a trailing escape.
func TestFitCutsAndTagsLongNames(t *testing.T) {
	if got := Fit("Report", ".md", false); got != "Report.md" {
		t.Errorf("Fit(Report) = %q, want Report.md", got)
	}
	long := strings.Repeat("a", 300)
	got := Fit(long, ".md", false)
	if len(got) != Max || !strings.HasSuffix(got, ".md") || !strings.Contains(got, "~") {
		t.Errorf("Fit(long) = %q (%d bytes), want %d bytes, tagged, ending in .md", got, len(got), Max)
	}
	if other := Fit(long+"b", ".md", false); other == got {
		t.Errorf("two long names sharing a prefix are both written to %q", got)
	}
	tagged := Fit("Report", ".md", true)
	if !strings.HasPrefix(tagged, "Report~") || len(tagged) != len("Report~")+2*TagBytes+len(".md") {
		t.Errorf("Fit(Report, tagged) = %q, want Report~ and a %d-byte hash", tagged, 2*TagBytes)
	}
	budget := Max - len(".md") - len("~") - 2*TagBytes
	for _, name := range []string{
		strings.Repeat("é", 150),
		strings.Repeat("a", budget-1) + ".20" + strings.Repeat("b", 30),
		strings.Repeat("a", budget-1) + "%20" + strings.Repeat("b", 30),
	} {
		got := Fit(name, ".md", false)
		stem := got[:strings.LastIndexByte(got, '~')]
		if !strings.HasPrefix(name, stem) || strings.HasSuffix(stem, "%") || strings.HasSuffix(stem, ".") || strings.HasSuffix(stem, "%2") || strings.HasSuffix(stem, ".2") {
			t.Errorf("Fit(%q) = %q cuts inside a sequence or escape", name[:10]+"…", got)
		}
	}
}

// A stem Windows reads as a device is tagged whatever its case, extension or
// trailing spaces, so the file is a file; other stems are left alone.
func TestFitTagsDeviceStems(t *testing.T) {
	for _, name := range []string{"CON", "con", "Con ", "nul.report", "COM1", "LPT¹"} {
		if !DeviceStem(name) {
			t.Errorf("DeviceStem(%q) = false, want true", name)
		}
		if got := Fit(name, ".md", false); !strings.Contains(got, "~") {
			t.Errorf("Fit(%q) = %q, want tagged", name, got)
		}
	}
	for _, name := range []string{"CONSOLE", "COM10", "Report.con", "%43ON"} {
		if DeviceStem(name) {
			t.Errorf("DeviceStem(%q) = true, want false", name)
		}
	}
}

// Plan tags every name whose file meets another's letter case aside, a name
// whose plain file is another's tagged file is tagged in turn, a name that
// meets none keeps its plain file, and two names still meeting tagged are refused.
func TestPlanKeepsFilesApart(t *testing.T) {
	file := func(name string, tagged bool) string { return Fit(name, ".md", tagged) }
	tagged := file("Report", true)
	tagName := strings.TrimSuffix(tagged, ".md")
	got, err := Plan([]string{"Report", "report", tagName, "other"}, file)
	if err != nil {
		t.Fatal(err)
	}
	if got["Report"] != tagged || got["other"] != "other.md" {
		t.Errorf("files = %v, want Report as %s and other plain", got, tagged)
	}
	if got[tagName] == tagged || !strings.Contains(got[tagName], "~") {
		t.Errorf("%s named like the tag is written to %q, want a tagged file other than %s", tagName, got[tagName], tagged)
	}
	folded := map[string]string{}
	for name, f := range got {
		if other, met := folded[CaseFolded(f)]; met {
			t.Errorf("%s and %s are both written to %s", other, name, f)
		}
		folded[CaseFolded(f)] = name
	}
	_, err = Plan([]string{"Report", "Report"}, file)
	var collision *CollisionError
	if !errors.As(err, &collision) || collision.File != tagged {
		t.Errorf("one name twice err = %v, want a CollisionError on %s", err, tagged)
	}
}

// Case folding is Unicode's simple folding, which strings.EqualFold decides:
// a final sigma folds with a sigma, a Kelvin sign with a k, and a name that
// differs in more than case folds apart.
func TestCaseFoldedAgreesWithEqualFold(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"Report.dot", "report.dot", true},
		{"σ", "ς", true},
		{"Σ", "ς", true},
		{"k", "\u212a", true},
		{"ß", "ẞ", true},
		{"ſ", "S", true},
		{"i", "İ", false},
		{"Report.dot", "Reports.dot", false},
	} {
		if got := CaseFolded(tc.a) == CaseFolded(tc.b); got != tc.want || got != strings.EqualFold(tc.a, tc.b) {
			t.Errorf("CaseFolded(%q) == CaseFolded(%q) is %v, want %v, as EqualFold says %v", tc.a, tc.b, got, tc.want, strings.EqualFold(tc.a, tc.b))
		}
	}
}
