package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/convert"
)

// TestIsExperimental checks that the RDF mapping is what marks a conversion
// experimental, in either direction, and that notation alone does not.
func TestIsExperimental(t *testing.T) {
	cases := []struct {
		from, to convert.Format
		want     bool
	}{
		{convert.FormatSysML, convert.FormatTurtle, true},
		{convert.FormatTurtle, convert.FormatSysML, true},
		{convert.FormatTurtle, convert.FormatTurtle, true},
		{convert.FormatSysML, convert.FormatSysML, false},
		{convert.FormatXMI, convert.FormatSysML, true},
		{convert.FormatXMI, convert.FormatTurtle, true},
	}
	for _, c := range cases {
		if got := convert.IsExperimental(c.from, c.to); got != c.want {
			t.Errorf("convert.IsExperimental(%s, %s) = %t, want %t", c.from, c.to, got, c.want)
		}
	}
}

// TestNotices checks each experimental conversion names what is experimental
// about it: the migration, the RDF mapping, or both for XMI to Turtle.
func TestNotices(t *testing.T) {
	cases := []struct {
		from, to convert.Format
		want     []string
	}{
		{convert.FormatSysML, convert.FormatSysML, nil},
		{convert.FormatSysML, convert.FormatTurtle, []string{convert.ExperimentalNotice}},
		{convert.FormatXMI, convert.FormatSysML, []string{convert.MigrationNotice}},
		{convert.FormatXMI, convert.FormatTurtle, []string{convert.MigrationNotice, convert.ExperimentalNotice}},
	}
	for _, c := range cases {
		got := convert.Notices(c.from, c.to)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("convert.Notices(%s, %s) = %q, want %q", c.from, c.to, got, c.want)
		}
		if convert.IsExperimental(c.from, c.to) != (len(got) > 0) {
			t.Errorf("convert.IsExperimental(%s, %s) disagrees with its notices", c.from, c.to)
		}
	}
	for _, want := range []string{"experimental", "docs/reference/sysml-v1-migration.md"} {
		if !strings.Contains(convert.MigrationNotice, want) {
			t.Errorf("migration notice is missing %q:\n%s", want, convert.MigrationNotice)
		}
	}
}

// TestExperimentalNoticeNamesTheStatusSection checks the notice points at the
// documentation that carries the status, so every surface quoting it does.
func TestExperimentalNoticeNamesTheStatusSection(t *testing.T) {
	for _, want := range []string{"experimental", "docs/reference/rdf-mapping.md"} {
		if !strings.Contains(convert.ExperimentalNotice, want) {
			t.Errorf("notice is missing %q:\n%s", want, convert.ExperimentalNotice)
		}
	}
}
