package opensysml_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const vehicleSource = `package Demo {
	part def Vehicle {
		attribute mass default = 1500.0;
	}
	part sedan : Vehicle {
		attribute :>> mass = 1800.0;
	}
}`

func newClient(t *testing.T) opensysml.Client {
	t.Helper()
	client, err := opensysml.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func parseVehicle(t *testing.T, client opensysml.Client) *opensysml.Model {
	t.Helper()
	model, err := client.ParseSource(context.Background(), vehicleSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	return model
}

func TestServerInfoReportsCapabilities(t *testing.T) {
	client := newClient(t)
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if !info.Has(opensysml.CapabilityEvaluateSubject) {
		t.Errorf("capabilities %v do not include %s", info.Capabilities, opensysml.CapabilityEvaluateSubject)
	}
	if info.Has("no_such_capability") {
		t.Error("Has answered true for a capability the service does not report")
	}
}

func TestParseSourceAnswersAModel(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	if model.Hash == "" {
		t.Error("model has no hash")
	}
	if model.Root == nil {
		t.Fatal("model has no root")
	}
	if len(model.Diagnostics) != 0 {
		t.Errorf("clean source has diagnostics: %v", model.Diagnostics)
	}
}

func TestASyntaxErrorIsADiagnosticNotAnError(t *testing.T) {
	client := newClient(t)
	model, err := client.ParseSource(context.Background(), "part def {")
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if len(model.Diagnostics) == 0 {
		t.Error("broken source parsed without diagnostics")
	}
	for _, diag := range model.Diagnostics {
		if diag.Code != "syntax" {
			t.Errorf("syntax error coded %q, want syntax: %s", diag.Code, diag.Message)
		}
	}
}

func TestADiagnosticCarriesItsCode(t *testing.T) {
	client := newClient(t)
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if !info.Has(opensysml.CapabilityDiagnosticCodes) {
		t.Errorf("capabilities %v do not include %s", info.Capabilities, opensysml.CapabilityDiagnosticCodes)
	}
	model, err := client.ParseSource(context.Background(), "package P { part def W { part hub : Missing; } }")
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	found := false
	for _, diag := range model.Diagnostics {
		if diag.Code == "" {
			t.Errorf("diagnostic without a code: %s", diag.Message)
		}
		found = found || diag.Code == "unresolved"
	}
	if !found {
		t.Errorf("no unresolved diagnostic among %v", model.Diagnostics)
	}
}

func TestAMissingFileIsNotFound(t *testing.T) {
	client := newClient(t)
	_, err := client.ParseFile(context.Background(), "/nonexistent/path/model.sysml")
	if !errors.Is(err, opensysml.CodeNotFound) {
		t.Errorf("err = %v, want CodeNotFound", err)
	}
}

func TestAnUnknownModelIsNotFound(t *testing.T) {
	client := newClient(t)
	_, err := client.Evaluate(context.Background(), &opensysml.Model{Hash: "no-such-model"}, "2 + 2")
	if !errors.Is(err, opensysml.CodeNotFound) {
		t.Errorf("err = %v, want CodeNotFound", err)
	}
	var statusErr *opensysml.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("err = %T, want *StatusError", err)
	}
	if statusErr.Code != opensysml.CodeNotFound {
		t.Errorf("code = %v, want CodeNotFound", statusErr.Code)
	}
}

func TestANilModelIsInvalid(t *testing.T) {
	client := newClient(t)
	_, err := client.Evaluate(context.Background(), nil, "2 + 2")
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}

func TestEvaluateAnswersATypedValue(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	value, err := client.Evaluate(context.Background(), model, "2 + 2")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got, ok := value.(opensysml.Int); !ok || got != 4 {
		t.Errorf("value = %#v, want Int(4)", value)
	}
}

func TestEvaluateAgainstASubjectReadsItsValue(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	value, err := client.Evaluate(context.Background(), model, "mass",
		opensysml.WithSubject("Demo::sedan"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got, ok := value.(opensysml.Real); !ok || got != 1800.0 {
		t.Errorf("value = %#v, want Real(1800)", value)
	}
}

func TestAnUnparsableExpressionIsAFailureNotAStatus(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	_, err := client.Evaluate(context.Background(), model, "invalid syntax (((")
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
	var failure *opensysml.FailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %T, want *FailureError", err)
	}
	if failure.Op != "Evaluate" || failure.Message == "" {
		t.Errorf("failure = %+v, want Op Evaluate with a message", failure)
	}
}

func TestLookupSymbolFindsADefinition(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	symbol, err := client.LookupSymbol(context.Background(), model, "Demo::Vehicle")
	if err != nil {
		t.Fatalf("LookupSymbol: %v", err)
	}
	if symbol.Name != "Vehicle" || symbol.Kind == "" {
		t.Errorf("symbol = %+v, want name Vehicle with a kind", symbol)
	}
}

func TestAnUnknownSymbolIsAFailure(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	_, err := client.LookupSymbol(context.Background(), model, "Demo::Nope")
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("err = %v, want ErrFailure", err)
	}
}

func TestInstantiateAnswersTheReachableInstances(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	instantiation, err := client.Instantiate(context.Background(), model, "Demo::Vehicle")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if instantiation.Root == nil {
		t.Fatal("instantiation has no root")
	}
	if len(instantiation.Instances) == 0 {
		t.Error("instantiation names no reachable instances")
	}
}

func TestReturnedValuesAreCopiesTheCallerOwns(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)
	first, err := client.LookupSymbol(context.Background(), model, "Demo::Vehicle")
	if err != nil {
		t.Fatalf("LookupSymbol: %v", err)
	}
	first.Name = "Mutated"
	first.ChildIDs = append(first.ChildIDs[:0], "gone")
	for key := range first.Metadata {
		first.Metadata[key] = "gone"
	}
	second, err := client.LookupSymbol(context.Background(), model, "Demo::Vehicle")
	if err != nil {
		t.Fatalf("LookupSymbol: %v", err)
	}
	if second.Name != "Vehicle" {
		t.Errorf("a mutation of one answer changed the next: name = %q", second.Name)
	}
	for _, child := range second.ChildIDs {
		if child == "gone" {
			t.Error("a mutation of one answer's children changed the next")
		}
	}
}
