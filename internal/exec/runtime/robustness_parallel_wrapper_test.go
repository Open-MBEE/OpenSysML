package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestRuntimeRobustnessParallelWrapperLeavesInnerRegionInactive(t *testing.T) {
	_, visits, err := executeStateSource(t, "Machine", `
		package test {
			state Machine {
				entry; then S;
				state S parallel {
					state R {
						state A;
					}
				}
				state finished;
				transition first S then finished;
			}
		}
	`)
	if err != nil {
		t.Fatalf("execute parallel state with an unstarted stand-in: %v", err)
	}
	if slices.Contains(visits, "A") {
		t.Errorf("the unstarted body state A was entered: %v", visits)
	}
	if slices.Contains(visits, "finished") {
		t.Errorf("S completed while its stand-in body was unstarted: %v", visits)
	}
}

func TestRuntimeRobustnessParallelWrapperRejectsUnresolvedMetadata(t *testing.T) {
	src := `
		package test {
			state def Machine {
				entry; then S;
				state S parallel {
					@NoSuchMetadata;
					state R {
						entry; then A;
						state A;
					}
				}
			}
		}
	`
	root := parseAndBuild(t, src)
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", root)
	idx.ExpandWildcardImports()
	diagnostics := passes.Analyze("<test>", root, nil, idx)
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "NoSuchMetadata") {
			return
		}
	}
	t.Fatalf("unresolved parallel-state annotation produced no diagnostic: %v", diagnostics)
}
