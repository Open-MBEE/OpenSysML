package migrate

import (
	"strings"
	"testing"
)

func projectText(p *projection) (string, [][2]string) {
	project, renamed := p.build(qlit("rows"))
	return strings.Join(project.lines(""), "\n"), renamed
}

// Properties ahead of the computed columns keep the plain properties list;
// a computed column ahead of a property makes the whole projection ordered.
func TestProjectionKeepsSourceOrder(t *testing.T) {
	plain := &projection{}
	plain.property("name")
	plain.property("qualifiedName")
	plain.column("mass", qlit("Pump::mass"))
	got, renamed := projectText(plain)
	want := `Project(
    source = rows,
    properties = ("name", "qualifiedName"),
    columns = (
        Column(name = "mass", expression = Pump::mass)))`
	if got != want || len(renamed) != 0 {
		t.Errorf("plain projection = \n%s\nrenamed %v, want\n%s", got, renamed, want)
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
	got, renamed = projectText(mixed)
	want = `Project(
    source = rows,
    columns = (
        PropertyColumn(name = "name"),
        Column(name = "mass", expression = Pump::mass),
        PropertyColumn(name = "owner"),
        Column(name = "flow", expression = Pump::flow)))`
	if got != want || len(renamed) != 0 {
		t.Errorf("mixed projection = \n%s\nrenamed %v, want\n%s", got, renamed, want)
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
	got, renamed := projectText(p)
	want := `Project(
    source = rows,
    columns = (
        Column(name = "name 2", expression = Pump::label),
        PropertyColumn(name = "name"),
        Column(name = "mass", expression = Pump::mass),
        Column(name = "mass 2", expression = Pump::dryMass)))`
	if got != want {
		t.Errorf("projection = \n%s\nwant\n%s", got, want)
	}
	if len(renamed) != 2 || renamed[0] != [2]string{"name", "name 2"} || renamed[1] != [2]string{"mass", "mass 2"} {
		t.Errorf("renamed = %v", renamed)
	}
}
