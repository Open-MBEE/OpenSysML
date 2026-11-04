package export_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultMultiplicityIsStated checks that a usage whose multiplicity is the
// implicit [1..1] (SysML v2 §7.6.3) owns a Multiplicity stating it, through an
// OwningMembership last among its owned relationships, as the pilot states it;
// a declared range, a withheld usage, an action and a KerML feature own none.
func TestDefaultMultiplicityIsStated(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "dm.sysml")
	if err := os.WriteFile(model, []byte(`package DM {
    part def Vehicle {
        attribute mass : ScalarValues::Real;
        attribute tags : ScalarValues::String [2];
        part wheel : Wheel;
        part spare :> wheel;
        action go;
    }
    part def Wheel;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	byID := elementsByID(apiJSON(t, model))
	byName := map[string]map[string]any{}
	for _, el := range byID {
		if name, ok := el["qualifiedName"].(string); ok {
			byName[name] = el
		}
	}
	for _, name := range []string{"DM::Vehicle::mass", "DM::Vehicle::wheel"} {
		usage := byName[name]
		if usage == nil {
			t.Fatalf("%s is not exported", name)
		}
		multiplicity := byID[refID(usage["multiplicity"])]
		if multiplicity == nil || multiplicity["@type"] != "Multiplicity" {
			t.Fatalf("%s multiplicity = %v, want a Multiplicity", name, usage["multiplicity"])
		}
		if _, named := multiplicity["qualifiedName"]; named && multiplicity["qualifiedName"] != nil {
			t.Errorf("%s's default multiplicity has qualifiedName %v, want none", name, multiplicity["qualifiedName"])
		}
		membership := byID[refID(multiplicity["owningMembership"])]
		if membership == nil || membership["@type"] != "OwningMembership" {
			t.Fatalf("%s's default multiplicity is owned through %v, want an OwningMembership", name, multiplicity["owningMembership"])
		}
		if membership["isImplied"] == true {
			t.Errorf("%s's default multiplicity membership is implied; the pilot states it as declared", name)
		}
		owned := refIDs(usage["ownedRelationship"])
		if len(owned) == 0 || owned[len(owned)-1] != membership["@id"] {
			t.Errorf("%s ownedRelationship = %v, want the default multiplicity's membership last", name, owned)
		}
		if members := refIDs(usage["ownedMember"]); len(members) == 0 || members[len(members)-1] != multiplicity["@id"] {
			t.Errorf("%s ownedMember = %v, want the default multiplicity last", name, members)
		}
	}
	tags := byName["DM::Vehicle::tags"]
	if rng := byID[refID(tags["multiplicity"])]; rng == nil || rng["@type"] != "MultiplicityRange" {
		t.Errorf("tags [2] multiplicity = %v, want its declared MultiplicityRange", tags["multiplicity"])
	} else if members := refIDs(tags["ownedMember"]); len(members) != 1 || members[0] != rng["@id"] {
		t.Errorf("tags ownedMember = %v, want the declared range alone", members)
	}
	for _, name := range []string{"DM::Vehicle::spare", "DM::Vehicle::go"} {
		usage := byName[name]
		if usage == nil {
			t.Fatalf("%s is not exported", name)
		}
		if usage["multiplicity"] != nil {
			t.Errorf("%s multiplicity = %v, want none: a subsetting usage and an action take no default", name, usage["multiplicity"])
		}
		for _, el := range byID {
			if el["@type"] == "Multiplicity" && refID(el["owner"]) == usage["@id"] {
				t.Errorf("%s owns a Multiplicity %s, want none", name, el["@id"])
			}
		}
	}
	kerml := filepath.Join(dir, "dk.kerml")
	if err := os.WriteFile(kerml, []byte("package DK {\n    classifier C { feature f : C; }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, el := range apiJSON(t, kerml) {
		if el["@type"] == "Multiplicity" {
			t.Errorf("a KerML feature owns a Multiplicity %s; only a SysML usage takes the implicit [1..1]", el["@id"])
		}
	}
}
