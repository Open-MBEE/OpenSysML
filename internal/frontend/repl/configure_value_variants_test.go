package repl

import "testing"

// valuedModel is a family whose variation point is value-typed: two of its
// variants declare the same value, so a constraint selecting one by `==` is met
// by either, as validating an object selecting the other answers.
const valuedModel = `
package Valued {
    private import ScalarValues::Integer;
    part def Gear { attribute ratio : Integer; }
    abstract part gearFamily : Gear {
        variation attribute ratioChoice :>> ratio {
            variant attribute low : Integer = 3;
            variant attribute lowAgain : Integer = 3;
            variant attribute high : Integer = 5;
        }
        assert constraint isLow { ratioChoice == ratioChoice::low }
    }
    part again :> gearFamily { attribute :>> ratioChoice = ratioChoice::lowAgain; }
    part steep :> gearFamily { attribute :>> ratioChoice = ratioChoice::high; }
}
`

// TestConfigurePermitsWhatValidateAccepts: `%configure` of the family permits
// every variant declaring the selected value, the same selections `%validate`
// of an object accepts, and refuses the one declaring another.
func TestConfigurePermitsWhatValidateAccepts(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, valuedModel)
	wants(t, run(t, s, "%configure Valued::gearFamily all"),
		"permits 2 selections of variants",
		"Valued::gearFamily::ratioChoice = Valued::gearFamily::ratioChoice::low",
		"Valued::gearFamily::ratioChoice = Valued::gearFamily::ratioChoice::lowAgain")
	wants(t, run(t, s, "%configure Valued::gearFamily ratioChoice=high"),
		"✗ the chosen variants are not consistent with Abstract part gearFamily",
		"Against: required condition { ratioChoice == ratioChoice::low } of isLow")
	run(t, s, "%instantiate Valued::again")
	wants(t, run(t, s, "%validate again"),
		"✓ assert constraint isLow holds (on Valued::again ID: 1)",
		"✓ Valued::again is valid: its 1 assertion holds")
	run(t, s, "%instantiate Valued::steep")
	wants(t, run(t, s, "%validate steep"),
		"✗ assert constraint isLow fails (on Valued::steep ID: 2)",
		"✗ Valued::steep is not valid: 1 of 1 assertion fails")
}
