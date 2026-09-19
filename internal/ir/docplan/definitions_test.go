package docplan

import "testing"

// definitionsQuery projects the columns a definitions block names.
const definitionsQuery = `
	calc def Documented :> Query {
		in root : Element;
		Project(
			source = OwnedElements(source = root),
			properties = ("shortName", "name", "documentation")
		)
	}
	part telescope {
		part <'M1'> mirror {
			doc /* The primary mirror. */
		}
	}
`

// TestCompileDefinitions locks the plan a definitions block compiles to: its
// term and description column names and its bound query.
func TestCompileDefinitions(t *testing.T) {
	fixture := loadPlanningFixture(t, definitionsQuery+`
		part def Report :> Document {
			attribute redefines title = "Report";
			part glossary : Definitions {
				attribute redefines term = "shortName";
				attribute redefines description = "documentation";
				calc rows : Documented {
					in root = telescope;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	if len(plan.Content()) != 1 {
		t.Fatalf("content = %d blocks, want 1", len(plan.Content()))
	}
	content := plan.Content()[0]
	if content.Kind() != ContentDefinitions {
		t.Fatalf("kind = %s, want %s", content.Kind(), ContentDefinitions)
	}
	if content.Name() != "glossary" {
		t.Fatalf("name = %q, want glossary", content.Name())
	}
	if content.Term() != "shortName" || content.Description() != "documentation" {
		t.Fatalf("columns = %q / %q, want shortName / documentation", content.Term(), content.Description())
	}
	query := content.Query()
	if query == nil || query.Entry() != "Observatory::Documented" {
		t.Fatalf("query = %+v, want Observatory::Documented", query)
	}
	if !content.Origin().Located() {
		t.Fatalf("content has no origin: %+v", content)
	}
}

// TestCompileDefinitionsInheritsColumns locks that a specialised definitions
// kind may fix its columns once and let each block bind only a query.
func TestCompileDefinitionsInheritsColumns(t *testing.T) {
	fixture := loadPlanningFixture(t, definitionsQuery+`
		part def Glossary :> Definitions {
			attribute redefines term = "name";
			attribute redefines description = "documentation";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part terms : Glossary {
				calc rows : Documented {
					in root = telescope;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()[0]
	if content.Term() != "name" || content.Description() != "documentation" {
		t.Fatalf("columns = %q / %q, want name / documentation", content.Term(), content.Description())
	}
}

// TestCompileDefinitionsDefersDynamicColumns locks that a projection whose
// properties are parameter-driven is accepted at plan time; execution reports
// a column the rows lack.
func TestCompileDefinitionsDefersDynamicColumns(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Dynamic :> Query {
			in root : Element;
			in properties : String[1..*];
			Project(source = OwnedElements(source = root), properties = properties)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part glossary : Definitions {
				attribute redefines term = "anything";
				attribute redefines description = "whatever";
				calc rows : Dynamic {
					in root = telescope;
					in properties = ("anything", "whatever");
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()[0]
	if content.Term() != "anything" || content.Description() != "whatever" {
		t.Fatalf("columns = %q / %q", content.Term(), content.Description())
	}
}

// TestCompileReportsDefinitionsErrors locks the typed diagnostics for every
// malformed definitions block.
func TestCompileReportsDefinitionsErrors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		kind      ErrorKind
		parameter string
		actual    string
	}{
		{
			name: "no query",
			kind: ErrorMissingQuery,
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = "shortName";
						attribute redefines description = "documentation";
					}
				}`,
		},
		{
			name:      "no term",
			kind:      ErrorMissingDefinitionColumn,
			parameter: "term",
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines description = "documentation";
						calc rows : Documented {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name:      "empty term",
			kind:      ErrorMissingDefinitionColumn,
			parameter: "term",
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = "";
						attribute redefines description = "documentation";
						calc rows : Documented {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name:      "no description",
			kind:      ErrorMissingDefinitionColumn,
			parameter: "description",
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = "shortName";
						calc rows : Documented {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name:      "unknown term column",
			kind:      ErrorUnknownDefinitionColumn,
			parameter: "term",
			actual:    "id",
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = "id";
						attribute redefines description = "documentation";
						calc rows : Documented {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name:      "unknown description column",
			kind:      ErrorUnknownDefinitionColumn,
			parameter: "description",
			actual:    "text",
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = "shortName";
						attribute redefines description = "text";
						calc rows : Documented {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name: "non-literal term",
			kind: ErrorInvalidAttribute,
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = 1;
						attribute redefines description = "documentation";
						calc rows : Documented {
							in root = telescope;
						}
					}
				}`,
		},
		{
			name: "nested content",
			kind: ErrorInvalidContent,
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = "shortName";
						attribute redefines description = "documentation";
						calc rows : Documented {
							in root = telescope;
						}
						part note : Paragraph {
							attribute redefines text = "nested";
						}
					}
				}`,
		},
		{
			name: "column run inside definitions",
			kind: ErrorInvalidContent,
			body: definitionsQuery + `
				part def Report :> Document {
					attribute redefines title = "Report";
					part glossary : Definitions {
						attribute redefines term = "shortName";
						attribute redefines description = "documentation";
						calc rows : Documented {
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
				t.Fatalf("kind = %s, want %s (%v)", planning.Kind, c.kind, err)
			}
			if c.parameter != "" && planning.Parameter != c.parameter {
				t.Fatalf("parameter = %q, want %q", planning.Parameter, c.parameter)
			}
			if c.actual != "" && planning.Actual != c.actual {
				t.Fatalf("actual = %q, want %q", planning.Actual, c.actual)
			}
			if c.kind == ErrorUnknownDefinitionColumn && planning.Query != "Observatory::Documented" {
				t.Fatalf("query = %q, want Observatory::Documented", planning.Query)
			}
			if !planning.Origin.Located() {
				t.Fatalf("error has no origin: %+v", planning)
			}
		})
	}
}
