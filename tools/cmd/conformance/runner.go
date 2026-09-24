package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Result is one scenario's outcome.
type Result struct {
	ID string `json:"id"`
	// pass, fail, skip or error: an error is the scenario itself being wrong
	// (an unknown RPC, an unreadable fixture), not a service disagreeing.
	Outcome string `json:"outcome"`
	RPC     string `json:"rpc"`
	// Why it was skipped, or what it disagreed about.
	Reason   string   `json:"reason,omitempty"`
	Failures []string `json:"failures,omitempty"`
	// The status the call answered, as a code name.
	Status              string   `json:"status"`
	WithoutCapability   bool     `json:"without_capability,omitempty"`
	MissingCapabilities []string `json:"missing_capabilities,omitempty"`
	DurationMS          float64  `json:"duration_ms"`
}

// Summary is the machine-readable report of a run.
type Summary struct {
	Protocol          string    `json:"protocol"`
	Service           string    `json:"service"`
	Capabilities      []string  `json:"capabilities"`
	Total             int       `json:"total"`
	Passed            int       `json:"passed"`
	Failed            int       `json:"failed"`
	Skipped           int       `json:"skipped"`
	Errored           int       `json:"errored"`
	WithoutCapability int       `json:"without_capability"`
	Results           []*Result `json:"results"`
}

// runner executes scenarios against one service.
type runner struct {
	service      *service
	client       client
	fixtures     string
	capabilities []string
	models       map[string]string // Model.key() → the hash the service gave its parse
	verbose      bool
	// Withheld runs validate GetServerInfo against the default capability set directly.
	omitHandshakeScenarios bool
	out                    io.Writer
	scenarioLog            io.Writer
}

// readCapabilities asks the service what it supports, which is what gates the
// scenarios that need one.
func (r *runner) readCapabilities(ctx context.Context) error {
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	response, err := r.client.call(call, "GetServerInfo", nil)
	if err != nil {
		return fmt.Errorf("GetServerInfo: %w", err)
	}
	fields := response.Descriptor().Fields()
	list := response.Get(fields.ByName("capabilities")).List()
	for i := 0; i < list.Len(); i++ {
		r.capabilities = append(r.capabilities, list.Get(i).String())
	}
	sort.Strings(r.capabilities)
	return nil
}

// runAll executes every scenario matching filter and reports as it goes.
func (r *runner) runAll(ctx context.Context, scenarios []*Scenario, filter *regexp.Regexp) *Summary {
	summary := &Summary{Protocol: r.client.protocol(), Service: r.service.binary, Capabilities: r.capabilities}
	for _, scenario := range scenarios {
		if filter != nil && !filter.MatchString(scenario.ID) {
			continue
		}
		if r.omitHandshakeScenarios && scenario.method() == "GetServerInfo" {
			continue
		}
		result := r.run(ctx, scenario)
		summary.Results = append(summary.Results, result)
		summary.Total++
		switch result.Outcome {
		case "pass":
			summary.Passed++
		case "fail":
			summary.Failed++
		case "skip":
			summary.Skipped++
		default:
			summary.Errored++
		}
		if result.WithoutCapability {
			summary.WithoutCapability++
		}
		r.report(result)
	}
	fmt.Fprintf(r.out, "\n[%s] %d scenarios: %d passed, %d failed, %d skipped, %d in error\n",
		r.client.protocol(),
		summary.Total, summary.Passed, summary.Failed, summary.Skipped, summary.Errored)
	return summary
}

// report prints one scenario's outcome.
func (r *runner) report(result *Result) {
	marks := map[string]string{"pass": "PASS", "fail": "FAIL", "skip": "SKIP", "error": "ERR "}
	fmt.Fprintf(r.scenarioLog, "%s %-46s %s\n", marks[result.Outcome], result.ID, result.Status)
	if result.Reason != "" {
		fmt.Fprintf(r.scenarioLog, "       %s\n", result.Reason)
	}
	for _, failure := range result.Failures {
		fmt.Fprintf(r.scenarioLog, "       %s\n", failure)
	}
}

// run executes one scenario: it parses the model the scenario names, makes the
// call, and compares the answer.
func (r *runner) run(ctx context.Context, scenario *Scenario) *Result {
	started := time.Now()
	result := &Result{ID: scenario.ID, RPC: scenario.method(), Outcome: "pass", Status: "OK"}
	defer func() { result.DurationMS = float64(time.Since(started).Microseconds()) / 1000 }()

	expect := &scenario.Expect
	if missing := r.missingCapabilities(scenario); len(missing) > 0 {
		result.MissingCapabilities = missing
		if scenario.ExpectWithoutCapability == nil {
			result.Outcome = "skip"
			result.Status = "-"
			result.Reason = fmt.Sprintf("the service does not report %s", strings.Join(missing, ", "))
			return result
		}
		expect = scenario.ExpectWithoutCapability
		result.WithoutCapability = true
		result.Reason = fmt.Sprintf("the service does not report %s, so the without-capability expectation applies",
			strings.Join(missing, ", "))
	}

	modelHash := ""
	if scenario.Model != nil {
		hash, err := r.modelHash(ctx, *scenario.Model)
		if err != nil {
			return errored(result, err)
		}
		modelHash = hash
	}

	request, err := r.request(scenario, modelHash)
	if err != nil {
		return errored(result, err)
	}

	call, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	response, callErr := r.client.call(call, scenario.method(), request)

	var uncovered *uncoveredError
	if errors.As(callErr, &uncovered) {
		result.Outcome = "skip"
		result.Status = "-"
		result.Reason = uncovered.reason
		return result
	}

	wantStatus := expect.Status
	if wantStatus == "" {
		wantStatus = "OK"
	}
	gotStatus := status.Code(callErr)
	result.Status = statusName(gotStatus)
	if callErr != nil {
		if !sameCode(wantStatus, gotStatus) {
			result.Outcome = "fail"
			result.Failures = []string{fmt.Sprintf("status: %s (%s), want %s",
				statusName(gotStatus), status.Convert(callErr).Message(), wantStatus)}
			return result
		}
		if expect.StatusMessageContains != "" &&
			!strings.Contains(status.Convert(callErr).Message(), expect.StatusMessageContains) {
			result.Outcome = "fail"
			result.Failures = []string{fmt.Sprintf("status message %q does not contain %q",
				status.Convert(callErr).Message(), expect.StatusMessageContains)}
		}
		return result
	}
	if wantStatus != "OK" {
		result.Outcome = "fail"
		result.Failures = []string{fmt.Sprintf("the call succeeded, want status %s", wantStatus)}
		return result
	}

	normalized := newNormalizer(modelHash).normalize(response)
	if r.verbose {
		rendered, _ := json.MarshalIndent(normalized, "       ", "  ")
		fmt.Fprintf(r.scenarioLog, "       %s\n", rendered)
	}
	if failures := check(expect, normalized); len(failures) > 0 {
		result.Outcome = "fail"
		result.Failures = failures
	}
	return result
}

// missingCapabilities are the capabilities a scenario needs that the service
// does not report.
func (r *runner) missingCapabilities(scenario *Scenario) []string {
	var missing []string
	for _, capability := range scenario.RequiresCapabilities {
		if !slices.Contains(r.capabilities, capability) {
			missing = append(missing, capability)
		}
	}
	return missing
}

// request builds the call's message from the scenario's protobuf-JSON, with the
// placeholders resolved.
func (r *runner) request(scenario *Scenario, modelHash string) (protoreflect.Message, error) {
	method, err := methodByName(scenario.method())
	if err != nil {
		return nil, err
	}
	message := dynamicpb.NewMessage(method.Input())
	if len(scenario.Request) == 0 {
		return message, nil
	}
	var tree any
	if err := json.Unmarshal(scenario.Request, &tree); err != nil {
		return nil, fmt.Errorf("request is not JSON: %w", err)
	}
	resolved, err := r.resolve(tree, modelHash)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(resolved)
	if err != nil {
		return nil, err
	}
	if err := protojson.Unmarshal(data, message.Interface()); err != nil {
		return nil, fmt.Errorf("request does not fit %s: %w", method.Input().FullName(), err)
	}
	return message, nil
}

var fixtureRef = regexp.MustCompile(`^\$\{fixture:([^}]+)\}$`)

// resolve replaces "${model_hash}" and "${fixture:<path>}" in a request.
func (r *runner) resolve(tree any, modelHash string) (any, error) {
	switch node := tree.(type) {
	case map[string]any:
		for key, value := range node {
			resolved, err := r.resolve(value, modelHash)
			if err != nil {
				return nil, err
			}
			node[key] = resolved
		}
		return node, nil
	case []any:
		for i, value := range node {
			resolved, err := r.resolve(value, modelHash)
			if err != nil {
				return nil, err
			}
			node[i] = resolved
		}
		return node, nil
	case string:
		if node == modelHashPlaceholder {
			if modelHash == "" {
				return nil, fmt.Errorf("the request names %s but the scenario declares no model", modelHashPlaceholder)
			}
			return modelHash, nil
		}
		if match := fixtureRef.FindStringSubmatch(node); match != nil {
			return r.fixture(match[1])
		}
		return node, nil
	default:
		return tree, nil
	}
}

// fixture reads a fixture's source.
func (r *runner) fixture(name string) (string, error) {
	path := filepath.Join(r.fixtures, filepath.Clean(name))
	if !strings.HasPrefix(path, r.fixtures+string(os.PathSeparator)) {
		return "", fmt.Errorf("fixture %q is outside %s", name, r.fixtures)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- the path is checked above to be inside the fixtures directory.
	if err != nil {
		return "", fmt.Errorf("reading fixture %q: %w", name, err)
	}
	return string(data), nil
}

// modelHash parses a scenario's model once per run and returns the hash the
// service gave it. A model that fails to parse is an error in the suite.
func (r *runner) modelHash(ctx context.Context, model Model) (string, error) {
	if hash, ok := r.models[model.key()]; ok {
		return hash, nil
	}
	rpc, request, err := r.parseRequest(model)
	if err != nil {
		return "", err
	}
	named := strings.Join(model.fixtureNames(), ", ")

	call, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	response, err := r.client.call(call, rpc, request)
	if err != nil {
		return "", fmt.Errorf("parsing fixture %q: %w", named, err)
	}
	out := response.Descriptor().Fields()
	if reported := response.Get(out.ByName("error")).String(); reported != "" {
		return "", fmt.Errorf("parsing fixture %q: %s", named, reported)
	}
	diagnostics := response.Get(out.ByName("diagnostics")).List()
	for i := 0; i < diagnostics.Len(); i++ {
		diagnostic := diagnostics.Get(i).Message()
		diagFields := diagnostic.Descriptor().Fields()
		if strings.EqualFold(diagnostic.Get(diagFields.ByName("severity")).String(), "error") {
			return "", fmt.Errorf("fixture %q does not parse clean: %s", named,
				diagnostic.Get(diagFields.ByName("message")).String())
		}
	}
	hash := response.Get(out.ByName("model_hash")).String()
	if hash == "" {
		return "", fmt.Errorf("parsing fixture %q returned no model hash", named)
	}
	r.models[model.key()] = hash
	return hash, nil
}

// parseRequest builds the parse a model needs: ParseFile for one fixture,
// ParseSources for several, each document named by its fixture.
func (r *runner) parseRequest(model Model) (string, protoreflect.Message, error) {
	if len(model.Fixtures) == 0 {
		source, err := r.fixture(model.Fixture)
		if err != nil {
			return "", nil, err
		}
		method, err := methodByName("ParseFile")
		if err != nil {
			return "", nil, err
		}
		request := dynamicpb.NewMessage(method.Input())
		fields := method.Input().Fields()
		request.Set(fields.ByName("content"), protoreflect.ValueOfString(source))
		if model.Language != "" {
			request.Set(fields.ByName("language"), protoreflect.ValueOfString(model.Language))
		}
		if model.StrictConformance {
			request.Set(fields.ByName("strict_conformance"), protoreflect.ValueOfBool(true))
		}
		return "ParseFile", request, nil
	}

	method, err := methodByName("ParseSources")
	if err != nil {
		return "", nil, err
	}
	request := dynamicpb.NewMessage(method.Input())
	fields := method.Input().Fields()
	documents := request.Mutable(fields.ByName("documents")).List()
	for _, name := range model.Fixtures {
		source, err := r.fixture(name)
		if err != nil {
			return "", nil, err
		}
		document := documents.NewElement().Message()
		docFields := document.Descriptor().Fields()
		document.Set(docFields.ByName("name"), protoreflect.ValueOfString(name))
		document.Set(docFields.ByName("content"), protoreflect.ValueOfString(source))
		if model.Language != "" {
			document.Set(docFields.ByName("language"), protoreflect.ValueOfString(model.Language))
		}
		documents.Append(protoreflect.ValueOfMessage(document))
	}
	if model.StrictConformance {
		request.Set(fields.ByName("strict_conformance"), protoreflect.ValueOfBool(true))
	}
	return "ParseSources", request, nil
}

// errored marks a scenario as broken rather than as a service disagreement.
func errored(result *Result, err error) *Result {
	result.Outcome = "error"
	result.Status = "-"
	result.Reason = err.Error()
	return result
}

// statusNames are the canonical gRPC status names, which is how a scenario
// spells a status: the Go spelling ("NotFound") is one language's.
var statusNames = map[codes.Code]string{
	codes.OK:                 "OK",
	codes.Canceled:           "CANCELLED",
	codes.Unknown:            "UNKNOWN",
	codes.InvalidArgument:    "INVALID_ARGUMENT",
	codes.DeadlineExceeded:   "DEADLINE_EXCEEDED",
	codes.NotFound:           "NOT_FOUND",
	codes.AlreadyExists:      "ALREADY_EXISTS",
	codes.PermissionDenied:   "PERMISSION_DENIED",
	codes.ResourceExhausted:  "RESOURCE_EXHAUSTED",
	codes.FailedPrecondition: "FAILED_PRECONDITION",
	codes.Aborted:            "ABORTED",
	codes.OutOfRange:         "OUT_OF_RANGE",
	codes.Unimplemented:      "UNIMPLEMENTED",
	codes.Internal:           "INTERNAL",
	codes.Unavailable:        "UNAVAILABLE",
	codes.DataLoss:           "DATA_LOSS",
	codes.Unauthenticated:    "UNAUTHENTICATED",
}

// statusName is a status code as a scenario spells it.
func statusName(code codes.Code) string {
	if name, ok := statusNames[code]; ok {
		return name
	}
	return code.String()
}

// sameCode reports whether a status code is the one a scenario named.
func sameCode(want string, got codes.Code) bool {
	return strings.EqualFold(want, statusName(got)) || strings.EqualFold(want, got.String())
}
