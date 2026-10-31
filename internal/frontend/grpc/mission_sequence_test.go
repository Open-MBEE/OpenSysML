package grpc

import (
	"context"
	"os"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func missionSequenceRequest(t *testing.T, service *Service, performer string, events ...string) *pb.ExecuteActionResponse {
	t.Helper()
	source, err := os.ReadFile("../../../examples/runtime-showcase/mission-sequence.sysml")
	if err != nil {
		t.Fatalf("reading the mission example: %v", err)
	}
	modelHash := parseStateTraceModel(t, service, string(source))
	response, err := service.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:         modelHash,
		ActionSymbolId:    "MissionSequence::LunarMission",
		PerformerSymbolId: performer,
		Trace:             true,
		Events:            events,
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	return response
}

func TestMissionSequenceApollo11WaitsForGoThenLands(t *testing.T) {
	service := mustNewService(t, 10)
	waiting := missionSequenceRequest(t, service, "MissionSequence::apollo11")
	if waiting.Error == "" || !strings.Contains(waiting.Error, "GoForDescent") {
		t.Fatalf("apollo11 without events: error %q, want the wait on GoForDescent", waiting.Error)
	}
	last := waiting.Trace[len(waiting.Trace)-1]
	if last.Kind != "wait" || last.Event != "GoForDescent" {
		t.Fatalf("last record %s %q, want wait GoForDescent", last.Kind, last.Event)
	}

	landed := missionSequenceRequest(t, service, "MissionSequence::apollo11", "GoForDescent")
	if landed.Error != "" {
		t.Fatalf("apollo11 with GoForDescent: %s", landed.Error)
	}
	texts := traceTexts(landed.Trace)
	for _, want := range []string{"accept GoForDescent", "step awaitGo -> descent", "step transEarthInjection -> splashdown", "complete"} {
		if !containsText(texts, want) {
			t.Errorf("trace lacks %q:\n%s", want, strings.Join(texts, "\n"))
		}
	}
	for _, out := range []string{"landed", "returned"} {
		if got := landed.Outputs[out]; got == nil || !got.GetSequence().GetElements()[0].GetBoolValue() {
			t.Errorf("%s = %v, want true", out, got)
		}
	}
}

func TestMissionSequenceApollo8SkipsTheLanding(t *testing.T) {
	service := mustNewService(t, 10)
	flown := missionSequenceRequest(t, service, "MissionSequence::apollo8")
	if flown.Error != "" {
		t.Fatalf("apollo8: %s", flown.Error)
	}
	texts := traceTexts(flown.Trace)
	if !containsText(texts, "transEarthInjection") || containsText(texts, "step awaitGo") {
		t.Fatalf("apollo8 trace, want the trans-Earth branch and no awaitGo:\n%s", strings.Join(texts, "\n"))
	}
	if got := flown.Outputs["landed"]; got == nil || got.GetSequence().GetElements()[0].GetBoolValue() {
		t.Errorf("landed = %v, want false", got)
	}
}

func containsText(texts []string, want string) bool {
	for _, text := range texts {
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}
