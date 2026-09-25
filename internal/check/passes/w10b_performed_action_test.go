package passes

import (
	"strings"
	"testing"
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
