package edit

import (
	"testing"
)

func TestAddStateIntoBodilessSubstate(t *testing.T) {
	source := "state def S {\n" +
		"    state toasting;\n" +
		"}\n"
	model := loadContent(t, "substate.sysml", source)
	requireClean(t, model)

	result := applyOne(t, model, AddMember("S::toasting", "state", "heating"))
	want := "state def S {\n" +
		"    state toasting {\n" +
		"        state heating;\n" +
		"    }\n" +
		"}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("edited source:\n%s\nwant:\n%s", got, want)
	}
	requireClean(t, loadContent(t, "substate.sysml", string(result.Content)))
}

func TestAddTransitionIntoBodilessSubstate(t *testing.T) {
	source := "state def S {\n" +
		"    state toasting;\n" +
		"}\n"
	model := loadContent(t, "substate-transition.sysml", source)
	requireClean(t, model)

	result, err := Apply(model, []Operation{
		AddMember("S::toasting", "state", "heating"),
		AddTransition("S::toasting", "", "", "heating", "", "", "", true),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	requireClean(t, loadContent(t, "substate-transition.sysml", string(result.Content)))
	want := "state def S {\n" +
		"    state toasting {\n" +
		"        state heating;\n" +
		"        entry; then heating;\n" +
		"    }\n" +
		"}\n"
	if got := string(result.Content); got != want {
		t.Fatalf("edited source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveIntoBodilessSubstate(t *testing.T) {
	source := "package P {\n" +
		"    state def S {\n" +
		"        state toasting;\n" +
		"    }\n" +
		"    action cool;\n" +
		"}\n"
	model := loadContent(t, "substate-move.sysml", source)
	requireClean(t, model)

	result := applyOne(t, model, Move("P::cool", "P::S::toasting"))
	requireClean(t, loadContent(t, "substate-move.sysml", string(result.Content)))
	if got := string(result.Content); got != "package P {\n"+
		"    state def S {\n"+
		"        state toasting {\n"+
		"            action cool;\n"+
		"        }\n"+
		"    }\n"+
		"}\n" {
		t.Fatalf("moved source:\n%s", got)
	}
}
