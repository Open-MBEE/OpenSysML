package docplan

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const valueRunsModel = `
		part def Mount { attribute mass : Real = 80.0; }
		part def Telescope { part mount : Mount; attribute aperture : Real = 2.4; }
		part telescope : Telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			rep language "markdown" /* Aperture [](sysml:value:telescope.aperture) and
			 * [mount mass](sysml:value:telescope.mount.mass); nominal [](sysml:value:Telescope::aperture).
			 */
		}
	`

// A `sysml:value:` destination plans a value run reading the named feature
// from the chain's root, or from the owner of a feature named outright, with
// its label kept and its origin inside the body.
func TestCompileMarkdownBodyValueRuns(t *testing.T) {
	fixture := loadPlanningFixture(t, valueRunsModel)
	plan := fixture.mustCompile(t, "Report")
	var values []Run
	for _, run := range plan.Content()[0].Runs() {
		if run.Kind() == RunValue {
			values = append(values, run)
		}
	}
	if len(values) != 3 {
		t.Fatalf("value runs = %d, want 3", len(values))
	}
	want := []struct{ text, root, path, feature string }{
		{"", "Observatory::telescope", "aperture", "Observatory::Telescope::aperture"},
		{"mount mass", "Observatory::telescope", "mount.mass", "Observatory::Mount::mass"},
		{"", "Observatory::Telescope", "aperture", "Observatory::Telescope::aperture"},
	}
	full := "package Observatory {" + planningImports + valueRunsModel + "}"
	for i, run := range values {
		if run.Text() != want[i].text || symbols.FQNOf(run.ValueRoot()) != want[i].root || run.ValuePath() != want[i].path || run.ValueFeature() != want[i].feature {
			t.Errorf("value run %d = %q root %s path %q feature %s, want %+v", i, run.Text(), symbols.FQNOf(run.ValueRoot()), run.ValuePath(), run.ValueFeature(), want[i])
		}
		if o := run.Origin(); !o.Located() || !strings.HasPrefix(full[o.Span.Offset:o.Span.End()], "[") {
			t.Errorf("value run %d origin covers %q, want the link", i, full[o.Span.Offset:o.Span.End()])
		}
	}
}

// A value target that names no feature, cannot be read as one, or names an
// element rather than a datum is a typed planning error at its body line.
func TestCompileMarkdownBodyValueErrors(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		kind     ErrorKind
		contains string
	}{
		{"unknown", "rep language \"markdown\" /* Fine.\n *\n * Mass [](sysml:value:telescope.nowhere). */", ErrorUnknownValueTarget, "names unknown target telescope.nowhere (body line 3)"},
		{"unparseable", "rep language \"markdown\" /* Mass [](sysml:value:1+2). */", ErrorInvalidValueTarget, `got "sysml:value:1+2" (body line 1)`},
		{"part usage", "rep language \"markdown\" /* Mount [](sysml:value:telescope.mount). */", ErrorInvalidValueTarget, "got Observatory::Telescope::mount, a part usage (body line 1)"},
		{"definition", "rep language \"markdown\" /* Type [](sysml:value:Telescope). */", ErrorInvalidValueTarget, "got Observatory::Telescope, a part definition (body line 1)"},
		{"content block", "rep language \"markdown\" /* See [](sysml:value:details). */\n part details : Section { attribute redefines title = \"D\"; }", ErrorInvalidValueTarget, "Observatory::Report::details, a part usage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				part def Mount { attribute mass : Real = 80.0; }
				part def Telescope { part mount : Mount; }
				part telescope : Telescope;
				part def Report :> Document {
					attribute redefines title = "Report";
					`+tc.body+`
				}
			`)
			_, err := fixture.compile(t, "Report")
			planning := bodyPlanningError(t, err)
			if planning.Kind != tc.kind {
				t.Fatalf("kind %s (%v), want %s", planning.Kind, err, tc.kind)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("message %q lacks %q", err.Error(), tc.contains)
			}
			if !planning.Origin.Located() {
				t.Errorf("error %v has no origin", err)
			}
		})
	}
}

// The AsciiDoc `sysml:value[<target>]` macro plans the same value run, its
// second positional attribute the label.
func TestCompileAsciiDocBodyValueRuns(t *testing.T) {
	useRecordedAsciiDoc(t)
	fixture := loadPlanningFixture(t, `
		package Optics {
			part def Glass { attribute mass : Real; }
			part mirror : Glass { :>> mass = 120.0; }
			part mount : Glass { :>> mass = 80.0; }
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			`+asciiDocBody(t, "asciidoc_values.adoc")+`
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	var got []string
	for _, run := range plan.Content()[0].Runs() {
		if run.Kind() == RunValue {
			got = append(got, run.Text()+"|"+symbols.FQNOf(run.ValueRoot())+"|"+run.ValuePath())
		}
	}
	want := []string{"|Observatory::Optics::mirror|mass", "mount mass:|Observatory::Optics::mount|mass"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("value runs %q, want %q", got, want)
	}
}
