package view

import (
	"strings"
	"testing"
)

const requirementPartiesModel = `package Model {
	private import StandardViewDefinitions::*;
	part def SystemDefinition;
	part def User;
	part def Regulator;
	part def Unrelated;
	requirement def MassRequirement {
		subject sys : SystemDefinition;
		actor user : User;
		stakeholder reg : Regulator;
	}
	requirement def Plain {
		subject : SystemDefinition;
	}
	view requirements : GeneralView {
		filter %s;
		expose Model::**;
	}
}`

// A requirement graph draws each requirement's subject, actors and
// stakeholders: the definitions typing them beside it, each edge named by its role.
func TestRequirementGraphDrawsSubjectActorStakeholder(t *testing.T) {
	rendering := renderSource(t, "Model::requirements", strings.Replace(requirementPartiesModel, "%s", "@SysML::RequirementDefinition", 1))
	if rendering.Kind != KindRequirement {
		t.Fatalf("kind = %s, want %s", rendering.Kind, KindRequirement)
	}
	text := textWithQualifiers(t, rendering)
	for _, want := range []string{
		"part def Model::SystemDefinition\n",
		"Model::MassRequirement o-- Model::SystemDefinition: «subject» sys\n",
		"Model::MassRequirement o-- Model::User: «actor» user\n",
		"Model::MassRequirement o-- Model::Regulator: «stakeholder» reg\n",
		"Model::Plain o-- Model::SystemDefinition: «subject»\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	if unwanted := "Model::Unrelated"; strings.Contains(text, unwanted) {
		t.Errorf("a definition no requirement relates to is drawn as %q:\n%s", unwanted, text)
	}
}

// A filter selecting requirements and definitions draws both in the
// requirement graph, a definition no requirement relates to included.
func TestRequirementGraphKeepsFilteredDefinitions(t *testing.T) {
	rendering := renderSource(t, "Model::requirements", strings.Replace(requirementPartiesModel, "%s", "@SysML::RequirementDefinition or @SysML::PartDefinition", 1))
	if rendering.Kind != KindRequirement {
		t.Fatalf("kind = %s, want %s", rendering.Kind, KindRequirement)
	}
	text := textWithQualifiers(t, rendering)
	for _, want := range []string{
		"part def Model::Unrelated\n",
		"requirement def Model::MassRequirement\n",
		"Model::MassRequirement o-- Model::SystemDefinition: «subject» sys\n",
		"Model::MassRequirement o-- Model::User: «actor» user\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	for _, notice := range rendering.Notices {
		if strings.Contains(notice, "exposed element(s) not drawn") {
			t.Errorf("the filtered definitions are reported as not drawn: %s", notice)
		}
	}
}

// A definition graph names no role: the edge stays the association of a feature.
func TestDefinitionGraphNamesNoRequirementRole(t *testing.T) {
	text := textWithQualifiers(t, renderSource(t, "Model::requirements", strings.Replace(requirementPartiesModel, "%s", "@SysML::Definition", 1)))
	if want := "Model::MassRequirement o-- Model::User: user\n"; !strings.Contains(text, want) {
		t.Errorf("rendering lacks %q:\n%s", want, text)
	}
}

const requirementRedefinedPartiesModel = `package Model {
	private import StandardViewDefinitions::*;
	part def Base;
	part def SystemDefinition :> Base;
	part def User;
	requirement def MassRequirement {
		subject sys : SystemDefinition;
		actor user : User;
	}
	requirement massOfCar : MassRequirement {
		subject :>> sys;
		actor :>> user;
	}
	view requirements : GeneralView {
		filter %s;
		expose Model::**;
	}
}`

// A role redeclared without a type goes to the type of the feature it redefines.
func TestRequirementGraphDrawsRedefinedRoles(t *testing.T) {
	text := textWithQualifiers(t, renderSource(t, "Model::requirements", strings.Replace(requirementRedefinedPartiesModel, "%s", "@SysML::RequirementUsage", 1)))
	for _, want := range []string{
		"Model::massOfCar o-- Model::SystemDefinition: «subject» sys\n",
		"Model::massOfCar o-- Model::User: «actor» user\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
}

// A role the filter draws as a node keeps its role on the edge from its owner,
// and its type is drawn beside it.
func TestRequirementGraphNamesTheRoleOfADrawnParty(t *testing.T) {
	text := textWithQualifiers(t, renderSource(t, "Model::requirements", strings.Replace(requirementRedefinedPartiesModel, "%s", "@SysML::RequirementDefinition or @SysML::PartUsage", 1)))
	for _, want := range []string{
		"part def Model::User\n",
		"Model::MassRequirement o-- Model::MassRequirement::user: «actor»\n",
		"Model::MassRequirement::user --:|> Model::User\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
}

// A party's type is a node before any edge is drawn, so an exposed element
// declared before the requirement still reaches it.
func TestRequirementGraphPartyEdgesIgnoreDeclarationOrder(t *testing.T) {
	text := textWithQualifiers(t, renderSource(t, "Model::requirements", `package Model {
	private import StandardViewDefinitions::*;
	package Parts {
		part def SystemDefinition;
	}
	part car : Parts::SystemDefinition;
	requirement def MassRequirement {
		subject sys : Parts::SystemDefinition;
	}
	view requirements : GeneralView {
		filter @SysML::RequirementDefinition or @SysML::PartUsage;
		expose Model::*;
	}
}`))
	for _, want := range []string{
		"Model::car --:|> Model::Parts::SystemDefinition\n",
		"Model::MassRequirement o-- Model::Parts::SystemDefinition: «subject» sys\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
}
