package runtime

import (
	"testing"
)

func TestIntegration_ParseAndInstantiate(t *testing.T) {
	// End-to-end: parse → validate → instantiate → verify default value
	src := `
		part def Wheel {
			attribute diameter: Real = 0.5;
		}
	`

	model, resolver, rootScope := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 100000)

	wheelSym := resolveSymbol(t, rootScope, "Wheel")
	inst, err := ctx.Instantiate(wheelSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	fv, err := inst.GetFeatureValue(ctx, "diameter")
	if err != nil {
		t.Fatalf("GetFeatureValue failed: %v", err)
	}

	if fv.Value.Const.Real != 0.5 {
		t.Errorf("expected diameter=0.5, got %v", fv.Value.Const.Real)
	}
}
