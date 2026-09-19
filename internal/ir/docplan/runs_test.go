package docplan

import (
	"strings"
	"testing"
)

// TestCompileParagraphRuns locks the happy path of inline runs: styled spans,
// links, and references compile in declaration order with their attributes.
func TestCompileParagraphRuns(t *testing.T) {
	fixture := loadPlanningFixture(t, `
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
					ref redefines target = details;
				}
				part named : Ref {
					attribute redefines text = "custom label";
					ref redefines target = details;
				}
			}
			part details : Section {
				attribute redefines title = "Details";
				part body : Paragraph {
					attribute redefines text = "body";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].Runs()
	if len(runs) != 7 {
		t.Fatalf("runs = %d, want 7", len(runs))
	}
	for i, want := range []struct {
		kind  RunKind
		style RunStyle
		text  string
	}{
		{RunSpan, StylePlain, "plain"},
		{RunSpan, StyleEmphasis, "emphasized"},
		{RunSpan, StyleStrong, "bolded"},
		{RunSpan, StyleCode, "x > 1"},
		{RunLink, "", "the spec"},
		{RunRef, "", "Details"},
		{RunRef, "", "custom label"},
	} {
		if runs[i].Kind() != want.kind || runs[i].Style() != want.style || runs[i].Text() != want.text {
			t.Errorf("run %d = %s %q %q, want %s %q %q",
				i, runs[i].Kind(), runs[i].Style(), runs[i].Text(), want.kind, want.style, want.text)
		}
		if !runs[i].Origin().Located() {
			t.Errorf("run %d has no origin", i)
		}
	}
	if runs[4].Target() != "https://example.com/spec" {
		t.Errorf("link target = %q", runs[4].Target())
	}
	for _, i := range []int{5, 6} {
		if path := strings.Join(runs[i].RefPath(), "/"); path != "details" {
			t.Errorf("ref %d path = %q", i, path)
		}
	}
}

// TestCompiledRunsAreImmutable checks the defensive copies around a plan's
// inline runs and reference paths.
func TestCompiledRunsAreImmutable(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part formatted : Paragraph {
				part see : Ref {
					ref redefines target = details;
				}
			}
			part details : Section {
				attribute redefines title = "Details";
				part body : Paragraph {
					attribute redefines text = "body";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].Runs()
	runs[0] = Run{}
	if plan.Content()[0].Runs()[0].Kind() != RunRef {
		t.Fatal("mutating returned runs changed the plan")
	}
	path := plan.Content()[0].Runs()[0].RefPath()
	path[0] = "mutated"
	if plan.Content()[0].Runs()[0].RefPath()[0] != "details" {
		t.Fatal("mutating returned ref path changed the plan")
	}
}

// TestCompileRefToRedefinedContent checks that an inherited reference binds to
// the derived document's redefinition of its target.
func TestCompileRefToRedefinedContent(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Base :> Document {
			attribute redefines title = "Base";
			part formatted : Paragraph {
				part see : Ref {
					ref redefines target = details;
				}
			}
			part details : Section {
				attribute redefines title = "Details";
				part body : Paragraph {
					attribute redefines text = "body";
				}
			}
		}
		part def Report :> Base {
			attribute redefines title = "Derived";
			part redefines details : Section {
				attribute redefines title = "Replaced";
				part body : Paragraph {
					attribute redefines text = "new body";
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	var runs []Run
	for _, node := range plan.Content() {
		if node.Name() == "formatted" {
			runs = node.Runs()
		}
	}
	if len(runs) != 1 || runs[0].Kind() != RunRef {
		t.Fatalf("runs = %+v", runs)
	}
	if runs[0].Text() != "Replaced" {
		t.Errorf("ref label = %q, want %q", runs[0].Text(), "Replaced")
	}
	if path := strings.Join(runs[0].RefPath(), "/"); path != "details" {
		t.Errorf("ref path = %q", path)
	}
}

// TestCompileGroupedTable locks a grouped table plan: the group column is
// checked against the query's statically-known projection.
func TestCompileGroupedTable(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Zoned :> Query {
			in root : Element;
			Project(
				source = OwnedElements(source = root),
				properties = ("zone", "name")
			)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part zones : Table {
				attribute redefines groupBy = "zone";
				calc rows : Zoned {
					in root = telescope;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	if got := plan.Content()[0].GroupBy(); got != "zone" {
		t.Fatalf("groupBy = %q", got)
	}
}

// TestCompileGroupedTableSeesComputedColumns locks that static projection
// inspection recognizes computed Column names for groupBy validation.
func TestCompileGroupedTableSeesComputedColumns(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Subsystem {
			attribute mass : Real;
		}
		calc def Margins :> Query {
			in root : Element;
			Project(
				source = OwnedElements(source = root),
				properties = ("name"),
				columns = (Column(name = "label", expression = "sub: " + Element::name))
			)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part labels : Table {
				attribute redefines groupBy = "label";
				calc rows : Margins {
					in root = telescope;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	if got := plan.Content()[0].GroupBy(); got != "label" {
		t.Fatalf("groupBy = %q", got)
	}
}

// TestCompileGroupedTableSeesRelatedColumns locks that static projection
// inspection recognizes RelatedColumn names for groupBy validation.
func TestCompileGroupedTableSeesRelatedColumns(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Coverage :> Query {
			in root : Element;
			Project(
				source = OwnedElements(source = root),
				properties = ("name"),
				columns = (RelatedColumn("verified", "verification", "incoming", 1, "any"))
			)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part coverage : Table {
				attribute redefines groupBy = "verified";
				calc rows : Coverage {
					in root = telescope;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	if got := plan.Content()[0].GroupBy(); got != "verified" {
		t.Fatalf("groupBy = %q", got)
	}
}

// TestCompileReportsRunAndGroupErrors locks the typed diagnostics for every
// malformed inline-run and grouping form.
func TestCompileReportsRunAndGroupErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		kind ErrorKind
	}{
		{
			name: "runs conflict with text",
			kind: ErrorConflictingRuns,
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part both : Paragraph {
						attribute redefines text = "text";
						part lead : Span {
							attribute redefines text = "plain";
						}
					}
				}`,
		},
		{
			name: "runs conflict with query",
			kind: ErrorConflictingRuns,
			body: `
				calc def Names :> Query {
					in root : Element;
					OwnedElements(source = root)
				}
				part telescope;
				part def Report :> Document {
					attribute redefines title = "Report";
					part both : Paragraph {
						part lead : Span {
							attribute redefines text = "plain";
						}
						calc names : Names {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name: "ambiguous run kind",
			kind: ErrorAmbiguousRun,
			body: `
				part def Both :> Span, Link;
				part def Report :> Document {
					attribute redefines title = "Report";
					part formatted : Paragraph {
						part run : Both {
							attribute redefines text = "text";
							attribute redefines target = "https://example.com";
						}
					}
				}`,
		},
		{
			name: "span without text",
			kind: ErrorMissingRunText,
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part formatted : Paragraph {
						part empty : Span;
					}
				}`,
		},
		{
			name: "invalid span style",
			kind: ErrorInvalidRunStyle,
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part formatted : Paragraph {
						part styled : Span {
							attribute redefines text = "text";
							attribute redefines style = "underline";
						}
					}
				}`,
		},
		{
			name: "link without target",
			kind: ErrorMissingLinkTarget,
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part formatted : Paragraph {
						part dangling : Link {
							attribute redefines text = "text";
						}
					}
				}`,
		},
		{
			name: "ref without target",
			kind: ErrorMissingRefTarget,
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part formatted : Paragraph {
						part dangling : Ref {
							attribute redefines text = "text";
						}
					}
				}`,
		},
		{
			name: "ref to non-content element",
			kind: ErrorInvalidRefTarget,
			body: `
				part telescope;
				part def Report :> Document {
					attribute redefines title = "Report";
					part formatted : Paragraph {
						part outside : Ref {
							ref redefines target = telescope;
						}
					}
				}`,
		},
		{
			name: "group column not projected",
			kind: ErrorUnknownGroupColumn,
			body: `
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
					part zones : Table {
						attribute redefines groupBy = "zone";
						calc rows : Named {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name: "empty group column",
			kind: ErrorUnknownGroupColumn,
			body: `
				calc def Named :> Query {
					in root : Element;
					OwnedElements(source = root)
				}
				part telescope;
				part def Report :> Document {
					attribute redefines title = "Report";
					part zones : Table {
						attribute redefines groupBy = "";
						calc rows : Named {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name: "query inside a run",
			kind: ErrorInvalidContent,
			body: `
				calc def Names :> Query {
					in root : Element;
					OwnedElements(source = root)
				}
				part telescope;
				part def Report :> Document {
					attribute redefines title = "Report";
					part formatted : Paragraph {
						part lead : Span {
							attribute redefines text = "text";
							calc names : Names {
								in root = telescope;
							}
						}
					}
				}`,
		},
		{
			name: "run outside a paragraph",
			kind: ErrorInvalidContent,
			body: `
				part def Report :> Document {
					attribute redefines title = "Report";
					part misplaced : Section {
						attribute redefines title = "Section";
						part lead : Span {
							attribute redefines text = "text";
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
