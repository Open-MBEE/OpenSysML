package usage

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestWrapKeepsWordsWholeWithinTheWidth(t *testing.T) {
	text := "one two three four five six seven"
	got := Wrap(text, 10)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 10 {
			t.Errorf("line %q is wider than 10", line)
		}
	}
	if strings.Join(strings.Fields(got), " ") != text {
		t.Errorf("wrapping changed the words: %q", got)
	}
}

func groupedDoc() (Doc, *flag.FlagSet) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	var b bool
	var s string
	fs.BoolVar(&b, "help", false, "Show this help")
	fs.BoolVar(&b, "h", false, "Show this help")
	fs.StringVar(&s, "target", "c", "Compile to `language` "+strings.Repeat("and more ", 12))
	d := Doc{
		Command:  "t",
		Synopsis: []string{"[options]"},
		Options: []OptionGroup{{Title: "General", Options: []Option{
			Opt("help", "", "h"),
			Opt("target", "<language>"),
		}}},
	}
	return d, fs
}

func TestGroupedOptionsFoldAliasesAndWrap(t *testing.T) {
	d, fs := groupedDoc()
	var out bytes.Buffer
	d.WriteText(&out, fs)
	text := out.String()
	for _, want := range []string{"\nGeneral:\n", "  -h, -help   ", "  -target <language>   ", "(default c)"} {
		if !strings.Contains(text, want) {
			t.Errorf("help lacks %q:\n%s", want, text)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if len(line) > textWidth {
			t.Errorf("line is %d wide: %q", len(line), line)
		}
	}

	out.Reset()
	d.WriteRoff(&out, fs, ManMeta{})
	roff := out.String()
	for _, want := range []string{".SS General\n", `.BR \-h ", " \-help` + "\n", `.BR \-target " \fIlanguage\fP"` + "\n", "The default is c."} {
		if !strings.Contains(roff, want) {
			t.Errorf("man page lacks %q:\n%s", want, roff)
		}
	}
}

func TestCheckOptionsFindsStrayFlags(t *testing.T) {
	d, fs := groupedDoc()
	if err := d.CheckOptions(fs); err != nil {
		t.Fatalf("complete groups: %v", err)
	}
	var extra bool
	fs.BoolVar(&extra, "extra", false, "not grouped")
	if err := d.CheckOptions(fs); err == nil || !strings.Contains(err.Error(), "-extra") {
		t.Errorf("ungrouped flag not reported: %v", err)
	}
	d.Options[0].Options = append(d.Options[0].Options, Opt("extra", ""), Opt("missing", ""))
	if err := d.CheckOptions(fs); err == nil || !strings.Contains(err.Error(), "-missing") {
		t.Errorf("undeclared option not reported: %v", err)
	}
}
