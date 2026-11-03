package queryexec

import "testing"

// TestQueryRobustnessDerivation exercises ends whose role statement and participant are hard
// to tell apart; each answers the ends' stated roles or an empty cell, never a panic or a hang.
const relatedQuery = `
	calc def Related :> Query {
		in source : Element[1..*] ordered;
		in kind : String;
		in direction : String;
		in maxDepth : Integer;
		RelatedElements(source = source, relationshipKind = kind, direction = direction, maxDepth = maxDepth)
	}
`

func TestQueryRobustnessDerivation(t *testing.T) {
	t.Run("participant_tagged_with_the_opposite_role", testDerivationOppositeTag)
	t.Run("end_referencing_the_role_feature_itself", testDerivationRoleFeatureReferent)
	t.Run("participant_tagged_with_both_roles", testDerivationBothTags)
	t.Run("participant_subsetting_the_role_feature", testDerivationSubsettingParticipant)
}

// The end's own role statement wins over the participant's tag: a `#original`
// end naming a requirement tagged `@derive` still relates it as the original.
func testDerivationOppositeTag(t *testing.T) {
	fixture := loadExecutionFixture(t, relatedQuery+`
	private import RequirementDerivation::*;
	requirement parent { @derive; }
	requirement child { @original; }
	#derivation connection { end #original ::> parent; end #derive ::> child; }
`)
	assertRelated(t, fixture, "child", "derivation", "incoming", 1, []string{"parent"})
	assertRelated(t, fixture, "parent", "derivation", "incoming", 1, nil)
}

// An end that references the library's role feature names no requirement of
// the model: the derivation states no edge and the column is empty.
func testDerivationRoleFeatureReferent(t *testing.T) {
	fixture := loadExecutionFixture(t, relatedQuery+`
	private import RequirementDerivation::*;
	requirement child;
	#derivation connection { end ::> originalRequirements; end #derive ::> child; }
`)
	assertRelated(t, fixture, "child", "derivation", "incoming", 1, nil)
	assertRelated(t, fixture, "child", "derivation", "outgoing", 1, nil)
}

// A requirement tagged with both roles is still the participant of the end
// that names it, in the role that end states.
func testDerivationBothTags(t *testing.T) {
	fixture := loadExecutionFixture(t, relatedQuery+`
	private import RequirementDerivation::*;
	requirement middle { @original; @derive; }
	requirement top;
	requirement bottom;
	#derivation connection { end #original ::> top; end #derive ::> middle; }
	#derivation connection { end #original ::> middle; end #derive ::> bottom; }
`)
	assertRelated(t, fixture, "middle", "derivation", "incoming", 1, []string{"top"})
	assertRelated(t, fixture, "middle", "derivation", "outgoing", 1, []string{"bottom"})
	assertRelated(t, fixture, "bottom", "derivation", "incoming", 2, []string{"middle", "top"})
}

// A requirement of the model that subsets the library role feature is a
// participant like a tagged one; only the library's own features state a role.
func testDerivationSubsettingParticipant(t *testing.T) {
	fixture := loadExecutionFixture(t, relatedQuery+`
	private import RequirementDerivation::*;
	requirement customOriginal :> originalRequirements;
	requirement child;
	#derivation connection { end #original ::> customOriginal; end #derive ::> child; }
`)
	assertRelated(t, fixture, "child", "derivation", "incoming", 1, []string{"customOriginal"})
	assertRelated(t, fixture, "customOriginal", "derivation", "outgoing", 1, []string{"child"})
}
