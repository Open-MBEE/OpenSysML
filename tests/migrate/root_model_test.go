package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A root uml:Model is written as a package of its name, as the OMG
// transformation maps a Model: its members, comments and relationship-driven
// members sit inside it, and references qualify from the package, with `$::`
// only where a member still shadows the global name.
func TestRootModelWrittenAsPackage(t *testing.T) {
	t.Run("members directly under the Model", func(t *testing.T) {
		r := migrateFixtureFile(t, "root_model_defs")
		notation := string(r.Notation)
		if !strings.HasPrefix(notation, "package SEM {\n    doc /* Signal electronics model. */\n    part def 'Imaging Subsystem' {") {
			t.Errorf("the Model is not written as package SEM holding its comment and members:\n%s", notation)
		}
		if !strings.HasSuffix(strings.TrimRight(notation, "\n"), "\n}") {
			t.Errorf("the package is not closed last:\n%s", notation)
		}
		wantLine(t, r.Notation, "part BSED : SEM::BSED;")
		wantLine(t, r.Notation, "attribute gain : Real;")
		wantNoLine(t, r.Notation, "$::")
		wantNote(t, r, "_m", migrate.Mapped, "written as package SEM")
		for _, e := range entriesFor(r, "_m") {
			if e.Target != "SEM" {
				t.Errorf("Model target = %q, want SEM", e.Target)
			}
		}
		for _, e := range entriesFor(r, "_bsed") {
			if e.Target != "SEM::BSED" {
				t.Errorf("BSED target = %q, want SEM::BSED", e.Target)
			}
		}
	})
	t.Run("packages under the Model", func(t *testing.T) {
		r := migrateFixtureFile(t, "root_model_packages")
		wantLine(t, r.Notation, "package Catalog {\n    package Components {\n        part def Motor;")
		wantLine(t, r.Notation, "part motor : Components::Motor;")
		wantLine(t, r.Notation, "part Wheel : Components::Wheel;")
		wantNote(t, r, "_m", migrate.Mapped, "written as package Catalog")
	})
	t.Run("unnamed Model", func(t *testing.T) {
		r := migrateFixtureFile(t, "root_model_unnamed")
		wantLine(t, r.Notation, "package root_model_unnamed {\n    part def Pump {")
		wantNote(t, r, "_m", migrate.Approximated, "written as package root_model_unnamed; the Model has no name; the package is named after the file")
	})
	t.Run("two Models", func(t *testing.T) {
		r := migrateFixtureFile(t, "root_models_two")
		wantLine(t, r.Notation, "package Alpha {\n    part def Sensor;\n}\npackage Beta {")
		wantLine(t, r.Notation, "part sensor : Alpha::Sensor;")
		wantLine(t, r.Notation, "part Alpha : $::Alpha::Sensor;")
		wantNote(t, r, "_alpha", migrate.Mapped, "written as package Alpha")
		wantNote(t, r, "_beta", migrate.Mapped, "written as package Beta")
	})
}
