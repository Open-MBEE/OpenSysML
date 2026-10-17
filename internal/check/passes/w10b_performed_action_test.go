package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestPerformedActionReferentKindAndTyping(t *testing.T) {
	src := `
		package P {
			part def PartType;
			action def ActionType;
			action def Actions {
				action bad : PartType;
			}
			ref partRef : PartType;
			perform partRef;
			ref actions : Actions;
			perform actions.bad;
		}
	`
	messages := w11aMessages(t, src, true)
	if !containsMessage(messages, "Must reference an action.") {
		t.Fatalf("expected performed action kind diagnostic, got %v", messages)
	}
	if !containsMessage(messages, "An action must be typed by action definitions.") {
		t.Fatalf("expected performed action typing diagnostic, got %v", messages)
	}
}

func containsMessage(messages []string, want string) bool {
	for _, message := range messages {
		if strings.Contains(message, want) {
			return true
		}
	}
	return false
}

func TestIncludedUseCaseReferentKindAndTyping(t *testing.T) {
	src := `
		package P {
			case def CaseType;
			use case def UseCaseType;
			part def PartType {
				use case bad : CaseType;
			}
			part p : PartType;
			ref unknown;
			use case enclosing {
				include unknown;
				include p.bad;
			}
		}
	`
	var messages []string
	for _, d := range typeDiags(t, src) {
		messages = append(messages, d.Message)
	}
	if !containsMessage(messages, "Must reference a use case.") {
		t.Fatalf("expected included use case kind diagnostic, got %v", messages)
	}
	if !containsMessage(messages, "A use case must be typed by one use case definition.") {
		t.Fatalf("expected included use case typing diagnostic, got %v", messages)
	}
}

func TestIncludedUseCaseDeclarationIsNotAReference(t *testing.T) {
	src := `
		package P {
			use case def UseCaseType;
			use case enclosing {
				include use case declared : UseCaseType;
			}
		}
	`
	for _, message := range w11aMessages(t, src, true) {
		if message == "Must reference a use case." {
			t.Fatalf("unexpected reference-kind diagnostic for declaration form")
		}
	}
}

func TestIncludedUseCaseWithUnresolvedTypeIsNotClassified(t *testing.T) {
	src := `
		package P {
			use case enclosing {
				include use case declared : MissingUseCaseType;
			}
		}
	`
	for _, message := range w11aMessages(t, src, true) {
		if message == "A use case must be typed by one use case definition." {
			t.Fatalf("unexpected use-case typing diagnostic for unresolved type")
		}
	}
}

func TestPerformedFlowReferentIsPerformable(t *testing.T) {
	src := `
		package P {
			flow f;
			perform f;
		}
	`
	for _, d := range w9cLibraryDiags(t, src, false) {
		if d.Message == "Must reference an action." {
			t.Fatalf("unexpected action-kind diagnostic for flow performance at %v", d.Span)
		}
	}
}

// lineOf is the 1-based line of offset in src.
func lineOf(src string, offset int) int {
	return 1 + strings.Count(src[:offset], "\n")
}

// A definition target draws the kind and non-usage errors at the target, as the
// pilot does; qualified names and aliases behave the same; a usage stays clean.
func TestIncludedDefinitionIsNotAUseCaseReference(t *testing.T) {
	src := `package Inc {
    use case def UD;
    use case uc1 : UD;
    alias UDAlias for UD;
    use case def Outer {
        include UD;
        include uc1;
        include Inc::UD;
        include UDAlias;
    }
}`
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	byLine := map[int][]string{}
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Severity != diag.SeverityError {
			continue
		}
		byLine[lineOf(src, d.Span.Offset)] = append(byLine[lineOf(src, d.Span.Offset)], d.Message)
	}
	for _, line := range []int{6, 8, 9} {
		got := byLine[line]
		if len(got) != 2 || !containsMessage(got, "Must reference a use case.") ||
			!containsMessage(got, "includes target must be a usage, found useCaseDef") {
			t.Errorf("line %d: want the kind and non-usage target diagnostics, got %v", line, got)
		}
	}
	for line, got := range byLine {
		if line != 6 && line != 8 && line != 9 {
			t.Errorf("line %d: unexpected diagnostics %v", line, got)
		}
	}
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Severity == diag.SeverityError && src[d.Span.Offset:d.Span.End()] != "UD" &&
			src[d.Span.Offset:d.Span.End()] != "Inc::UD" && src[d.Span.Offset:d.Span.End()] != "UDAlias" {
			t.Errorf("diagnostic %q is not at the include target: %q", d.Message, src[d.Span.Offset:d.Span.End()])
		}
	}
}

// Every include shape naming a use case usage is clean: direct, typed, with a
// body, by qualified name, via alias, through a chain, inside a use case usage.
func TestIncludeShapesReferencingUseCaseUsagesAreClean(t *testing.T) {
	src := `package IncPos {
    use case def UD {
        use case inner : UD;
    }
    use case uc1 : UD;
    alias a for uc1;
    use case def Outer {
        include uc1;
        include use case x : UD;
        include uc1 {
            subject s;
        }
        include IncPos::uc1;
        include a;
        include uc1.inner;
        use case nested : UD {
            include uc1;
            include IncPos::uc1;
        }
    }
    part def Sys {
        use case u : UD {
            include uc1;
        }
    }
}`
	// The library is loaded so the usages are typed and the type tier runs.
	for _, d := range w9cLibraryDiags(t, src, false) {
		if d.Severity == diag.SeverityError {
			t.Errorf("unexpected error at line %d: %s", lineOf(src, d.Span.Offset), d.Message)
		}
	}
}

// A perform or exhibit of a definition draws the kind message beside the
// non-usage target message, as the pilot reports both.
func TestPerformedDefinitionIsNotAReference(t *testing.T) {
	src := `package P {
    action def AD;
    state def SD;
    part def Sys {
        perform AD;
        exhibit SD;
    }
}`
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	byLine := map[int][]string{}
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Severity == diag.SeverityError {
			byLine[lineOf(src, d.Span.Offset)] = append(byLine[lineOf(src, d.Span.Offset)], d.Message)
		}
	}
	if got := byLine[5]; len(got) != 2 || !containsMessage(got, "Must reference an action.") ||
		!containsMessage(got, "references target must be a usage, found actionDef") {
		t.Errorf("perform AD: got %v", got)
	}
	if got := byLine[6]; len(got) != 2 || !containsMessage(got, "Must reference a state.") ||
		!containsMessage(got, "references target must be a usage, found stateDef") {
		t.Errorf("exhibit SD: got %v", got)
	}
}
