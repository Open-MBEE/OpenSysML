package main

import (
	"strings"
	"testing"
)

// chainRedefinitionModel satisfies a requirement by a part while redefining a
// chain through that part (`redefines vehicle1.fuel`), which holds during the
// check and nowhere else.
const chainRedefinitionModel = `package ChainProbe {
    private import ScalarValues::Integer;
    part def Vehicle {
        attribute fuel : Integer default 0;
    }
    part vehicle1 : Vehicle;
    requirement def Full {
        subject vehicle : Vehicle;
        attribute target : Integer default 1300;
        require constraint { vehicle.fuel == target }
    }
    satisfy requirement check : Full by vehicle1 {
        attribute redefines vehicle1.fuel = 1300;
    }
}`

// TestSatisfyChainRedefinition checks that a satisfy usage's chain redefinition
// through its satisfying feature is in force while -satisfy judges it, on a
// fresh subject and on one %instantiate made first, and that the object reads
// its own value again once the check is over.
func TestSatisfyChainRedefinition(t *testing.T) {
	binary := buildCLI(t)

	t.Run("satisfy", func(t *testing.T) {
		wantReport(t, check(t, binary, chainRedefinitionModel, "-strict", "-satisfy"), exitHolds,
			"✓ satisfy check by vehicle1 holds")
	})

	t.Run("instantiated subject", func(t *testing.T) {
		stdin := "%instantiate ChainProbe::vehicle1\n%satisfy\n%eval ChainProbe::vehicle1.fuel\n%quit\n"
		got := runPiped(t, binary, stdin, chainRedefinitionModel)
		if got.status != exitHolds {
			t.Errorf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
		}
		for _, want := range []string{"✓ satisfy check by vehicle1 holds (on ChainProbe::vehicle1 ID: 1)", "✓ ChainProbe::vehicle1.fuel\n  = 0"} {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("the session did not report %q:\n%s", want, got.output())
			}
		}
	})

	t.Run("chain through the subject stays rejected", func(t *testing.T) {
		model := strings.Replace(chainRedefinitionModel, "redefines vehicle1.fuel", "redefines vehicle.fuel", 1)
		wantReport(t, check(t, binary, model, "-strict", "-validate"), exitUnevaluable,
			"error[redefinition-through-reference]")
	})
}
