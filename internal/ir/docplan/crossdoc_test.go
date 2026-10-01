package docplan

import (
	"errors"
	"strings"
	"testing"
)

// crossDocumentFixture declares two documents where one references the
// other's content block and root.
const crossDocumentFixture = `
	part def Appendix :> Document {
		attribute redefines title = "Appendix";
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part body : Paragraph {
				attribute redefines text = "detail";
			}
		}
	}
	part def Report :> Document {
		attribute redefines title = "Report";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = Appendix::tables;
			}
			part whole : Ref {
				ref redefines target = Appendix;
			}
			part named : Ref {
				attribute redefines text = "the appendix";
				ref redefines target = Appendix;
			}
		}
	}
`

// TestCompileCrossDocumentRefs locks planning-time resolution of references
// into another document: content blocks carry the target document and its
// named path, root references the document alone.
func TestCompileCrossDocumentRefs(t *testing.T) {
	fixture := loadPlanningFixture(t, crossDocumentFixture)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].Runs()
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(runs))
	}
	for i, want := range []struct {
		document string
		path     string
		text     string
	}{
		{"Observatory::Appendix", "tables", "Detail Tables"},
		{"Observatory::Appendix", "", "Appendix"},
		{"Observatory::Appendix", "", "the appendix"},
	} {
		if runs[i].Kind() != RunRef {
			t.Fatalf("run %d kind = %s", i, runs[i].Kind())
		}
		if runs[i].RefDocument() != want.document {
			t.Errorf("run %d document = %q, want %q", i, runs[i].RefDocument(), want.document)
		}
		if path := strings.Join(runs[i].RefPath(), "/"); path != want.path {
			t.Errorf("run %d path = %q, want %q", i, path, want.path)
		}
		if runs[i].Text() != want.text {
			t.Errorf("run %d text = %q, want %q", i, runs[i].Text(), want.text)
		}
		if !runs[i].Origin().Located() {
			t.Errorf("run %d has no origin", i)
		}
	}
}

// TestCompileSameDocumentRefStaysLocal checks a reference within one document
// carries no target document.
func TestCompileSameDocumentRefStaysLocal(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
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
	run := plan.Content()[0].Runs()[0]
	if run.RefDocument() != "" {
		t.Errorf("document = %q, want \"\"", run.RefDocument())
	}
	if strings.Join(run.RefPath(), "/") != "details" {
		t.Errorf("path = %q", strings.Join(run.RefPath(), "/"))
	}
}

// TestCompileCrossDocumentRefToNestedBlock checks a reference into a nested
// section of another document carries the full named path.
func TestCompileCrossDocumentRefToNestedBlock(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Appendix :> Document {
			attribute redefines title = "Appendix";
			part tables : Section {
				attribute redefines title = "Detail Tables";
				part masses : Section {
					attribute redefines title = "Masses";
					part body : Paragraph {
						attribute redefines text = "detail";
					}
				}
			}
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = Appendix::tables::masses;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	run := plan.Content()[0].Runs()[0]
	if run.RefDocument() != "Observatory::Appendix" {
		t.Errorf("document = %q", run.RefDocument())
	}
	if path := strings.Join(run.RefPath(), "/"); path != "tables/masses" {
		t.Errorf("path = %q", path)
	}
	if run.Text() != "Masses" {
		t.Errorf("text = %q", run.Text())
	}
}

// TestCompileCrossDocumentRefErrors checks references to unknown names, to a
// document's own root and to members of a document that are not content
// blocks fail with typed, located errors.
func TestCompileCrossDocumentRefErrors(t *testing.T) {
	cases := []struct {
		name   string
		target string
		kind   ErrorKind
	}{
		{"unknown", "Appendix::missing", ErrorUnknownRefTarget},
		{"not content", "Appendix::tables::body::lead", ErrorInvalidRefTarget},
		{"own root", "Report", ErrorInvalidRefTarget},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				part def Appendix :> Document {
					attribute redefines title = "Appendix";
					part tables : Section {
						attribute redefines title = "Detail Tables";
						part body : Paragraph {
							part lead : Span {
								attribute redefines text = "detail";
							}
						}
					}
				}
				part def Report :> Document {
					attribute redefines title = "Report";
					part intro : Paragraph {
						part see : Ref {
							ref redefines target = `+tc.target+`;
						}
					}
				}
			`)
			_, err := fixture.compile(t, "Report")
			var planErr *Error
			if !errors.As(err, &planErr) || planErr.Kind != tc.kind {
				t.Fatalf("error = %v, want kind %s", err, tc.kind)
			}
			if !planErr.Origin.Located() {
				t.Error("error has no origin")
			}
		})
	}
}

// TestCompileRefToElement checks a reference to a model element outside every
// document resolves to the element, labelled by its name when the run states
// no text, with neither a content path nor a document.
func TestCompileRefToElement(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Mount;
		part telescope;
		part Loose : Paragraph {
			attribute redefines text = "unowned";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = telescope;
				}
				part kind : Ref {
					ref redefines target = Mount.metadata;
				}
				part named : Ref {
					attribute redefines text = "the loose paragraph";
					ref redefines target = Loose;
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
		element, text string
		defaulted     bool
	}{
		{"Observatory::telescope", "telescope", true},
		{"Observatory::Mount", "Mount", true},
		{"Observatory::Loose", "the loose paragraph", false},
	} {
		run := runs[i]
		if run.RefElement() != want.element || run.Text() != want.text || run.TextDefaulted() != want.defaulted {
			t.Errorf("run %d = element %q, text %q, defaulted %v; want %q, %q, %v",
				i, run.RefElement(), run.Text(), run.TextDefaulted(), want.element, want.text, want.defaulted)
		}
		if len(run.RefPath()) != 0 || run.RefDocument() != "" {
			t.Errorf("run %d has path %v and document %q, want neither", i, run.RefPath(), run.RefDocument())
		}
	}
}

// TestCompileCrossDocumentRefThroughUsage checks the analysable authoring
// pattern: a document usage names the target document, and dot notation
// reaches its content blocks.
func TestCompileCrossDocumentRefThroughUsage(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		ref appendix : Appendix;
		part def Appendix :> Document {
			attribute redefines title = "Appendix";
			part tables : Section {
				attribute redefines title = "Detail Tables";
				part body : Paragraph {
					attribute redefines text = "detail";
				}
			}
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = appendix.tables;
				}
				part whole : Ref {
					ref redefines target = appendix;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].Runs()
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	if runs[0].RefDocument() != "Observatory::Appendix" {
		t.Errorf("content document = %q", runs[0].RefDocument())
	}
	if path := strings.Join(runs[0].RefPath(), "/"); path != "tables" {
		t.Errorf("content path = %q", path)
	}
	if runs[1].RefDocument() != "Observatory::Appendix" {
		t.Errorf("root document = %q", runs[1].RefDocument())
	}
	if len(runs[1].RefPath()) != 0 {
		t.Errorf("root path = %v, want none", runs[1].RefPath())
	}
	if runs[1].Text() != "Appendix" {
		t.Errorf("root text = %q", runs[1].Text())
	}
}

// TestCompileAmbiguousCrossDocumentRef checks a usage typed by more than one
// document definition is an ambiguous target, reported with its location.
func TestCompileAmbiguousCrossDocumentRef(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		ref both : Appendix, Glossary;
		part def Appendix :> Document {
			attribute redefines title = "Appendix";
		}
		part def Glossary :> Document {
			attribute redefines title = "Glossary";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = both;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	var planErr *Error
	if !errors.As(err, &planErr) || planErr.Kind != ErrorAmbiguousRefTarget {
		t.Fatalf("error = %v, want kind %s", err, ErrorAmbiguousRefTarget)
	}
	if !strings.Contains(planErr.Error(), "Observatory::Appendix and Observatory::Glossary") {
		t.Errorf("message = %q", planErr.Error())
	}
	if !planErr.Origin.Located() {
		t.Error("error has no origin")
	}
}

// TestCompileInheritedContentTargetsDerivedDocument checks a chain through a
// usage typed by a derived document links to that document, not to the base
// definition declaring the inherited block.
func TestCompileInheritedContentTargetsDerivedDocument(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		ref extra : DerivedAppendix;
		part def BaseAppendix :> Document {
			attribute redefines title = "Base Appendix";
			part tables : Section {
				attribute redefines title = "Detail Tables";
				part body : Paragraph {
					attribute redefines text = "detail";
				}
			}
		}
		part def DerivedAppendix :> BaseAppendix {
			attribute redefines title = "Derived Appendix";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = extra.tables;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	run := plan.Content()[0].Runs()[0]
	if run.RefDocument() != "Observatory::DerivedAppendix" {
		t.Errorf("document = %q, want Observatory::DerivedAppendix", run.RefDocument())
	}
	if path := strings.Join(run.RefPath(), "/"); path != "tables" {
		t.Errorf("path = %q", path)
	}
}

// TestCompileQualifiedInheritedContentTargetsNamedDocument checks a
// qualified target naming an inherited block through a derived document
// links to the document the author wrote, not the declaring base.
func TestCompileQualifiedInheritedContentTargetsNamedDocument(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def BaseAppendix :> Document {
			attribute redefines title = "Base Appendix";
			part tables : Section {
				attribute redefines title = "Detail Tables";
				part body : Paragraph {
					attribute redefines text = "detail";
				}
			}
		}
		part def DerivedAppendix :> BaseAppendix {
			attribute redefines title = "Derived Appendix";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = DerivedAppendix::tables;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	run := plan.Content()[0].Runs()[0]
	if run.RefDocument() != "Observatory::DerivedAppendix" {
		t.Errorf("document = %q, want Observatory::DerivedAppendix", run.RefDocument())
	}
	if path := strings.Join(run.RefPath(), "/"); path != "tables" {
		t.Errorf("path = %q", path)
	}
}

// TestCompileUntypedRedefinitionInheritsDocument checks references through an
// untyped redefining usage inherit the redefined usage's document type, for a
// document-root target and a chained content target alike.
func TestCompileUntypedRedefinitionInheritsDocument(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		ref appx : Appendix;
		ref extra :>> appx;
		part def Appendix :> Document {
			attribute redefines title = "Appendix";
			part tables : Section {
				attribute redefines title = "Detail Tables";
				part body : Paragraph {
					attribute redefines text = "detail";
				}
			}
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = extra.tables;
				}
				part whole : Ref {
					ref redefines target = extra;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].Runs()
	if runs[0].RefDocument() != "Observatory::Appendix" {
		t.Errorf("chain document = %q, want Observatory::Appendix", runs[0].RefDocument())
	}
	if path := strings.Join(runs[0].RefPath(), "/"); path != "tables" {
		t.Errorf("chain path = %q", path)
	}
	if runs[1].RefDocument() != "Observatory::Appendix" {
		t.Errorf("root document = %q, want Observatory::Appendix", runs[1].RefDocument())
	}
	if len(runs[1].RefPath()) != 0 {
		t.Errorf("root path = %v, want none", runs[1].RefPath())
	}
}

// TestCompileAmbiguousChainRoot checks a chain rooted in a usage typed by
// more than one document definition is an ambiguous target.
func TestCompileAmbiguousChainRoot(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		ref both : Appendix, Glossary;
		part def Appendix :> Document {
			attribute redefines title = "Appendix";
			part tables : Section {
				attribute redefines title = "Detail Tables";
				part body : Paragraph {
					attribute redefines text = "detail";
				}
			}
		}
		part def Glossary :> Appendix {
			attribute redefines title = "Glossary";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = both.tables;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	var planErr *Error
	if !errors.As(err, &planErr) || planErr.Kind != ErrorAmbiguousRefTarget {
		t.Fatalf("error = %v, want kind %s", err, ErrorAmbiguousRefTarget)
	}
	if !strings.Contains(planErr.Error(), "Observatory::Appendix and Observatory::Glossary") {
		t.Errorf("message = %q", planErr.Error())
	}
}

// TestCompileUnknownChainTarget checks a dot-notation target whose member
// does not exist is an unknown target naming the written chain.
func TestCompileUnknownChainTarget(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		ref appendix : Appendix;
		part def Appendix :> Document {
			attribute redefines title = "Appendix";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				part see : Ref {
					ref redefines target = appendix.missing;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	var planErr *Error
	if !errors.As(err, &planErr) || planErr.Kind != ErrorUnknownRefTarget {
		t.Fatalf("error = %v, want kind %s", err, ErrorUnknownRefTarget)
	}
	if !strings.Contains(planErr.Error(), "appendix.missing") {
		t.Errorf("message = %q", planErr.Error())
	}
	if !planErr.Origin.Located() {
		t.Error("error has no origin")
	}
}
