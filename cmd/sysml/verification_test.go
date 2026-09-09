package main

import (
	"encoding/json"
	"strings"
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

// objectiveSubjectModel declares a verification whose objective is a requirement
// on a Lander named differently from the verification's subject; the library
// binds that subject to the verification's, so the objective decides.
const objectiveSubjectModel = `package Ver {
    private import ScalarValues::*;
    part def Lander { attribute touchdownSpeed : Real; }
    part scout : Lander { attribute :>> touchdownSpeed = 1.2; }
    part heavy : Lander { attribute :>> touchdownSpeed = 1.6; }
    requirement def SoftLanding {
        subject lander : Lander;
        in attribute limit : Real default = 1.5;
        require constraint { lander.touchdownSpeed <= limit }
    }
    requirement softLanding : SoftLanding { subject lander = scout; }
    verification def TouchdownCheck {
        subject lander : Lander;
        in attribute limit : Real = 1.5;
        objective : SoftLanding { in limit = limit; verify softLanding; }
        VerificationCases::PassIf(lander.touchdownSpeed <= limit)
    }
    verification checkScout : TouchdownCheck { subject lander = scout; }
    verification checkHeavy : TouchdownCheck { subject lander = heavy; }
    part context { assert satisfy softLanding by scout; }
}`

// TestVerificationObjectiveDecidesOnEverySurface checks that the objective of
// a verification case decides against the verification's subject, agreeing
// with the body's verdict under -analysis, and that -requirement and -satisfy
// report the same body verdicts for the cases verifying the requirement.
func TestVerificationObjectiveDecidesOnEverySurface(t *testing.T) {
	binary := buildCLI(t)

	scout := check(t, binary, objectiveSubjectModel, "-analysis", "Ver::checkScout")
	wantReport(t, scout, 0,
		"✓ Ver::checkScout", "result = VerdictKind::pass",
		"objective obj: satisfied",
		"✓ Verification Ver::checkScout verdict: pass")
	heavy := check(t, binary, objectiveSubjectModel, "-analysis", "Ver::checkHeavy")
	wantReport(t, heavy, 1,
		"✗ Ver::checkHeavy", "result = VerdictKind::fail",
		"objective obj: not satisfied: lander.touchdownSpeed <= limit",
		"✗ Verification Ver::checkHeavy verdict: fail")
	for _, got := range []runOutcome{scout, heavy} {
		if out := got.output(); strings.Contains(out, "undecided") || strings.Contains(out, "Cases::Case::obj") {
			t.Errorf("the objective did not decide against the verification's subject:\n%s", out)
		}
	}

	wantReport(t, check(t, binary, objectiveSubjectModel, "-requirement", "Ver::softLanding"), 0,
		"✓ Requirement Ver::softLanding satisfied",
		"✓ Verification Ver::checkScout verdict: pass",
		"✗ Verification Ver::checkHeavy verdict: fail")
	wantReport(t, check(t, binary, objectiveSubjectModel, "-satisfy"), 0,
		"✓ satisfy softLanding by scout holds",
		"✓ Verification Ver::checkScout verdict: pass",
		"✗ Verification Ver::checkHeavy verdict: fail")
}
