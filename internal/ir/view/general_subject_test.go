package view

import (
	"strings"
	"testing"
)

const subjectModel = `package Model {
	private import StandardViewDefinitions::*;
	part def SystemDefinition;
	part def User;
	requirement def MassRequirement {
		subject sys : SystemDefinition;
		actor user : User;
	}
	use case def Drive {
		subject vehicle : SystemDefinition;
	}
	view definitions : GeneralView {
		filter %s;
		expose Model::**;
	}
}`

// A subject parameter the view does not draw is the edge to its drawn type,
// as an actor in the same position is.
func TestGeneralViewSubjectEdgeToDrawnType(t *testing.T) {
	text := textWithQualifiers(t, renderSource(t, "Model::definitions", strings.Replace(subjectModel, "%s", "@SysML::Definition", 1)))
	for _, want := range []string{
		"Model::MassRequirement o-- Model::SystemDefinition: sys\n",
		"Model::MassRequirement o-- Model::User: user\n",
		"Model::Drive o-- Model::SystemDefinition: vehicle\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
}

// A subject parameter the view draws is a node its owner references and its
// type types.
func TestGeneralViewDrawnSubject(t *testing.T) {
	text := textWithQualifiers(t, renderSource(t, "Model::definitions", strings.Replace(subjectModel, "%s", "@SysML::Definition or @SysML::Usage", 1)))
	for _, want := range []string{
		"Model::MassRequirement o-- Model::MassRequirement::sys\n",
		"Model::MassRequirement::sys --:|> Model::SystemDefinition\n",
		"Model::Drive o-- Model::Drive::vehicle\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	if unwanted := "Model::MassRequirement o-- Model::SystemDefinition: sys"; strings.Contains(text, unwanted) {
		t.Errorf("a drawn subject is also drawn as %q:\n%s", unwanted, text)
	}
}
