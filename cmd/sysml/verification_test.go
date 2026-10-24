package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		"✓ Ver::checkSlow", "result = [VerdictKind::pass]",
		"✓ Verification Ver::checkSlow verdict: pass")
	wantReport(t, check(t, binary, verificationModel, "-analysis", "Ver::checkFast"), 1,
		"✗ Ver::checkFast", "result = [VerdictKind::fail]",
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
		"✓ Ver::checkScout", "result = [VerdictKind::pass]",
		"objective obj: satisfied",
		"✓ Verification Ver::checkScout verdict: pass")
	heavy := check(t, binary, objectiveSubjectModel, "-analysis", "Ver::checkHeavy")
	wantReport(t, heavy, 1,
		"✗ Ver::checkHeavy", "result = [VerdictKind::fail]",
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

// definitionOwnedModel declares a verification case inside a part definition,
// whose body reads the owning object's cost.
const definitionOwnedModel = `package Cost {
    private import ScalarValues::Real;
    requirement def CostRequirement {
        subject product : Definition;
        attribute maximumCost : Real;
        require constraint costOk { product.cost <= maximumCost }
    }
    requirement costReq : CostRequirement { :>> maximumCost = 100.0; }
    part def Definition {
        attribute cost : Real;
        satisfy costReq;
        verification costTest {
            objective costObjective { verify costReq; }
            return verdict = VerificationCases::PassIf(cost <= 100.0);
        }
    }
    part cheap : Definition { attribute :>> cost = 80.0; }
    part dear : Definition { attribute :>> cost = 120.0; }
}`

// TestRunVerificationOnADefinitionOwnedCase checks that -analysis runs a case a
// part definition owns on the object named after it, and that -validate and
// -requirement run it on the object they check, naming the object.
func TestRunVerificationOnADefinitionOwnedCase(t *testing.T) {
	binary := buildCLI(t)

	// The objective states only `verify costReq`, so it is satisfied and the
	// case's status is the body's verdict, reported beside it.
	wantReport(t, check(t, binary, definitionOwnedModel, "-instantiate", "Cost::cheap", "-analysis", "Cost::Definition::costTest Cost::cheap"), 0,
		"✓ Cost::Definition::costTest", "objective costObjective: satisfied",
		"verdict = [VerdictKind::pass]",
		"✓ Verification Cost::Definition::costTest on Cost::cheap (object #1) verdict: pass")
	wantReport(t, check(t, binary, definitionOwnedModel, "-instantiate", "Cost::dear", "-analysis", "Cost::Definition::costTest Cost::dear"), 1,
		"✗ Cost::Definition::costTest", "verdict = [VerdictKind::fail]",
		"✗ Verification Cost::Definition::costTest on Cost::dear (object #1) verdict: fail")
	wantReport(t, check(t, binary, definitionOwnedModel, "-instantiate", "Cost::dear", "-validate=Cost::dear"), 1,
		"✗ satisfy costReq fails (on Cost::dear ID: 1)",
		"✗ Verification Cost::Definition::costTest on Cost::dear (object #1) verdict: fail")
	wantReport(t, check(t, binary, definitionOwnedModel, "-instantiate", "Cost::cheap", "-requirement", "Cost::costReq Cost::cheap"), 0,
		"✓ Requirement Cost::costReq satisfied (on Cost::cheap ID: 1)",
		"✓ Verification Cost::Definition::costTest on Cost::cheap (object #1) verdict: pass")
	wantReport(t, check(t, binary, definitionOwnedModel, "-analysis", "Cost::Definition::costTest"), 2,
		"? Verification Cost::Definition::costTest verdict: inconclusive — needs an object of Cost::Definition: none is instantiated")
}

// conformanceFixtureDir holds the execution-conformance fixtures, whose instance
// cases pin a validation's verdicts and the verification runs reported beside them.
const conformanceFixtureDir = "../../internal/exec/runtime/testdata/conformance"

// validationFixture is the part of an instance fixture a validation report states.
type validationFixture struct {
	Documents   []string `json:"documents"`
	Instantiate string   `json:"instantiate"`
	Validation  struct {
		Verdicts []struct {
			Assertion     string `json:"assertion"`
			Status        string `json:"status"`
			Verifications []struct {
				Case    string `json:"case"`
				Verdict string `json:"verdict"`
			} `json:"verifications"`
		} `json:"verdicts"`
		Valid *bool `json:"valid"`
	} `json:"validation"`
}

// TestValidateAgreesWithConformanceFixtures holds -validate over each fixture model
// pinning verification runs to the satisfaction and verification verdicts it pins.
func TestValidateAgreesWithConformanceFixtures(t *testing.T) {
	binary := buildCLI(t)
	paths, err := filepath.Glob(filepath.Join(conformanceFixtureDir, "validation_verification_*.expected.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no validation_verification fixtures found: %v", err)
	}
	statuses := map[string]string{"holds": "holds", "violated": "fails", "undecided": "undecided"}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".expected.json")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var fixture validationFixture
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			args := []string{filepath.Join(conformanceFixtureDir, name+".sysml")}
			for _, doc := range fixture.Documents {
				args = append(args, filepath.Join(conformanceFixtureDir, doc))
			}
			args = append(args, "-instantiate", fixture.Instantiate, "-validate="+fixture.Instantiate, "-json")
			got := checkPaths(t, binary, args...)
			var report struct {
				Exit   int `json:"exit"`
				Checks []struct {
					Subject       string `json:"subject"`
					Status        string `json:"status"`
					Verifications []struct {
						Case string `json:"case"`
						Kind string `json:"kind"`
					} `json:"verifications"`
				} `json:"checks"`
			}
			if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
				t.Fatalf("report: %v\n%s", err, got.output())
			}
			if got.status != report.Exit {
				t.Errorf("process exit %d, report exit %d", got.status, report.Exit)
			}
			if fixture.Validation.Valid != nil && (report.Exit == 0) != *fixture.Validation.Valid {
				t.Errorf("exit %d, fixture valid = %v", report.Exit, *fixture.Validation.Valid)
			}
			verdicts := fixture.Validation.Verdicts
			if len(report.Checks) != len(verdicts)+1 {
				t.Fatalf("%d checks for %d verdicts and the summary:\n%s", len(report.Checks), len(verdicts), got.output())
			}
			for i, want := range verdicts {
				check := report.Checks[i]
				if !strings.HasPrefix(check.Subject, want.Assertion+" on ") || check.Status != statuses[want.Status] {
					t.Errorf("check %d: %s %s, fixture pins %s %s", i+1, check.Subject, check.Status, want.Assertion, want.Status)
				}
				if want.Verifications == nil {
					continue
				}
				if len(check.Verifications) != len(want.Verifications) {
					t.Errorf("check %d: %d verification(s), fixture pins %d", i+1, len(check.Verifications), len(want.Verifications))
					continue
				}
				for j, run := range want.Verifications {
					if v := check.Verifications[j]; v.Case != run.Case || v.Kind != run.Verdict {
						t.Errorf("check %d verification %d: %s %s, fixture pins %s %s", i+1, j+1, v.Case, v.Kind, run.Case, run.Verdict)
					}
				}
			}
		})
	}
}
