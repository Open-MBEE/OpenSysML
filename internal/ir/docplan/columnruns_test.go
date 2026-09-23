package docplan

import "testing"

// TestCompileColumnRuns locks the happy path of column runs: span and link
// templates compile in declaration order with their column mappings.
func TestCompileColumnRuns(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Zoned :> Query {
			in root : Element;
			Project(
				source = OwnedElements(source = root),
				properties = ("zone", "name"),
				columns = (Column(name = "url", expression = "https://example.com/" + Element::name))
			)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part styled : Paragraph {
				calc names : Zoned {
					in root = telescope;
				}
				part plainName : SpanColumn {
					attribute redefines column = "name";
				}
				part em : SpanColumn {
					attribute redefines column = "name";
					attribute redefines style = "emphasis";
				}
				part zoned : SpanColumn {
					attribute redefines column = "name";
					attribute redefines styleColumn = "zone";
				}
				part linked : LinkColumn {
					attribute redefines column = "name";
					attribute redefines targetColumn = "url";
				}
			}
			part items : List {
				calc names : Zoned {
					in root = telescope;
				}
				part codeName : SpanColumn {
					attribute redefines column = "name";
					attribute redefines style = "code";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].ColumnRuns()
	if len(runs) != 4 {
		t.Fatalf("column runs = %d, want 4", len(runs))
	}
	for i, want := range []struct {
		kind         TemplateKind
		column       string
		style        RunStyle
		styleColumn  string
		targetColumn string
	}{
		{TemplateSpan, "name", StylePlain, "", ""},
		{TemplateSpan, "name", StyleEmphasis, "", ""},
		{TemplateSpan, "name", "", "zone", ""},
		{TemplateLink, "name", "", "", "url"},
	} {
		got := runs[i]
		if got.Kind() != want.kind || got.Column() != want.column || got.Style() != want.style ||
			got.StyleColumn() != want.styleColumn || got.TargetColumn() != want.targetColumn {
			t.Errorf("column run %d = %+v, want %+v", i, got, want)
		}
		if !got.Origin().Located() {
			t.Errorf("column run %d has no origin", i)
		}
	}
	items := plan.Content()[1].ColumnRuns()
	if len(items) != 1 || items[0].Kind() != TemplateSpan || items[0].Style() != StyleCode {
		t.Fatalf("list column runs = %+v", items)
	}
}

// TestCompiledColumnRunsAreImmutable checks the defensive copy around a
// plan's column runs.
func TestCompiledColumnRunsAreImmutable(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Named :> Query {
			in root : Element;
			Project(
				source = OwnedElements(source = root),
				properties = ("name")
			)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part styled : Paragraph {
				calc names : Named {
					in root = telescope;
				}
				part plainName : SpanColumn {
					attribute redefines column = "name";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].ColumnRuns()
	runs[0] = ColumnRun{}
	if plan.Content()[0].ColumnRuns()[0].Column() != "name" {
		t.Fatal("mutating returned column runs changed the plan")
	}
}

// TestCompileColumnRunsSkipsUnknownableProjections locks that parameter-driven
// projections skip static column validation.
func TestCompileColumnRunsSkipsUnknownableProjections(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Chosen :> Query {
			in root : Element;
			in props : String[0..*] ordered;
			Project(
				source = OwnedElements(source = root),
				properties = props
			)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part styled : Paragraph {
				calc names : Chosen {
					in root = telescope;
					in props = ("name");
				}
				part anyName : SpanColumn {
					attribute redefines column = "anything";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	if runs := plan.Content()[0].ColumnRuns(); len(runs) != 1 || runs[0].Column() != "anything" {
		t.Fatalf("column runs = %+v", runs)
	}
}

// TestCompileReportsColumnRunErrors locks the typed diagnostics for every
// malformed column-run form.
func TestCompileReportsColumnRunErrors(t *testing.T) {
	const query = `
		calc def Named :> Query {
			in root : Element;
			Project(
				source = OwnedElements(source = root),
				properties = ("name")
			)
		}
		part telescope;
	`
	cases := []struct {
		name string
		body string
		kind ErrorKind
	}{
		{
			name: "column run without query",
			kind: ErrorColumnRunWithoutQuery,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						part plainName : SpanColumn {
							attribute redefines column = "name";
						}
					}
				}`,
		},
		{
			name: "column runs conflict with text",
			kind: ErrorConflictingColumnRuns,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						attribute redefines text = "text";
						calc names : Named {
							in root = telescope;
						}
						part plainName : SpanColumn {
							attribute redefines column = "name";
						}
					}
				}`,
		},
		{
			name: "column runs conflict with inline runs",
			kind: ErrorConflictingColumnRuns,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part lead : Span {
							attribute redefines text = "plain";
						}
						part plainName : SpanColumn {
							attribute redefines column = "name";
						}
					}
				}`,
		},
		{
			name: "span column without column",
			kind: ErrorMissingRunColumn,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part unnamed : SpanColumn;
					}
				}`,
		},
		{
			name: "link column without target column",
			kind: ErrorMissingRunColumn,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part dangling : LinkColumn {
							attribute redefines column = "name";
						}
					}
				}`,
		},
		{
			name: "empty style column",
			kind: ErrorMissingRunColumn,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part blank : SpanColumn {
							attribute redefines column = "name";
							attribute redefines styleColumn = "";
						}
					}
				}`,
		},
		{
			name: "style conflicts with style column",
			kind: ErrorConflictingRunStyle,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part both : SpanColumn {
							attribute redefines column = "name";
							attribute redefines style = "code";
							attribute redefines styleColumn = "name";
						}
					}
				}`,
		},
		{
			name: "invalid fixed style",
			kind: ErrorInvalidRunStyle,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part styledName : SpanColumn {
							attribute redefines column = "name";
							attribute redefines style = "underline";
						}
					}
				}`,
		},
		{
			name: "unknown text column",
			kind: ErrorUnknownRunColumn,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part plainName : SpanColumn {
							attribute redefines column = "zone";
						}
					}
				}`,
		},
		{
			name: "unknown style column",
			kind: ErrorUnknownRunColumn,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part styledName : SpanColumn {
							attribute redefines column = "name";
							attribute redefines styleColumn = "zone";
						}
					}
				}`,
		},
		{
			name: "unknown target column",
			kind: ErrorUnknownRunColumn,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part linked : LinkColumn {
							attribute redefines column = "name";
							attribute redefines targetColumn = "url";
						}
					}
				}`,
		},
		{
			name: "ambiguous column run kind",
			kind: ErrorAmbiguousRun,
			body: query + `
				part def Both :> SpanColumn, LinkColumn;
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part run : Both {
							attribute redefines column = "name";
							attribute redefines targetColumn = "name";
						}
					}
				}`,
		},
		{
			name: "bare column run kind",
			kind: ErrorInvalidContent,
			body: query + `
				part def Custom :> ColumnRun;
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part run : Custom;
					}
				}`,
		},
		{
			name: "query inside a column run",
			kind: ErrorInvalidContent,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part styled : Paragraph {
						calc names : Named {
							in root = telescope;
						}
						part plainName : SpanColumn {
							attribute redefines column = "name";
							calc extra : Named {
								in root = telescope;
							}
						}
					}
				}`,
		},
		{
			name: "column run outside a paragraph or list",
			kind: ErrorInvalidContent,
			body: query + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part rows : Table {
						calc rows : Named {
							in root = telescope;
						}
						part plainName : SpanColumn {
							attribute redefines column = "name";
						}
					}
				}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, c.body)
			_, err := fixture.compile(t, "Report")
			planning := planningError(t, err)
			if planning.Kind != c.kind {
				t.Fatalf("kind = %s, want %s", planning.Kind, c.kind)
			}
			if !planning.Origin.Located() {
				t.Fatalf("error has no origin: %+v", planning)
			}
		})
	}
}
