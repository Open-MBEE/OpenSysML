package docir

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
)

const formattedFixture = `
	part def Sub {
		attribute zone : String;
		attribute mass : Real;
	}
	part telescope {
		part optics : Sub {
			attribute redefines zone = "payload";
			attribute redefines mass = 8.5;
		}
		part mount : Sub {
			attribute redefines zone = "support";
			attribute redefines mass = 15.0;
		}
		part camera : Sub {
			attribute redefines zone = "payload";
			attribute redefines mass = 3.0;
		}
	}
	calc def Zoned :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("zone", "name", "mass")
		)
	}
	part def Report :> Document {
		attribute redefines title = "Report";
		part formatted : Paragraph {
			part lead : Span {
				attribute redefines text = "plain";
			}
			part em : Span {
				attribute redefines text = "emphasized";
				attribute redefines style = "emphasis";
			}
			part bold : Span {
				attribute redefines text = "bolded";
				attribute redefines style = "strong";
			}
			part code : Span {
				attribute redefines text = "x > 1";
				attribute redefines style = "code";
			}
			part spec : Link {
				attribute redefines text = "the spec";
				attribute redefines target = "https://example.com/spec";
			}
			part see : Ref {
				ref redefines target = zones;
			}
		}
		part zones : Table {
			attribute redefines groupBy = "zone";
			calc rows : Zoned {
				in root = telescope;
			}
		}
	}
`

// TestEvaluateInlineRuns locks the typed run kinds an evaluated paragraph
// carries, and the anchor a referenced node receives.
func TestEvaluateInlineRuns(t *testing.T) {
	fixture := loadEvaluationFixture(t, formattedFixture)
	document := fixture.mustEvaluate(t, "Report")
	runs := document.Content()[0].Runs()
	if len(runs) != 6 {
		t.Fatalf("runs = %d, want 6", len(runs))
	}
	for i, want := range []struct {
		kind   RunKind
		text   string
		target string
	}{
		{RunPlain, "plain", ""},
		{RunEmphasis, "emphasized", ""},
		{RunStrong, "bolded", ""},
		{RunCode, "x > 1", ""},
		{RunLink, "the spec", "https://example.com/spec"},
		{RunRef, "zones", "zones"},
	} {
		if runs[i].Kind() != want.kind || runs[i].Text() != want.text || runs[i].Target() != want.target {
			t.Errorf("run %d = %s %q %q, want %s %q %q",
				i, runs[i].Kind(), runs[i].Text(), runs[i].Target(), want.kind, want.text, want.target)
		}
		if !runs[i].Origin().Located() {
			t.Errorf("run %d has no origin", i)
		}
	}
	zones := document.Content()[1]
	if zones.Anchor() != "zones" {
		t.Errorf("referenced table anchor = %q", zones.Anchor())
	}
	if document.Content()[0].Anchor() != "" {
		t.Errorf("unreferenced paragraph has anchor %q", document.Content()[0].Anchor())
	}
}

// TestEvaluateGroupedTable locks grouping semantics: rows partition by the
// group column's text in order of first appearance, keeping row order.
func TestEvaluateGroupedTable(t *testing.T) {
	fixture := loadEvaluationFixture(t, formattedFixture)
	document := fixture.mustEvaluate(t, "Report")
	zones := document.Content()[1]
	if zones.GroupBy() != "zone" {
		t.Fatalf("groupBy = %q", zones.GroupBy())
	}
	groups := zones.Groups()
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	if groups[0].Key() != "payload" || groups[1].Key() != "support" {
		t.Fatalf("group keys = %q, %q", groups[0].Key(), groups[1].Key())
	}
	if len(groups[0].Rows()) != 2 || len(groups[1].Rows()) != 1 {
		t.Fatalf("group sizes = %d, %d", len(groups[0].Rows()), len(groups[1].Rows()))
	}
	if len(zones.Rows()) != 3 {
		t.Fatalf("ungrouped rows = %d", len(zones.Rows()))
	}
}

// TestEvaluatedGroupsAreImmutable checks the defensive copies around a
// grouped table's groups and their rows.
func TestEvaluatedGroupsAreImmutable(t *testing.T) {
	fixture := loadEvaluationFixture(t, formattedFixture)
	document := fixture.mustEvaluate(t, "Report")
	zones := document.Content()[1]
	groups := zones.Groups()
	groups[0] = TableGroup{}
	if zones.Groups()[0].Key() != "payload" {
		t.Fatal("mutating returned groups changed the table")
	}
	rows := zones.Groups()[0].Rows()
	rows[0] = queryexec.Row{}
	if zones.Groups()[0].Rows()[0].Cells() == nil {
		t.Fatal("mutating returned group rows changed the table")
	}
}

// TestAnchorForEncoding locks the anchor encoding: identifier bytes pass
// through, everything else hex-encodes, segments join with "-".
func TestAnchorForEncoding(t *testing.T) {
	for _, c := range []struct {
		path []string
		want string
	}{
		{[]string{"zones"}, "zones"},
		{[]string{"a_1", "b"}, "a_1-b"},
		{[]string{"sp ace"}, "sp.20ace"},
		{[]string{"a-b"}, "a.2Db"},
		{[]string{"a", "b"}, "a-b"},
	} {
		if got := AnchorFor(c.path); got != c.want {
			t.Errorf("AnchorFor(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
