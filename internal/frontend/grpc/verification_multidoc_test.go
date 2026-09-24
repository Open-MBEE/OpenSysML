package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A requirement and the cases verifying it belong to one model but not
// necessarily to one document: these two are written apart, so the verdicts the
// bodies produce are only reported if every document of the model is searched.
const (
	multiDocRequirement = `package Req {
	private import ScalarValues::*;

	part def Widget {
		attribute m : Integer default = 0;
	}

	part good : Widget;
	part bad : Widget {
		attribute :>> m = 1;
	}

	requirement def Zeroed {
		subject w : Widget;
		require constraint { w.m == 0 }
	}

	requirement zeroed : Zeroed {
		subject w = good;
	}

	part checks {
		assert satisfy zeroed by good;
	}
}
`
	multiDocVerification = `package Ver {
	private import Req::*;

	verification def Check {
		subject w : Widget;
		objective { verify zeroed; }
		VerificationCases::PassIf(w.m == 0)
	}

	verification checkGood : Check {
		subject w = Req::good;
	}

	verification checkBad : Check {
		subject w = Req::bad;
	}
}
`
)

// mustParseTwoDocuments parses both documents as one model and returns its hash.
func mustParseTwoDocuments(t *testing.T, srv *Service) string {
	t.Helper()
	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("req.sysml", multiDocRequirement, "ver.sysml", multiDocVerification),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	for _, diag := range resp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("the two documents have errors: %v", resp.Diagnostics)
		}
	}
	return resp.ModelHash
}

// TestVerifyRequirementFindsCasesInAnotherDocument verifies the body verdicts of
// a requirement's verification cases are reported when the cases are written in
// a document other than the requirement's, each case answering once.
func TestVerifyRequirementFindsCasesInAnotherDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := mustParseTwoDocuments(t, srv)

	resp, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Req::zeroed",
		SubjectSymbolId: "Req::good",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyRequirement reported %q", resp.Error)
	}
	if !resp.Verdict.Holds {
		t.Errorf("zeroed by good: holds = false, want true (%s)", resp.Verdict.Condition)
	}
	wantVerdicts(t, "VerifyRequirement across documents", resp.VerificationVerdicts,
		"Ver::checkGood=pass", "Ver::checkBad=fail")
}

// TestVerifySatisfactionFindsCasesInAnotherDocument verifies the same beside the
// verdict of a satisfaction assertion, for the named assertion and for the whole
// model.
func TestVerifySatisfactionFindsCasesInAnotherDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := mustParseTwoDocuments(t, srv)

	for _, symbol := range []string{"", "Req::checks"} {
		resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
			ModelHash: hash,
			SymbolId:  symbol,
		})
		if err != nil {
			t.Fatalf("VerifySatisfaction(%q): %v", symbol, err)
		}
		if resp.Error != "" {
			t.Fatalf("VerifySatisfaction(%q) reported %q", symbol, resp.Error)
		}
		wantVerdicts(t, "VerifySatisfaction across documents", resp.VerificationVerdicts,
			"Ver::checkGood=pass", "Ver::checkBad=fail")
	}
}
