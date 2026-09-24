package symbols

import "testing"

// A usage declared without a name takes the effective name of the feature it
// reference-subsets, so `perform providePower.generateTorque;` contributes the
// member `generateTorque` to the part performing it.
func TestPerformShorthandTakesReferencedName(t *testing.T) {
	root := build(t, `package P {
		action providePower { action generateTorque; }
		part torqueGenerator { perform providePower.generateTorque; }
	}`)

	pkg, _ := root.LookupLocal("P")
	gen, _ := pkg.Scope.LookupLocal("torqueGenerator")
	if _, ok := gen.Scope.LookupLocal("generateTorque"); !ok {
		t.Fatalf("torqueGenerator members = %v, want generateTorque", gen.Scope.MemberNames())
	}
}

func TestReferencesShorthandTakesReferencedName(t *testing.T) {
	root := build(t, `package P {
		action takePicture;
		part camera { perform action references takePicture; }
	}`)

	pkg, _ := root.LookupLocal("P")
	camera, _ := pkg.Scope.LookupLocal("camera")
	if _, ok := camera.Scope.LookupLocal("takePicture"); !ok {
		t.Fatalf("camera members = %v, want takePicture", camera.Scope.MemberNames())
	}
}

// A declared name always wins over the referenced feature's name.
func TestPerformDeclaredNameWins(t *testing.T) {
	root := build(t, `package P {
		action takePicture;
		part camera { perform action takePhoto references takePicture; }
	}`)

	pkg, _ := root.LookupLocal("P")
	camera, _ := pkg.Scope.LookupLocal("camera")
	if _, ok := camera.Scope.LookupLocal("takePhoto"); !ok {
		t.Fatalf("camera members = %v, want takePhoto", camera.Scope.MemberNames())
	}
	if _, ok := camera.Scope.LookupLocal("takePicture"); ok {
		t.Fatalf("camera should not bind the referenced name when takePhoto is declared")
	}
}
