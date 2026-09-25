package migrate

import (
	"slices"
	"strings"
	"testing"
)

func projectText(p *projection) (string, []string) {
	project, notes := p.build(qlit("rows"))
	return strings.Join(project.lines(""), "\n"), notes
}

// Properties ahead of the computed columns are written as they stand; a
// property behind a computed column is written first and noted.
func TestProjectionWritesPropertiesFirst(t *testing.T) {
	plain := &projection{}
	plain.property("name")
	plain.property("qualifiedName")
	plain.column("mass", qlit("Pump::mass"))
	got, notes := projectText(plain)
	want := `Project(
    source = rows,
    properties = ("name", "qualifiedName"),
    columns = (
        Column(name = "mass", expression = Pump::mass)))`
	if got != want || len(notes) != 0 {
		t.Errorf("plain projection = \n%s\nnotes %v, want\n%s", got, notes, want)
	}

	pure := &projection{}
	pure.property("name")
	if got, _ := projectText(pure); got != `Project(source = rows, properties = ("name"))` {
		t.Errorf("pure projection = %s", got)
	}

	mixed := &projection{}
	mixed.property("name")
	mixed.column("mass", qlit("Pump::mass"))
	mixed.property("owner")
	mixed.column("flow", qlit("Pump::flow"))
	got, notes = projectText(mixed)
	want = `Project(
    source = rows,
    properties = ("name", "owner"),
    columns = (
        Column(name = "mass", expression = Pump::mass),
        Column(name = "flow", expression = Pump::flow)))`
	wantNotes := []string{"Project lists its properties first: name, owner precede the other columns"}
	if got != want || !slices.Equal(notes, wantNotes) {
		t.Errorf("mixed projection = \n%s\nnotes %v, want\n%s\nnotes %v", got, notes, want, wantNotes)
	}
}

// A computed caption that repeats a property, listed before or after it, or
// another computed caption, is suffixed; a repeated property is listed once.
func TestProjectionClaimsPropertyNamesFirst(t *testing.T) {
	p := &projection{}
	p.column("name", qlit("Pump::label"))
	if !p.property("name") || p.property("name") {
		t.Fatal("a property is listed exactly once")
	}
	p.column("mass", qlit("Pump::mass"))
	p.column("mass", qlit("Pump::dryMass"))
	got, notes := projectText(p)
	want := `Project(
    source = rows,
    properties = ("name"),
    columns = (
        Column(name = "name 2", expression = Pump::label),
        Column(name = "mass", expression = Pump::mass),
        Column(name = "mass 2", expression = Pump::dryMass)))`
	if got != want {
		t.Errorf("projection = \n%s\nwant\n%s", got, want)
	}
	wantNotes := []string{
		"the column name is written as name 2: column names are unique",
		"the column mass is written as mass 2: column names are unique",
		"Project lists its properties first: name precede the other columns",
	}
	if !slices.Equal(notes, wantNotes) {
		t.Errorf("notes = %v, want %v", notes, wantNotes)
	}
}
