package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// validateModelSource holds a car whose engine, three wheels and injector each
// carry assertions, some holding and some failing, with a satisfaction stated
// about the nested engine, so one object exercises every verdict a validation
// reports and every depth the walk reaches.
const validateModelSource = `package Demo {
	part def Injector {
		attribute rate = 2.0;
		assert constraint ratePositive { rate > 0.0 }
	}

	part def Engine {
		attribute power = 300.0;
		part injector : Injector;
		assert constraint { power < 200.0 }
	}

	part def Wheel {
		attribute pressure default = 32.0;
		assert constraint pressureOk { pressure >= 30.0 }
	}

	part def Car {
		attribute mass = 1500.0;
		part engine : Engine;
		part wheels : Wheel[3] {
			attribute :>> pressure = 20.0;
		}
		assert constraint massOk { mass < 2000.0 }
		requirement lightEnough {
			attribute m = mass;
			require constraint { m < 1600.0 }
		}
	}

	requirement def PowerReq {
		subject e : Engine;
		require constraint { e.power > 100.0 }
	}

	part car : Car;
	requirement strongEngine : PowerReq;
	satisfy strongEngine by car.engine;

	part def Loose {
		attribute a;
		assert constraint { a > 0.0 }
	}
	part loose : Loose;

	part sound : Wheel;

	part def Crate;
	part crate : Crate;
}
`

// TestValidateInstanceReportsEveryAssertionOnEveryObject verifies a validation
// walks the object held to every depth and answers every assertion about each
// object reached, naming the object by its path.
func TestValidateInstanceReportsEveryAssertionOnEveryObject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, validateModelSource, "validate-instance")

	resp, err := srv.ValidateInstance(context.Background(), &pb.ValidateInstanceRequest{
		ModelHash: hash,
		SymbolId:  "Demo::car",
	})
	if err != nil {
		t.Fatalf("ValidateInstance: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("ValidateInstance reported %q", resp.Error)
	}

	type want struct {
		kind, element, path string
		holds               bool
	}
	wants := []want{
		{"constraint", "Demo::Car::massOk", "", true},
		{"requirement", "Demo::Car::lightEnough", "", true},
		{"constraint", "", "engine", false},
		{"constraint", "Demo::Injector::ratePositive", "engine.injector", true},
		{"constraint", "Demo::Wheel::pressureOk", "wheels[1]", false},
		{"constraint", "Demo::Wheel::pressureOk", "wheels[2]", false},
		{"constraint", "Demo::Wheel::pressureOk", "wheels[3]", false},
		{"satisfy", "", "engine", true},
	}
	if len(resp.Verdicts) != len(wants) {
		t.Fatalf("got %d verdicts, want %d: %v", len(resp.Verdicts), len(wants), resp.Verdicts)
	}
	for i, w := range wants {
		got := resp.Verdicts[i]
		if got.Kind != w.kind || got.ElementId != w.element || got.InstancePath != w.path || got.Holds != w.holds {
			t.Errorf("verdict %d = %s %q at %q holds=%v, want %s %q at %q holds=%v",
				i, got.Kind, got.ElementId, got.InstancePath, got.Holds, w.kind, w.element, w.path, w.holds)
		}
		if got.Error != "" {
			t.Errorf("verdict %d: a false condition is a verdict, not an error: %q", i, got.Error)
		}
		if !got.Holds && got.Condition == "" {
			t.Errorf("verdict %d fails but names no condition", i)
		}
		if got.InstanceId == 0 {
			t.Errorf("verdict %d names no instance", i)
		}
	}
	satisfy := resp.Verdicts[7]
	if satisfy.RequirementId != "Demo::strongEngine" {
		t.Errorf("satisfaction verdict requirement_id = %q, want Demo::strongEngine", satisfy.RequirementId)
	}
	if satisfy.Element != "satisfy strongEngine by car.engine" {
		t.Errorf("satisfaction verdict element = %q", satisfy.Element)
	}

	if resp.Summary == nil {
		t.Fatal("no summary verdict")
	}
	if resp.Summary.Kind != "object" || resp.Summary.ElementId != "Demo::car" || resp.Summary.InstanceTypeId != "Demo::car" {
		t.Errorf("summary = %v, want an object verdict about an object of Demo::car", resp.Summary)
	}
	if resp.Summary.Holds || resp.Summary.Error != "" {
		t.Errorf("summary holds=%v error=%q, want a plain verdict of false", resp.Summary.Holds, resp.Summary.Error)
	}
	if resp.Summary.InstanceId != resp.Verdicts[0].InstanceId {
		t.Errorf("summary is about instance %d, the root's assertions about %d", resp.Summary.InstanceId, resp.Verdicts[0].InstanceId)
	}
	if resp.Summary.Engine == "" {
		t.Error("summary names no engine")
	}
	if resp.Bounded {
		t.Error("a finite object tree was reported bounded")
	}
	// The car, the engine, the injector, three wheels and the requirement the
	// car carries.
	if len(resp.Instances) != 7 {
		t.Errorf("got %d instances, want 7", len(resp.Instances))
	}
}

// TestValidateInstanceOfAValidObjectHolds verifies an object every assertion
// about which holds gets a summary that holds.
func TestValidateInstanceOfAValidObjectHolds(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, validateModelSource, "validate-instance")

	resp, err := srv.ValidateInstance(context.Background(), &pb.ValidateInstanceRequest{
		ModelHash: hash,
		SymbolId:  "Demo::sound",
	})
	if err != nil {
		t.Fatalf("ValidateInstance: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("ValidateInstance reported %q", resp.Error)
	}
	if len(resp.Verdicts) != 1 || !resp.Verdicts[0].Holds {
		t.Fatalf("verdicts = %v, want one that holds", resp.Verdicts)
	}
	if !resp.Summary.Holds || resp.Summary.Error != "" {
		t.Errorf("summary holds=%v error=%q, want it to hold", resp.Summary.Holds, resp.Summary.Error)
	}
}

// TestValidateInstanceUndecidedIsNotAVerdict verifies an assertion that cannot
// be evaluated leaves both its own verdict and the summary undecided, with the
// reason, rather than answering false.
func TestValidateInstanceUndecidedIsNotAVerdict(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, validateModelSource, "validate-instance")

	resp, err := srv.ValidateInstance(context.Background(), &pb.ValidateInstanceRequest{
		ModelHash: hash,
		SymbolId:  "Demo::loose",
	})
	if err != nil {
		t.Fatalf("ValidateInstance: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("an unevaluable assertion is an undecided verdict, not a failure: %q", resp.Error)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("verdicts = %v, want one", resp.Verdicts)
	}
	v := resp.Verdicts[0]
	if v.Holds || v.Error == "" || v.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Errorf("verdict = %v, want undecided for an evaluation failure", v)
	}
	if resp.Summary.Holds || !strings.Contains(resp.Summary.Error, "1 undecided") {
		t.Errorf("summary holds=%v error=%q, want undecided naming the one undecided assertion",
			resp.Summary.Holds, resp.Summary.Error)
	}
	if resp.Summary.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Errorf("summary failure_reason = %v, want EVALUATION", resp.Summary.FailureReason)
	}
}

// TestValidateInstanceStatingNoAssertionDecidesNothing verifies an object no assertion
// is about is not shown valid: the summary neither holds nor is a violation, and says why.
func TestValidateInstanceStatingNoAssertionDecidesNothing(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, validateModelSource, "validate-instance")

	resp, err := srv.ValidateInstance(context.Background(), &pb.ValidateInstanceRequest{
		ModelHash: hash,
		SymbolId:  "Demo::crate",
	})
	if err != nil {
		t.Fatalf("ValidateInstance: %v", err)
	}
	if resp.Error != "" || len(resp.Verdicts) != 0 {
		t.Fatalf("error=%q verdicts=%v, want an answer with no verdict", resp.Error, resp.Verdicts)
	}
	if resp.Summary == nil || resp.Summary.Holds || !strings.Contains(resp.Summary.Error, "states no assertion") {
		t.Errorf("summary = %v, want undecided for want of an assertion", resp.Summary)
	}
	if resp.Summary.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Errorf("summary failure_reason = %v, want EVALUATION", resp.Summary.FailureReason)
	}
}

// TestValidateInstanceRefusesWhatIsNoPart verifies naming nothing, an unknown symbol or
// one with no object (a package, an attribute) is a failure, the last of the wrong kind.
func TestValidateInstanceRefusesWhatIsNoPart(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, validateModelSource, "validate-instance")

	for name, tc := range map[string]struct {
		symbol string
		reason pb.FailureReason
	}{
		"nothing":   {"", pb.FailureReason_FAILURE_REASON_UNSPECIFIED},
		"unknown":   {"Demo::nosuch", pb.FailureReason_FAILURE_REASON_EVALUATION},
		"package":   {"Demo", pb.FailureReason_FAILURE_REASON_WRONG_KIND},
		"attribute": {"Demo::Car::mass", pb.FailureReason_FAILURE_REASON_WRONG_KIND},
	} {
		resp, err := srv.ValidateInstance(context.Background(), &pb.ValidateInstanceRequest{
			ModelHash: hash,
			SymbolId:  tc.symbol,
		})
		if err != nil {
			t.Fatalf("ValidateInstance(%s): %v", name, err)
		}
		if resp.Error == "" {
			t.Errorf("ValidateInstance(%s) reported no failure: %v", name, resp)
		}
		if resp.FailureReason != tc.reason {
			t.Errorf("ValidateInstance(%s) failure_reason = %v (%s), want %v", name, resp.FailureReason, resp.Error, tc.reason)
		}
		if resp.Summary != nil || len(resp.Verdicts) != 0 {
			t.Errorf("ValidateInstance(%s) reported verdicts beside its failure", name)
		}
	}
}
