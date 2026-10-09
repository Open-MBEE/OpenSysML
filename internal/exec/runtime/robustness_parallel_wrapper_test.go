package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestRuntimeRobustnessParallelWrapperLeavesRegionInactive(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `
		package test {
			state def Machine {
				entry; then S;
				state S parallel {
					state R {
						state A;
					}
				}
			}
		}
	`)
	if err != nil {
		t.Fatalf("a wrapped region without an entry should remain inactive, not fail: %v", err)
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
