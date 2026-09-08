package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestCapabilityAvailabilityDrivesAdvertisementAndRefusal(t *testing.T) {
	ctx := context.Background()
	for _, capability := range Capabilities() {
		t.Run(capability, func(t *testing.T) {
			available := mustNewService(t, 10)
			t.Cleanup(available.Close)
			info, err := available.GetServerInfo(ctx, &pb.ServerInfoRequest{})
			if err != nil {
				t.Fatalf("GetServerInfo available: %v", err)
			}
			if !slices.Contains(info.Capabilities, capability) {
				t.Fatalf("available service does not advertise %q", capability)
			}
			if err := available.requireCapability(capability); err != nil {
				t.Errorf("advertised capability %q was refused: %v", capability, err)
			}

			unavailable := mustNewServiceWithout(t, capability)
			info, err = unavailable.GetServerInfo(ctx, &pb.ServerInfoRequest{})
			if err != nil {
				t.Fatalf("GetServerInfo unavailable: %v", err)
			}
			if slices.Contains(info.Capabilities, capability) {
				t.Fatalf("unavailable service advertises %q", capability)
			}
			err = unavailable.requireCapability(capability)
			if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), capability) {
				t.Errorf("refusal = %v, want UNIMPLEMENTED naming %q", err, capability)
			}
		})
	}
}

func TestUnavailableCapabilityMustBeKnown(t *testing.T) {
	if _, err := NewServiceWithUnavailableCapabilitiesForTesting(10, "test", []string{"not-a-capability"}); err == nil {
		t.Fatal("unknown unavailable capability was accepted")
	}
}

func TestCapabilityGatedRequestsAreRefused(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		capability string
		call       func(*Service) error
	}{
		{"convert", CapabilityConvert, func(s *Service) error {
			_, err := s.Convert(ctx, &pb.ConvertRequest{})
			return err
		}},
		{"query", CapabilityQuery, func(s *Service) error {
			_, err := s.Query(ctx, &pb.QueryRequest{})
			return err
		}},
		{"oslc query", CapabilityOSLCQuery, func(s *Service) error {
			_, err := s.Query(ctx, &pb.QueryRequest{OslcQuery: "oslc.where=rdf:type=\"PartUsage\""})
			return err
		}},
		{"apply edits", CapabilityApplyEdits, func(s *Service) error {
			_, err := s.ApplyEdits(ctx, &pb.ApplyEditsRequest{})
			return err
		}},
		{"authoring", CapabilityAuthoring, func(s *Service) error {
			_, err := s.ApplyEdits(ctx, &pb.ApplyEditsRequest{
				Operations: []*pb.EditOperation{addMemberOp("", "part def", "P")},
			})
			return err
		}},
		{"inline language", CapabilityInlineLanguage, func(s *Service) error {
			_, err := s.ParseFile(ctx, &pb.ParseFileRequest{
				Source:   &pb.ParseFileRequest_Content{Content: "package P;"},
				Language: "sysml",
			})
			return err
		}},
		{"strict conformance", CapabilityStrictConformance, func(s *Service) error {
			_, err := s.ParseFile(ctx, &pb.ParseFileRequest{
				Source:            &pb.ParseFileRequest_Content{Content: "package P;"},
				StrictConformance: true,
			})
			return err
		}},
		{"evaluate subject", CapabilityEvaluateSubject, func(s *Service) error {
			_, err := s.Evaluate(ctx, &pb.EvaluateRequest{SubjectSymbolId: "P"})
			return err
		}},
		{"verify constraint", CapabilityVerification, func(s *Service) error {
			_, err := s.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{})
			return err
		}},
		{"verify requirement", CapabilityVerification, func(s *Service) error {
			_, err := s.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{})
			return err
		}},
		{"verify satisfaction", CapabilityVerification, func(s *Service) error {
			_, err := s.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{})
			return err
		}},
		{"evaluate calc", CapabilityVerification, func(s *Service) error {
			_, err := s.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{})
			return err
		}},
		{"run analysis", CapabilityVerification, func(s *Service) error {
			_, err := s.RunAnalysis(ctx, &pb.RunAnalysisRequest{})
			return err
		}},
		{"execute action schedule", CapabilitySchedule, func(s *Service) error {
			_, err := s.ExecuteAction(ctx, &pb.ExecuteActionRequest{Schedule: "declared"})
			return err
		}},
		{"execute state schedule", CapabilitySchedule, func(s *Service) error {
			_, err := s.ExecuteState(ctx, &pb.ExecuteStateRequest{Schedule: "declared"})
			return err
		}},
		{"run analysis schedule", CapabilitySchedule, func(s *Service) error {
			_, err := s.RunAnalysis(ctx, &pb.RunAnalysisRequest{Schedule: "declared"})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call(mustNewServiceWithout(t, test.capability))
			if connect.CodeOf(err) != connect.CodeUnimplemented {
				t.Fatalf("status = %s, want UNIMPLEMENTED: %v", connect.CodeOf(err), err)
			}
			if !strings.Contains(err.Error(), test.capability) {
				t.Errorf("message = %q, want capability %q", err.Error(), test.capability)
			}
		})
	}
}

func TestUnsetCapabilityGatedFieldsKeepWorking(t *testing.T) {
	ctx := context.Background()
	source := &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: "package P;"},
	}
	for _, capability := range []string{CapabilityStrictConformance, CapabilityInlineLanguage} {
		srv := mustNewServiceWithout(t, capability)
		if _, err := srv.ParseFile(ctx, source); err != nil {
			t.Errorf("ParseFile without %s field: %v", capability, err)
		}
	}

	srv := mustNewServiceWithout(t, CapabilityEvaluateSubject)
	parsed, err := srv.ParseFile(ctx, source)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if _, err := srv.Evaluate(ctx, &pb.EvaluateRequest{
		ModelHash:  parsed.ModelHash,
		Expression: "2 + 2",
	}); err != nil {
		t.Errorf("Evaluate without subject: %v", err)
	}

	srv = mustNewServiceWithout(t, CapabilityOSLCQuery)
	_, err = srv.Query(ctx, &pb.QueryRequest{ModelHash: "missing", Query: &pb.Query{}})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("structured Query status = %s, want NOT_FOUND: %v", connect.CodeOf(err), err)
	}

	srv = mustNewServiceWithout(t, CapabilityAuthoring)
	_, err = srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: "missing",
		Operations: []*pb.EditOperation{{Operation: &pb.EditOperation_Rename{
			Rename: &pb.RenameEdit{Target: "P", NewName: "Q"},
		}}},
	})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("non-authoring edit status = %s, want NOT_FOUND: %v", connect.CodeOf(err), err)
	}
}

func TestResponseCapabilitiesControlPopulationWithoutRefusing(t *testing.T) {
	ctx := context.Background()
	srv := mustNewServiceWithout(t,
		CapabilityTypeFacts,
		CapabilitySymbolAttributes,
		CapabilityFeatureValues,
		CapabilityEnumValues,
		CapabilityUnsetValue,
	)
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: `
enum def Color { red; }
part def P {
	attribute c : Color = Color::red;
	attribute n : Integer;
}
`},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	symbol, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{
		ModelHash: parsed.ModelHash,
		SymbolId:  "P::c",
	})
	if err != nil {
		t.Fatalf("GetSymbol: %v", err)
	}
	if symbol.Symbol.GetTypeInfo() != nil || symbol.Symbol.GetMultiplicity() != nil ||
		len(symbol.Symbol.GetSpecializations()) != 0 || len(symbol.Symbol.GetAttributes()) != 0 {
		t.Errorf("response-only facts were populated: %+v", symbol.Symbol)
	}
	instantiated, err := srv.Instantiate(ctx, &pb.InstantiateRequest{
		ModelHash: parsed.ModelHash,
		SymbolId:  "P",
	})
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if len(instantiated.Instance.GetFeatureValues()) != 0 {
		t.Errorf("feature_values = %v, want none", instantiated.Instance.GetFeatureValues())
	}
}

func TestUnavailableValueCapabilitiesUseUnsupportedNull(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityEnumValues, CapabilityUnsetValue)
	enumValue := &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: &pb.EnumLiteral{Name: "red"}}}
	unsetValue := &pb.Value{Kind: &pb.Value_Unset{Unset: true}}
	srv.filterValueCapabilities(enumValue)
	srv.filterValueCapabilities(unsetValue)
	if !strings.Contains(enumValue.GetNull(), "enumeration") {
		t.Errorf("enum value = %v, want unsupported null", enumValue)
	}
	if !strings.Contains(unsetValue.GetNull(), "unset") {
		t.Errorf("unset value = %v, want unsupported null", unsetValue)
	}
}
