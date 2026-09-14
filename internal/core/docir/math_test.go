package docir

import (
	"errors"
	"testing"
)

const mathModel = `
	part def Sub {
		attribute latex : String;
	}
	part telescope {
		part optics : Sub {
			attribute redefines latex = "f = \\frac{D}{N}";
		}
		part mount : Sub {
			attribute redefines latex = "  ";
		}
	}
	calc def Symbolic :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "latex")
		)
	}
`

// TestEvaluateMath locks the math run kind and the formula node: inline
// LaTeX keeps its source verbatim, a formula carries source, caption and
// the anchor a reference gives it.
func TestEvaluateMath(t *testing.T) {
	fixture := loadEvaluationFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part lead : Span {
					attribute redefines text = "Einstein found ";
				}
				part inline : Span {
					attribute redefines text = "E = mc^2";
					attribute redefines style = "math";
				}
				part see : Ref {
					ref redefines target = energy;
				}
			}
			part energy : Formula {
				attribute redefines source = "\\int_0^\\infty e^{-x^2}\\,dx = \\frac{\\sqrt{\\pi}}{2}";
				attribute redefines caption = "Gaussian integral";
			}
		}
	`)
	document := fixture.mustEvaluate(t, "Report")
	content := document.Content()
	if len(content) != 2 {
		t.Fatalf("content = %d, want 2", len(content))
	}
	runs := content[0].Runs()
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(runs))
	}
	if runs[1].Kind() != RunMath || runs[1].Text() != "E = mc^2" || !runs[1].Origin().Located() {
		t.Errorf("math run = %s %q located %v", runs[1].Kind(), runs[1].Text(), runs[1].Origin().Located())
	}
	if runs[2].Kind() != RunRef || runs[2].Target() != "energy" || runs[2].Text() != "Gaussian integral" {
		t.Errorf("ref run = %s %q -> %q", runs[2].Kind(), runs[2].Text(), runs[2].Target())
	}
	formula := content[1]
	if formula.Kind() != ContentFormula || formula.Name() != "energy" {
		t.Fatalf("formula = %s %q", formula.Kind(), formula.Name())
	}
	if formula.Source() != `\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt{\pi}}{2}` {
		t.Errorf("source = %q", formula.Source())
	}
	if formula.Caption() != "Gaussian integral" || formula.Anchor() != "energy" {
		t.Errorf("caption = %q anchor = %q", formula.Caption(), formula.Anchor())
	}
	if !formula.Origin().Located() {
		t.Error("formula has no origin")
	}
	if formula.Query() != "" || len(formula.Runs()) != 0 {
		t.Errorf("formula carries query %q or runs %d", formula.Query(), len(formula.Runs()))
	}
}

// TestEvaluateMathColumnMissingValue checks that a math column row with no
// value at all is the same typed error as a blank one, not a dropped formula.
func TestEvaluateMathColumnMissingValue(t *testing.T) {
	fixture := loadEvaluationFixture(t, `
		part def Sub {
			attribute latex : String;
		}
		part telescope {
			part optics : Sub {
				attribute redefines latex = "f = \\frac{D}{N}";
			}
			part stand : Sub;
		}
		calc def Symbolic :> Query {
			in root : Element;
			Project(
				source = OrderBy(
					source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
					property = "name",
					direction = "ascending",
					missing = "last",
					multiple = "error"
				),
				properties = ("name", "latex")
			)
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part fixed : Paragraph {
				calc rows : Symbolic {
					in root = telescope;
				}
				part expr : SpanColumn {
					attribute redefines column = "latex";
					attribute redefines style = "math";
				}
			}
		}
	`)
	_, err := fixture.evaluate(t, "Report")
	var evaluation *Error
	if !errors.As(err, &evaluation) {
		t.Fatalf("err = %v, want *docir.Error", err)
	}
	if evaluation.Kind != ErrorBlankMath || evaluation.Column != "latex" || evaluation.Row != 2 {
		t.Fatalf("error = %+v, want blank-math on column latex row 2", evaluation)
	}
	if evaluation.Query == "" || !evaluation.Origin.Located() {
		t.Fatalf("error lacks query or origin: %+v", evaluation)
	}
}

// TestEvaluateMathColumnRuns checks query-backed math: a fixed math style
// and a row-driven "math" style both yield math runs with verbatim LaTeX,
// while a blank LaTeX cell is a typed error naming query, column and row.
func TestEvaluateMathColumnRuns(t *testing.T) {
	fixture := loadEvaluationFixture(t, mathModel+`
		calc def Styled :> Query {
			in root : Element;
			Project(
				source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
				properties = ("name", "latex"),
				columns = (Column(name = "style", expression = "math"))
			)
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part fixed : Paragraph {
				calc rows : Symbolic {
					in root = telescope;
				}
				part expr : SpanColumn {
					attribute redefines column = "latex";
					attribute redefines style = "math";
				}
			}
		}
		part def Rows :> Document {
			attribute redefines title = "Rows";
			part driven : Paragraph {
				calc rows : Styled {
					in root = telescope;
				}
				part expr : SpanColumn {
					attribute redefines column = "name";
					attribute redefines styleColumn = "style";
				}
			}
		}
	`)
	_, err := fixture.evaluate(t, "Report")
	var evaluation *Error
	if !errors.As(err, &evaluation) {
		t.Fatalf("err = %v, want *docir.Error", err)
	}
	if evaluation.Kind != ErrorBlankMath || evaluation.Column != "latex" || evaluation.Row != 1 {
		t.Fatalf("error = %+v, want blank-math on column latex row 1", evaluation)
	}
	if evaluation.Query == "" || !evaluation.Origin.Located() {
		t.Fatalf("error lacks query or origin: %+v", evaluation)
	}

	document := fixture.mustEvaluate(t, "Rows")
	runs := document.Content()[0].Runs()
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	for i, run := range runs {
		if run.Kind() != RunMath {
			t.Errorf("run %d kind = %s, want math", i, run.Kind())
		}
	}
}

// TestEvaluatedFormulaIsCloned checks a formula's source survives the
// content copy every accessor hands out.
func TestEvaluatedFormulaIsCloned(t *testing.T) {
	fixture := loadEvaluationFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part energy : Formula {
				attribute redefines source = "E = mc^2";
			}
		}
	`)
	document := fixture.mustEvaluate(t, "Report")
	content := document.Content()
	content[0] = Content{}
	if got := document.Content()[0].Source(); got != "E = mc^2" {
		t.Fatalf("source after mutation = %q", got)
	}
}
