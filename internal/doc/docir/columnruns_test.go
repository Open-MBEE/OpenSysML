package docir

import (
	"errors"
	"testing"
)

const columnRunModel = `
	part def Sub {
		attribute zone : String;
		attribute mass : Real;
		attribute style : String;
		attribute url : String;
		attribute caption : String;
	}
	part telescope {
		part optics : Sub {
			attribute redefines zone = "payload";
			attribute redefines mass = 8.5;
			attribute redefines style = "emphasis";
			attribute redefines url = "https://example.com/optics";
		}
		part mount : Sub {
			attribute redefines zone = "support";
			attribute redefines mass = 15.0;
			attribute redefines style = "strong";
			attribute redefines url = "https://example.com/mount";
		}
	}
	calc def Styled :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "style", "url", "zone"),
			columns = (Column(name = "label", expression = "sub: " + Element::name))
		)
	}
`

// TestEvaluateColumnRuns locks styled query-produced runs: fixed styles,
// row-driven styles, links, and computed columns, in template order per row.
func TestEvaluateColumnRuns(t *testing.T) {
	fixture := loadEvaluationFixture(t, columnRunModel+`
		part def Report :> Document {
			attribute redefines title = "Report";
			part styled : Paragraph {
				calc names : Styled {
					in root = telescope;
				}
				part rowStyled : SpanColumn {
					attribute redefines column = "name";
					attribute redefines styleColumn = "style";
				}
				part codeLabel : SpanColumn {
					attribute redefines column = "label";
					attribute redefines style = "code";
				}
				part linked : LinkColumn {
					attribute redefines column = "name";
					attribute redefines targetColumn = "url";
				}
			}
			part items : List {
				calc names : Styled {
					in root = telescope;
				}
				part plainName : SpanColumn {
					attribute redefines column = "name";
				}
			}
		}
	`)
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
		{RunStrong, "mount", ""},
		{RunCode, "sub: mount", ""},
		{RunLink, "mount", "https://example.com/mount"},
		{RunEmphasis, "optics", ""},
		{RunCode, "sub: optics", ""},
		{RunLink, "optics", "https://example.com/optics"},
	} {
		if runs[i].Kind() != want.kind || runs[i].Text() != want.text || runs[i].Target() != want.target {
			t.Errorf("run %d = %s %q %q, want %s %q %q",
				i, runs[i].Kind(), runs[i].Text(), runs[i].Target(), want.kind, want.text, want.target)
		}
		if !runs[i].Origin().Located() {
			t.Errorf("run %d has no origin", i)
		}
	}
	items := document.Content()[1].Items()
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for i, want := range []string{"mount", "optics"} {
		runs := items[i].Runs()
		if len(runs) != 1 || runs[0].Kind() != RunPlain || runs[0].Text() != want {
			t.Errorf("item %d runs = %+v, want one plain %q", i, runs, want)
		}
	}
}

// TestEvaluateColumnRunErrors locks the typed runtime diagnostics: unknown
// columns, bad row styles, and bad link targets name query, column and row.
func TestEvaluateColumnRunErrors(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		kind   ErrorKind
		column string
		row    int
	}{
		{
			name:   "unprojected column",
			kind:   ErrorUnknownRunColumn,
			column: "missing",
			body: `
				calc def Chosen :> Query {
					in root : Element;
					in props : String[0..*] ordered;
					Project(source = OwnedElements(source = root), properties = props)
				}
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Chosen {
							in root = telescope;
							in props = ("name");
						}
						part plainName : SpanColumn {
							attribute redefines column = "missing";
						}
					}
				}`,
		},
		{
			name:   "invalid row style",
			kind:   ErrorInvalidRunStyle,
			column: "zone",
			row:    1,
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Styled {
							in root = telescope;
						}
						part zoned : SpanColumn {
							attribute redefines column = "name";
							attribute redefines styleColumn = "zone";
						}
					}
				}`,
		},
		{
			name:   "non-string row style",
			kind:   ErrorInvalidRunStyle,
			column: "mass",
			row:    1,
			body: `
				calc def Massed :> Query {
					in root : Element;
					Project(
						source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
						properties = ("name", "mass")
					)
				}
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Massed {
							in root = telescope;
						}
						part weighed : SpanColumn {
							attribute redefines column = "name";
							attribute redefines styleColumn = "mass";
						}
					}
				}`,
		},
		{
			name:   "missing link target",
			kind:   ErrorInvalidRunTarget,
			column: "caption",
			row:    1,
			body: `
				calc def Captioned :> Query {
					in root : Element;
					Project(
						source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
						properties = ("name", "caption")
					)
				}
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Captioned {
							in root = telescope;
						}
						part linked : LinkColumn {
							attribute redefines column = "name";
							attribute redefines targetColumn = "caption";
						}
					}
				}`,
		},
		{
			name:   "non-string link target",
			kind:   ErrorInvalidRunTarget,
			column: "mass",
			row:    1,
			body: `
				calc def Massed :> Query {
					in root : Element;
					Project(
						source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
						properties = ("name", "mass")
					)
				}
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Massed {
							in root = telescope;
						}
						part linked : LinkColumn {
							attribute redefines column = "name";
							attribute redefines targetColumn = "mass";
						}
					}
				}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fixture := loadEvaluationFixture(t, columnRunModel+c.body)
			_, err := fixture.evaluate(t, "Report")
			var evaluation *Error
			if !errors.As(err, &evaluation) {
				t.Fatalf("err = %v, want *docir.Error", err)
			}
			if evaluation.Kind != c.kind || evaluation.Column != c.column || evaluation.Row != c.row {
				t.Fatalf("error = %+v, want kind %s column %q row %d", evaluation, c.kind, c.column, c.row)
			}
			if evaluation.Query == "" || !evaluation.Origin.Located() {
				t.Fatalf("error lacks query or origin: %+v", evaluation)
			}
		})
	}
}
