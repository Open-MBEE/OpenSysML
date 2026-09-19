package docrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMarkdownEmphasisDelimiters checks that emphasis and strong delimiters
// stay flanking around escaped content, with whitespace kept outside.
func TestMarkdownEmphasisDelimiters(t *testing.T) {
	for _, c := range []struct{ marker, in, want string }{
		{"*", "plain", "*plain*"},
		{"*", "with *stars*", `*with \*stars\**`},
		{"*", " padded ", " *padded* "},
		{"*", "\ttabbed\t", "\t*tabbed*\t"},
		{"*", "\t ", "\t "},
		{"*", "  ", "  "},
		{"**", "bold_move", `**bold\_move**`},
		{"**", "a|b", `**a\|b**`},
	} {
		if got := delimited(c.marker, c.in); got != c.want {
			t.Errorf("delimited(%q, %q) = %q, want %q", c.marker, c.in, got, c.want)
		}
	}
}

// TestMarkdownCodeSpans checks the code-span fence contract: the fence is
// longer than any inner backtick run, with padding when the content needs it.
func TestMarkdownCodeSpans(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"x > 1", "`x > 1`"},
		{"a `tick`", "`` a `tick` ``"},
		{"``double``", "``` ``double`` ```"},
		{"`leading", "`` `leading ``"},
		{" padded ", "`  padded  `"},
		{"", "`  `"},
		{"two\nlines", "`two lines`"},
	} {
		if got := codeSpan(c.in); got != c.want {
			t.Errorf("codeSpan(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMarkdownLinkDestinations checks pointy-bracket destination escaping:
// backslashes and angle brackets escape, newlines percent-encode.
func TestMarkdownLinkDestinations(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"https://example.com/spec(v2).md", "https://example.com/spec(v2).md"},
		{"https://example.com/a b", "https://example.com/a b"},
		{`a\b`, `a\\b`},
		{"a<b>c", `a\<b\>c`},
		{"a\nb", "a%0Ab"},
		{"a\r\nb", "a%0Ab"},
	} {
		if got := destination(c.in); got != c.want {
			t.Errorf("destination(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMarkdownAnchorsEmptyReferencedList checks that a referenced list keeps
// its anchor even when its query returns no rows.
func TestMarkdownAnchorsEmptyReferencedList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty_ref.sysml")
	model := `
		package Anchors {
			private import DocumentQueries::*;
			private import KerML::Root::Element;
			private import ScalarValues::*;

			calc def Names :> Query {
				in root : Element;
				Project(
					source = OwnedElements(source = root),
					properties = ("name")
				)
			}

			part hollow;

			part def Report :> Document {
				attribute redefines title = "Report";
				part formatted : Paragraph {
					part see : Ref {
						ref redefines target = items;
					}
				}
				part items : List {
					calc rows : Names {
						in root = hollow;
					}
				}
			}
		}
	`
	if err := os.WriteFile(path, []byte(model), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got := renderFixtureDocument(t, path, "Anchors::Report")
	for _, want := range []string{`<a id="items"></a>`, `[items](#items)`} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
}

// TestMarkdownGroupedTableEmpty checks that a grouped table whose filter
// selects no rows still renders a header-only pipe table.
func TestMarkdownGroupedTableEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty_group.sysml")
	model := `
		package Groups {
			private import DocumentQueries::*;
			private import KerML::Root::Element;
			private import ScalarValues::*;

			calc def Zoned :> Query {
				in root : Element;
				WhereName(
					source = Project(
						source = OwnedElements(source = root),
						properties = ("zone", "name")
					),
					operator = "==",
					value = "nomatch"
				)
			}

			part def Widget {
				attribute zone : String;
			}

			part hollow {
				part inner : Widget {
					attribute redefines zone = "payload";
				}
			}

			part def Report :> Document {
				attribute redefines title = "Report";
				part zones : Table {
					attribute redefines groupBy = "zone";
					calc rows : Zoned {
						in root = hollow;
					}
				}
			}
		}
	`
	if err := os.WriteFile(path, []byte(model), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got := renderFixtureDocument(t, path, "Groups::Report")
	want := "| zone | name |\n| --- | --- |"
	if !strings.Contains(got, want) {
		t.Errorf("rendering does not contain %q\n%s", want, got)
	}
}

// TestMarkdownGoldenInlineRuns spot-checks the rendered inline runs, anchors,
// and grouped subtables of the telescope report.
func TestMarkdownGoldenInlineRuns(t *testing.T) {
	got := renderFixtureDocument(t,
		"testdata/telescope_report.sysml",
		"Observatory::MassReport")
	for _, want := range []string{
		// Styled spans escape their content inside the right delimiters.
		`*em\*ph\*asis*`,
		`**bold\_move**`,
		"`` mass >= `limit` ``",
		// The link's text escapes; the destination stays literal.
		`[the \[spec\]](<https://example.com/spec(v2).md>)`,
		// The default ref label is the target section's title; both refs
		// point at emitted anchors.
		`[Subsystem Masses \| by \*name\*](#breakdown)`,
		`[the zone groups](#zones)`,
		`<a id="breakdown"></a>`,
		`<a id="zones"></a>`,
		// Each group renders its key in strong emphasis, then a subtable.
		"**zone: payload**\n\n| zone | name | mass |",
		"**zone: support \\| \\*frame\\***",
		"| payload | optics | 8.5 |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
}
