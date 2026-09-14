package docplan

import (
	"strings"
	"testing"
)

// TestCompileMathSpan locks the inline math style: a span styled "math"
// keeps its LaTeX source verbatim, backslashes and braces included.
func TestCompileMathSpan(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part physics : Paragraph {
				part lead : Span {
					attribute redefines text = "Energy is";
				}
				part energy : Span {
					attribute redefines text = "E = mc^2";
					attribute redefines style = "math";
				}
				part sum : Span {
					attribute redefines text = "\\sum_{i=1}^{n} x_i";
					attribute redefines style = "math";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].Runs()
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(runs))
	}
	for i, want := range []struct {
		style RunStyle
		text  string
	}{
		{StylePlain, "Energy is"},
		{StyleMath, "E = mc^2"},
		{StyleMath, `\sum_{i=1}^{n} x_i`},
	} {
		if runs[i].Kind() != RunSpan || runs[i].Style() != want.style || runs[i].Text() != want.text {
			t.Errorf("run %d = %s %q %q, want span %q %q",
				i, runs[i].Kind(), runs[i].Style(), runs[i].Text(), want.style, want.text)
		}
		if !runs[i].Origin().Located() {
			t.Errorf("run %d has no origin", i)
		}
	}
}

// TestCompileMathSpanRejectsBlankSource checks that whitespace-only LaTeX
// is a missing run text, since it would typeset nothing.
func TestCompileMathSpanRejectsBlankSource(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part physics : Paragraph {
				part energy : Span {
					attribute redefines text = "   ";
					attribute redefines style = "math";
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingRunText || planning.Content != "Observatory::Report::physics::energy" {
		t.Fatalf("error = %+v", planning)
	}
}

// TestInvalidRunStyleNamesMath checks the style diagnostic lists "math"
// among the accepted styles.
func TestInvalidRunStyleNamesMath(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part physics : Paragraph {
				part energy : Span {
					attribute redefines text = "E";
					attribute redefines style = "maths";
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidRunStyle || planning.Actual != "maths" {
		t.Fatalf("error = %+v", planning)
	}
	if msg := planning.Error(); !strings.Contains(msg, `"code" or "math", got "maths"`) {
		t.Fatalf("message = %q", msg)
	}
}

// TestCompileMathSpanColumn checks a query-backed paragraph may style a
// projected column as math.
func TestCompileMathSpanColumn(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Symbols :> Query {
			in root : Element;
			Project(source = OwnedElements(source = root), properties = ("name", "latex"))
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part formulas : Paragraph {
				calc rows : Symbols {
					in root = telescope;
				}
				part expr : SpanColumn {
					attribute redefines column = "latex";
					attribute redefines style = "math";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].ColumnRuns()
	if len(runs) != 1 || runs[0].Kind() != TemplateSpan || runs[0].Style() != StyleMath {
		t.Fatalf("column runs = %+v", runs)
	}
}

// TestCompileFormula locks the display formula block: LaTeX source and an
// optional caption, referenceable by name.
func TestCompileFormula(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = energy;
				}
			}
			part energy : Formula {
				attribute redefines source = "E = mc^2";
				attribute redefines caption = "Mass-energy equivalence";
			}
			part bare : Formula {
				attribute redefines source = "\\frac{a}{b}";
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if len(content) != 3 {
		t.Fatalf("content = %d nodes, want 3", len(content))
	}
	energy := content[1]
	if energy.Kind() != ContentFormula || energy.Name() != "energy" {
		t.Fatalf("energy = %s %q", energy.Kind(), energy.Name())
	}
	if energy.Source() != "E = mc^2" || energy.Caption() != "Mass-energy equivalence" {
		t.Fatalf("energy source = %q caption = %q", energy.Source(), energy.Caption())
	}
	if !energy.Origin().Located() {
		t.Fatalf("energy origin = %+v", energy.Origin())
	}
	bare := content[2]
	if bare.Kind() != ContentFormula || bare.Source() != `\frac{a}{b}` || bare.Caption() != "" {
		t.Fatalf("bare = %s %q %q", bare.Kind(), bare.Source(), bare.Caption())
	}
	see := content[0].Runs()[0]
	if see.Kind() != RunRef || see.Text() != "Mass-energy equivalence" || strings.Join(see.RefPath(), "/") != "energy" {
		t.Fatalf("ref = %s %q %v", see.Kind(), see.Text(), see.RefPath())
	}
}

// TestCompileFormulaIsCloned checks the source survives the plan's
// defensive copy.
func TestCompileFormulaIsCloned(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part energy : Formula {
				attribute redefines source = "E = mc^2";
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	if got := plan.Content()[0].Source(); got != "E = mc^2" {
		t.Fatalf("source = %q", got)
	}
}

// TestCompileFormulaErrors checks the typed failures of a formula block.
func TestCompileFormulaErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		kind    ErrorKind
		content string
	}{
		{
			name: "missing source",
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part energy : Formula;
				}
			`,
			kind:    ErrorMissingFormulaSource,
			content: "Observatory::Report::energy",
		},
		{
			name: "blank source",
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part energy : Formula {
						attribute redefines source = " ";
					}
				}
			`,
			kind:    ErrorMissingFormulaSource,
			content: "Observatory::Report::energy",
		},
		{
			name: "non-literal source",
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part energy : Formula {
						attribute redefines source = 1 + 2;
					}
				}
			`,
			kind:    ErrorInvalidAttribute,
			content: "Observatory::Report::energy",
		},
		{
			name: "query in formula",
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part energy : Formula {
						attribute redefines source = "E";
						calc rows : OwnedElements {
							in redefines source = Report;
						}
					}
				}
			`,
			kind:    ErrorInvalidContent,
			content: "Observatory::Report::energy::rows",
		},
		{
			name: "runs in formula",
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part energy : Formula {
						attribute redefines source = "E";
						part lead : Span {
							attribute redefines text = "x";
						}
					}
				}
			`,
			kind:    ErrorInvalidContent,
			content: "Observatory::Report::energy::lead",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, tc.body)
			_, err := fixture.compile(t, "Report")
			planning := planningError(t, err)
			if planning.Kind != tc.kind || planning.Content != tc.content {
				t.Fatalf("error = %+v, want %s on %s", planning, tc.kind, tc.content)
			}
			if !planning.Origin.Located() {
				t.Fatalf("error has no origin: %+v", planning)
			}
		})
	}
}
