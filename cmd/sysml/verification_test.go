package main

import (
	"encoding/json"
	"testing"
)

// verificationModel declares verification cases run by TestRunVerification: one
// whose body passes the library's PassIf, one that fails it, one binding no
// verdict at all, and one whose subject nothing binds.
const verificationModel = `package Ver {
    private import ScalarValues::*;
    part def Lander { attribute speed : Real default = 1.2; }
    part slow : Lander;
    part fast : Lander { attribute :>> speed = 2.4; }
    requirement def Touchdown { subject l : Lander; require constraint { l.speed <= 1.5 } }
    requirement touchdown : Touchdown { subject l = slow; }
    verification def SpeedCheck {
        subject l : Lander;
        objective { verify touchdown; require constraint { l.speed <= 1.5 } }
        VerificationCases::PassIf(l.speed <= 1.5)
    }
    verification checkSlow : SpeedCheck { subject l = slow; }
    verification checkFast : SpeedCheck { subject l = fast; }
    verification def Silent {
        subject l : Lander;
        objective { verify touchdown; require constraint { l.speed <= 1.5 } }
        action measure;
    }
    verification silent : Silent { subject l = slow; }
    verification unbound : SpeedCheck;
    part context { assert satisfy touchdown by slow; }
}`

// TestRunVerification checks that -analysis runs a verification case the way it
// runs an analysis case and reports the verdict its body produced.
func TestRunVerification(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, verificationModel, "-analysis", "Ver::checkSlow"), 0,
		"✓ Ver::checkSlow", "result = VerdictKind::pass",
		"✓ Verification Ver::checkSlow verdict: pass")
	wantReport(t, check(t, binary, verificationModel, "-analysis", "Ver::checkFast"), 1,
		"✗ Ver::checkFast", "result = VerdictKind::fail",
		"✗ Verification Ver::checkFast verdict: fail")
	wantReport(t, check(t, binary, verificationModel, "-analysis", "Ver::silent"), 2,
		"? Verification Ver::silent verdict: inconclusive")
	wantReport(t, check(t, binary, verificationModel, "-analysis", "Ver::unbound"), 2,
		"? Verification Ver::unbound verdict: error", "l subject is unbound")
}

// subcaseModel declares a verification plan performing two subcases, one
// passing and one failing, while binding no verdict of its own.
const subcaseModel = `package Ver {
    private import ScalarValues::*;
    part def Lander { attribute speed : Real default = 1.2; }
    part slow : Lander;
    part fast : Lander { attribute :>> speed = 2.4; }
    verification def SpeedCheck {
        subject l : Lander;
        VerificationCases::PassIf(l.speed <= 1.5)
    }
    verification plan {
        subject l = slow;
        verification checkSlow : SpeedCheck { subject l = Ver::slow; }
        verification checkFast : SpeedCheck { subject l = Ver::fast; }
    }
}`

// TestVerificationSubcasesAreMarked checks that a verdict a case produced as a
// step of another is reported on its own line, marked as a subcase, and carries
// that mark in the JSON report.
func TestVerificationSubcasesAreMarked(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, subcaseModel, "-analysis", "Ver::plan"), 2,
		"? Verification Ver::plan verdict: inconclusive",
		"✓ Verification Ver::plan::checkSlow verdict: pass (subcase)",
		"✗ Verification Ver::plan::checkFast verdict: fail (subcase)")

	got := check(t, binary, subcaseModel, "-analysis", "Ver::plan", "-json")
	var report struct {
		Checks []struct {
			Verifications []struct {
				Case    string `json:"case"`
				Kind    string `json:"kind"`
				Subcase bool   `json:"subcase"`
			} `json:"verifications"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 {
		t.Fatalf("the report states no check:\n%s", got.stdout)
	}
	subcases := map[string]bool{}
	for _, v := range report.Checks[0].Verifications {
		subcases[v.Case] = v.Subcase
	}
	for name, want := range map[string]bool{
		"Ver::plan": false, "Ver::plan::checkSlow": true, "Ver::plan::checkFast": true,
	} {
		if got, ok := subcases[name]; !ok || got != want {
			t.Errorf("%s reports subcase %v (present %v), want %v", name, got, ok, want)
		}
	}
}

// TestVerificationVerdictsBesideRequirements checks that the requirement and
// satisfaction surfaces report the body verdicts beside their own verdict,
// which stays what the requirement engine decided, and that the JSON report
// carries them as data.
func TestVerificationVerdictsBesideRequirements(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, verificationModel, "-requirement", "Ver::touchdown"), 0,
		"✓ Requirement Ver::touchdown satisfied",
		"✓ Verification Ver::checkSlow verdict: pass",
		"✗ Verification Ver::checkFast verdict: fail",
		"? Verification Ver::silent verdict: inconclusive")
	wantReport(t, check(t, binary, verificationModel, "-satisfy"), 0,
		"✓ satisfy touchdown by slow holds",
		"✓ Verification Ver::checkSlow verdict: pass",
		"✗ Verification Ver::checkFast verdict: fail")

	got := check(t, binary, verificationModel, "-requirement", "Ver::touchdown", "-json")
	var report struct {
		Checks []struct {
			Status        string `json:"status"`
			Verifications []struct {
				Case   string `json:"case"`
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"verifications"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 || report.Checks[0].Status != "holds" {
		t.Fatalf("the report does not state the requirement's own verdict:\n%s", got.stdout)
	}
	kinds := map[string]string{}
	for _, v := range report.Checks[0].Verifications {
		kinds[v.Case] = v.Kind
	}
	for name, want := range map[string]string{
		"Ver::checkSlow": "pass", "Ver::checkFast": "fail", "Ver::silent": "inconclusive",
	} {
		if kinds[name] != want {
			t.Errorf("%s reports verdict %q, want %q", name, kinds[name], want)
		}
	}
}
