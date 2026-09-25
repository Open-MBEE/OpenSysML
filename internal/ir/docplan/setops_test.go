package docplan

import "testing"

// TestCompileGroupedTableThroughSetOperations locks that WhereRelated, Except
// and Union carry their source projection into static column checks: a
// projected column survives them, and Union with matching inputs still knows it.
func TestCompileGroupedTableThroughSetOperations(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Named :> Query {
			in root : Element;
			Project(source = OwnedElements(source = root), properties = ("zone", "name"))
		}
		calc def Unsatisfied :> Query {
			in root : Element;
			WhereRelated(source = Named(root = root), relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, exists = false)
		}
		calc def Rest :> Query {
			in root : Element;
			Except(source = Named(root = root), exclude = Unsatisfied(root = root))
		}
		calc def Both :> Query {
			in root : Element;
			Union(source = Unsatisfied(root = root), other = Rest(root = root))
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part unsatisfied : Table {
				attribute redefines groupBy = "zone";
				calc rows : Unsatisfied { in root = telescope; }
			}
			part rest : Table {
				attribute redefines groupBy = "zone";
				calc rows : Rest { in root = telescope; }
			}
			part both : Table {
				attribute redefines groupBy = "zone";
				calc rows : Both { in root = telescope; }
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	for i, content := range plan.Content() {
		if got := content.GroupBy(); got != "zone" {
			t.Fatalf("content %d groupBy = %q", i, got)
		}
	}
}

// TestCompileSetOperationsRejectUnprojectedGroupColumn locks that a column the
// operations' source never projected is still refused.
func TestCompileSetOperationsRejectUnprojectedGroupColumn(t *testing.T) {
	cases := map[string]string{
		"where related": `WhereRelated(source = Named(root = root), relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1)`,
		"except":        `Except(source = Named(root = root), exclude = OwnedElements(source = root))`,
		"union":         `Union(source = Named(root = root), other = Named(root = root))`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				part telescope;
				calc def Named :> Query {
					in root : Element;
					Project(source = OwnedElements(source = root), properties = ("name"))
				}
				calc def Rows :> Query {
					in root : Element;
					`+body+`
				}
				part def Report :> Document {
					attribute redefines title = "Report";
					part zones : Table {
						attribute redefines groupBy = "zone";
						calc rows : Rows { in root = telescope; }
					}
				}
			`)
			_, err := fixture.compile(t, "Report")
			if planning := planningError(t, err); planning.Kind != ErrorUnknownGroupColumn {
				t.Fatalf("kind = %v, want %v", planning.Kind, ErrorUnknownGroupColumn)
			}
		})
	}
}

// TestCompileUnionOfDifferingColumnsIsLeftToExecution locks that a Union whose
// inputs project different columns plans: execution reports the mismatch.
func TestCompileUnionOfDifferingColumnsIsLeftToExecution(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Zoned :> Query {
			in root : Element;
			Project(source = OwnedElements(source = root), properties = ("zone"))
		}
		calc def Named :> Query {
			in root : Element;
			Project(source = OwnedElements(source = root), properties = ("name"))
		}
		calc def Rows :> Query {
			in root : Element;
			Union(source = Zoned(root = root), other = Named(root = root))
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part zones : Table {
				attribute redefines groupBy = "zone";
				calc rows : Rows { in root = telescope; }
			}
		}
	`)
	fixture.mustCompile(t, "Report")
}
