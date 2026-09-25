package repl

import "testing"

// TestFeatureValuesEnumerationLiteral: an enum-typed default materializes, and the feature value
// shows the literal as the model writes it rather than a number or an error.
func TestFeatureValuesEnumerationLiteral(t *testing.T) {
	s := loadFixture(t, "testdata/enum_package.sysml")
	run(t, s, "%instantiate D::Car")

	got := run(t, s, "%features D::Car")
	wants(t, got,
		"c = Color::red",
		"l = Level::high",
		"isRed = true",
		"grade = 9",
	)
	rejects(t, got, "has no value", "<unknown>")
}

// A literal evaluates on its own, outside any instance.
func TestEvalEnumerationLiteral(t *testing.T) {
	s := loadFixture(t, "testdata/enum_package.sysml")

	wants(t, run(t, s, "%eval D::Color::red"), "= Color::red")
	wants(t, run(t, s, "%eval D::Level::low.n"), "= 1")
}
