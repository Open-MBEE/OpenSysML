package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Pilot SysMLValidator (2026-05) reports a usage typed by the wrong kind of
// element per usage kind, naming the kind of definition the usage takes, at the
// declaration rather than at the type reference. Our own kind rules decide when
// the typing is wrong (see compatMessage); this table decides how it is said.
var pilotTypingMessages = map[ast.UsageKind]string{
	ast.UsageAttribute:  "An attribute must be typed by attribute definitions.",
	ast.UsagePart:       msgOccurrenceUsageType,
	ast.UsageItem:       msgOccurrenceUsageType,
	ast.UsageOccurrence: msgOccurrenceUsageType,
	ast.UsagePort:       "A port must be typed by port definitions.",
	ast.UsageAction:     "An action must be typed by action definitions.",
	ast.UsageState:      "A state must be typed by state definitions.",
	ast.UsageConnection: "A connection must be typed by connection definitions.",
	ast.UsageInterface:  "An interface must be typed by interface definitions.",
	ast.UsageAllocation: "An allocation must be typed by allocation definitions.",
	ast.UsageFlow:       "A flow connection must be typed by flow connection definitions.",
}

// msgUsageTyping is the reference's message for a usage of no stated kind — a
// ReferenceUsage — whose type is not a definition.
const msgUsageTyping = "A usage must be typed by definitions."

// msgIndividualOneType is the reference's message for an individual usage,
// which is typed by a single individual definition (SysML v2 §8.3.9.11).
const msgIndividualOneType = "An individual must be typed by one individual definition."

// pilotTypingMessage is the message and code a wrong typing draws on decl, and
// whether the reference states one: KerML has no usage-kind taxonomy, and a
// definition carries no typing.
func pilotTypingMessage(decl declKind) (msg, code string, ok bool) {
	// A conjugated typing is the conjugation rule's, and an occurrence modifier
	// carries its own rule (§8.3.9.11) rather than the kind keyword's.
	// A `feature`, even in a SysML file, is KerML notation stating no usage kind.
	if decl.isDef || decl.isKerML() || decl.conjugated || decl.isOccurrenceUsage() ||
		decl.keyword == "feature" {
		return "", "", false
	}
	if decl.isReferenceUsage() {
		return msgUsageTyping, "usage-typing", true
	}
	if msg, ok := pilotTypingMessages[decl.useKind]; ok {
		return msg, "usage-typing", true
	}
	// A usage typed by exactly one definition names that definition's kind in
	// the same breath, so the one-type rule's message covers a wrong kind too.
	if msg, ok := oneTypeUsageMessages[decl.useKind]; ok {
		return msg, "one-type", true
	}
	return "", "", false
}
