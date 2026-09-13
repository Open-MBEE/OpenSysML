package grpc

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
)

// telescopeFixture is the document pipeline's own telescope-domain fixture, so
// the service answers exactly what the renderer's goldens lock in.
const telescopeFixture = "../core/docrender/testdata/telescope_report.sysml"

// telescopeGolden is the Markdown the fixture's MassReport renders to.
const telescopeGolden = "../core/docrender/testdata/telescope_report.golden.md"

// defaultedFixture declares queries whose parameters carry defaults.
const defaultedFixture = "../core/docrender/testdata/defaulted_queries.sysml"

// quantityFixture declares quantity-valued attributes and queries over them.
const quantityFixture = "../core/docrender/testdata/quantity_report.sysml"

// parseTelescope loads the telescope fixture into a fresh service.
func parseTelescope(t *testing.T, srv *Service) string {
	t.Helper()
	return parseFixture(t, srv, telescopeFixture)
}

// parseFixture loads a fixture file into a fresh service.
func parseFixture(t *testing.T, srv *Service, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: string(content)},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	return parsed.ModelHash
}

// element binds a model element by qualified name.
func element(fqn string) *pb.DocumentValue {
	return &pb.DocumentValue{Kind: &pb.DocumentValue_ElementId{ElementId: fqn}}
}

// binding binds one parameter to values.
func binding(parameter string, values ...*pb.DocumentValue) *pb.DocumentQueryBinding {
	return &pb.DocumentQueryBinding{Parameter: parameter, Values: values}
}

// protoMagnitude reads a quantity's magnitude whichever arm carries it.
func protoMagnitude(q *pb.Quantity) float64 {
	if realMag, ok := q.GetMagnitude().(*pb.Quantity_RealMagnitude); ok {
		return realMag.RealMagnitude
	}
	return float64(q.GetIntMagnitude())
}

// stringCell reads a cell expected to hold one string value.
func stringCell(t *testing.T, cell *pb.DocumentQueryCell) string {
	t.Helper()
	if len(cell.Values) != 1 {
		t.Fatalf("cell holds %d values, want 1", len(cell.Values))
	}
	return cell.Values[0].GetStringValue()
}

func TestRunDocumentQueryAnswersTypedRows(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash,
		QueryId:   "Observatory::SubsystemTable",
		Bindings:  []*pb.DocumentQueryBinding{binding("root", element("Observatory::telescope"))},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}

	columns := make([]string, 0, len(resp.Columns))
	for _, column := range resp.Columns {
		columns = append(columns, column.Name)
	}
	if len(columns) != 2 || columns[0] != "name" || columns[1] != "mass" {
		t.Fatalf("columns = %v, want [name mass]", columns)
	}

	wantNames := []string{"baffle|shroud *tricky*", "mount", "optics", "segmentControl"}
	wantMasses := []float64{1.5, 15, 8.5, 20}
	wantElements := []string{
		"Observatory::telescope::baffle|shroud *tricky*",
		"Observatory::telescope::mount",
		"Observatory::telescope::optics",
		"Observatory::telescope::segmentControl",
	}
	if len(resp.Rows) != len(wantNames) {
		t.Fatalf("rows = %d, want %d", len(resp.Rows), len(wantNames))
	}
	for i, row := range resp.Rows {
		if got := row.Element.GetElementId(); got != wantElements[i] {
			t.Errorf("row %d element = %q, want %q", i, got, wantElements[i])
		}
		if got := row.Element.GetElementType(); got != "PartUsage" {
			t.Errorf("row %d element type = %q, want PartUsage", i, got)
		}
		if len(row.Cells) != 2 {
			t.Fatalf("row %d holds %d cells, want 2", i, len(row.Cells))
		}
		if got := stringCell(t, row.Cells[0]); got != wantNames[i] {
			t.Errorf("row %d name = %q, want %q", i, got, wantNames[i])
		}
		massValues := row.Cells[1].Values
		if len(massValues) != 1 || massValues[0].GetRealValue() != wantMasses[i] {
			t.Errorf("row %d mass = %v, want %v", i, massValues, wantMasses[i])
		}
	}
}

// TestRunDocumentQueryAnswersQuantities: a quantity-valued attribute is answered
// as a quantity value keeping its magnitude and the unit the model spelt.
func TestRunDocumentQueryAnswersQuantities(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, quantityFixture)

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash,
		QueryId:   "Launcher::Masses",
		Bindings:  []*pb.DocumentQueryBinding{binding("root", element("Launcher::rocket"))},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	want := []struct {
		name      string
		magnitude int64
		unit      string
		tonnes    float64
	}{
		{"s1", 2290000, "kg", 2290},
		{"s2", 119000, "kg", 119},
		{"probe", 500000, "g", 500},
	}
	if len(resp.Rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(resp.Rows), len(want))
	}
	for i, row := range resp.Rows {
		if got := stringCell(t, row.Cells[0]); got != want[i].name {
			t.Errorf("row %d name = %q, want %q", i, got, want[i].name)
		}
		mass := row.Cells[1].Values
		if len(mass) != 1 || mass[0].GetQuantity() == nil {
			t.Fatalf("row %d mass = %v, want one quantity", i, mass)
		}
		got := mass[0].GetQuantity()
		if got.GetIntMagnitude() != want[i].magnitude || got.GetUnit() != want[i].unit {
			t.Errorf("row %d mass = %v, want %d [%s]", i, got, want[i].magnitude, want[i].unit)
		}
		if len(got.GetUnitTerm().GetFactors()) == 0 {
			t.Errorf("row %d mass carries no unit term: %v", i, got)
		}
		tonnes := row.Cells[2].Values
		if len(tonnes) != 1 || tonnes[0].GetQuantity() == nil {
			t.Fatalf("row %d tonnes = %v, want one quantity", i, tonnes)
		}
		if got := tonnes[0].GetQuantity(); got.GetRealMagnitude() != want[i].tonnes || got.GetUnit() != want[i].unit {
			t.Errorf("row %d tonnes = %v, want %v [%s]", i, got, want[i].tonnes, want[i].unit)
		}
	}
}

// TestRunDocumentQueryBindsQuantities: a quantity a query answered binds back
// into a parameter of its dimension; bound to a String parameter it is refused.
func TestRunDocumentQueryBindsQuantities(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, quantityFixture)
	root := binding("root", element("Launcher::rocket"))

	answered, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Launcher::Masses", Bindings: []*pb.DocumentQueryBinding{root},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery Masses failed: %v", err)
	}
	budget := answered.Rows[0].Cells[1].Values[0]
	if budget.GetQuantity() == nil {
		t.Fatalf("answered mass = %v, want a quantity", budget)
	}

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Launcher::Margin",
		Bindings: []*pb.DocumentQueryBinding{root, binding("budget", budget)},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery Margin failed: %v", err)
	}
	want := []struct {
		name   string
		margin float64
		unit   string
	}{{"s1", 0, "kg"}, {"s2", 2171000, "kg"}, {"probe", 2289500, "kg"}}
	if len(resp.Rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(resp.Rows), len(want))
	}
	for i, row := range resp.Rows {
		if got := stringCell(t, row.Cells[0]); got != want[i].name {
			t.Errorf("row %d name = %q, want %q", i, got, want[i].name)
		}
		margin := row.Cells[1].Values
		if len(margin) != 1 || margin[0].GetQuantity() == nil {
			t.Fatalf("row %d margin = %v, want one quantity", i, margin)
		}
		if got := margin[0].GetQuantity(); protoMagnitude(got) != want[i].margin || got.GetUnit() != want[i].unit {
			t.Errorf("row %d margin = %v, want %v [%s]", i, got, want[i].margin, want[i].unit)
		}
	}

	_, err = srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Launcher::Named",
		Bindings: []*pb.DocumentQueryBinding{root, binding("label", budget)},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("quantity bound to a String parameter: err = %v, want %s", err, connect.CodeInvalidArgument)
	}
}

func TestRunDocumentQueryBindsScalars(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash,
		QueryId:   "Observatory::HeavySubsystemNames",
		Bindings: []*pb.DocumentQueryBinding{
			binding("root", element("Observatory::telescope")),
			binding("threshold", &pb.DocumentValue{Kind: &pb.DocumentValue_StringValue{StringValue: "10"}}),
		},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	names := make([]string, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		names = append(names, stringCell(t, row.Cells[0]))
	}
	if len(names) != 2 || names[0] != "mount" || names[1] != "segmentControl" {
		t.Fatalf("names = %v, want [mount segmentControl]", names)
	}
}

func TestRunDocumentQueryUsesParameterDefaults(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, defaultedFixture)

	namesOf := func(t *testing.T, query string, bindings ...*pb.DocumentQueryBinding) []string {
		t.Helper()
		resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: query, Bindings: bindings,
		})
		if err != nil {
			t.Fatalf("RunDocumentQuery %s failed: %v", query, err)
		}
		names := make([]string, 0, len(resp.Rows))
		for _, row := range resp.Rows {
			names = append(names, stringCell(t, row.Cells[0]))
		}
		return names
	}

	if got := namesOf(t, "Observatory::HeavySubsystems"); !slices.Equal(got, []string{"mount", "segmentControl"}) {
		t.Errorf("defaulted names = %v, want [mount segmentControl]", got)
	}
	if got := namesOf(t, "Observatory::LightSubsystems"); !slices.Equal(got, []string{"mount", "optics", "segmentControl"}) {
		t.Errorf("redefined default names = %v, want [mount optics segmentControl]", got)
	}
	got := namesOf(t, "Observatory::HeavySubsystems",
		binding("root", element("Observatory::instruments")),
		binding("threshold", &pb.DocumentValue{Kind: &pb.DocumentValue_StringValue{StringValue: "1"}}))
	if !slices.Equal(got, []string{"spectrograph"}) {
		t.Errorf("explicitly bound names = %v, want [spectrograph]", got)
	}
}

func TestRunDocumentQueryAnswersEmptyRows(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash,
		QueryId:   "Observatory::MissingSubsystems",
		Bindings:  []*pb.DocumentQueryBinding{binding("root", element("Observatory::telescope"))},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	if len(resp.Columns) != 2 {
		t.Errorf("columns = %d, want 2: an empty answer still says what it projects", len(resp.Columns))
	}
	if len(resp.Rows) != 0 {
		t.Errorf("rows = %d, want 0", len(resp.Rows))
	}
}

func TestRunDocumentQueryIsDeterministic(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)

	request := &pb.RunDocumentQueryRequest{
		ModelHash: hash,
		QueryId:   "Observatory::SubsystemTable",
		Bindings:  []*pb.DocumentQueryBinding{binding("root", element("Observatory::telescope"))},
	}
	first, err := srv.RunDocumentQuery(context.Background(), request)
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	second, err := srv.RunDocumentQuery(context.Background(), request)
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	if first.String() != second.String() {
		t.Errorf("two runs answered differently:\n%s\n%s", first, second)
	}
}

func TestRunDocumentQueryFailures(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)
	root := binding("root", element("Observatory::telescope"))

	cases := []struct {
		name    string
		request *pb.RunDocumentQueryRequest
		code    connect.Code
	}{
		{"unknown model", &pb.RunDocumentQueryRequest{
			ModelHash: "deadbeef", QueryId: "Observatory::SubsystemTable",
		}, connect.CodeNotFound},
		{"unnamed query", &pb.RunDocumentQueryRequest{
			ModelHash: hash,
		}, connect.CodeInvalidArgument},
		{"unknown query", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::NoSuchQuery",
		}, connect.CodeNotFound},
		{"not a query", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::Subsystem",
		}, connect.CodeInvalidArgument},
		{"unknown binding", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::SubsystemTable",
			Bindings: []*pb.DocumentQueryBinding{root, binding("depth",
				&pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: 3}})},
		}, connect.CodeInvalidArgument},
		{"missing binding", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::SubsystemTable",
		}, connect.CodeInvalidArgument},
		{"binding type", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::HeavySubsystemNames",
			Bindings: []*pb.DocumentQueryBinding{root, binding("threshold",
				&pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: 10}})},
		}, connect.CodeInvalidArgument},
		{"unknown bound element", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::SubsystemTable",
			Bindings: []*pb.DocumentQueryBinding{binding("root", element("Observatory::nothing"))},
		}, connect.CodeInvalidArgument},
		{"unnamed binding", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::SubsystemTable",
			Bindings: []*pb.DocumentQueryBinding{binding("",
				&pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: 1}})},
		}, connect.CodeInvalidArgument},
		{"bound infinity", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::SubsystemTable",
			Bindings: []*pb.DocumentQueryBinding{binding("root",
				&pb.DocumentValue{Kind: &pb.DocumentValue_Infinity{Infinity: true}})},
		}, connect.CodeInvalidArgument},
		{"valueless binding", &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Observatory::SubsystemTable",
			Bindings: []*pb.DocumentQueryBinding{binding("root", &pb.DocumentValue{})},
		}, connect.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.RunDocumentQuery(context.Background(), tc.request)
			if connect.CodeOf(err) != tc.code {
				t.Errorf("code = %v (%v), want %v", connect.CodeOf(err), err, tc.code)
			}
		})
	}
}

func TestRunDocumentQueryRequiresCapability(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityDocumentQuery)
	_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: "any", QueryId: "Any",
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("code = %v, want Unimplemented", connect.CodeOf(err))
	}
}

func TestDocumentStatusMapsBudgetExhaustion(t *testing.T) {
	cases := []struct {
		kind queryexec.ErrorKind
		code connect.Code
	}{
		{queryexec.ErrorVisitBudget, connect.CodeResourceExhausted},
		{queryexec.ErrorInvocationBudget, connect.CodeResourceExhausted},
		{queryexec.ErrorInvocationDepth, connect.CodeResourceExhausted},
		{queryexec.ErrorUnsupportedOperation, connect.CodeFailedPrecondition},
		{queryexec.ErrorUnknownProperty, connect.CodeFailedPrecondition},
		{queryexec.ErrorUnknownBinding, connect.CodeInvalidArgument},
	}
	for _, tc := range cases {
		err := documentStatus(&queryexec.Error{Kind: tc.kind, Query: "Q"})
		if connect.CodeOf(err) != tc.code {
			t.Errorf("%s: code = %v, want %v", tc.kind, connect.CodeOf(err), tc.code)
		}
	}
}

func TestRenderDocumentMatchesTheRendererGolden(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)

	resp, err := srv.RenderDocument(context.Background(), &pb.RenderDocumentRequest{
		ModelHash: hash, DocumentId: "Observatory::MassReport",
	})
	if err != nil {
		t.Fatalf("RenderDocument failed: %v", err)
	}
	golden, err := os.ReadFile(telescopeGolden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if resp.Markdown != string(golden) {
		t.Errorf("markdown differs from the renderer's golden:\n%s", resp.Markdown)
	}
}

func TestRenderDocumentUsesParameterDefaults(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, defaultedFixture)

	resp, err := srv.RenderDocument(context.Background(), &pb.RenderDocumentRequest{
		ModelHash: hash, DocumentId: "Observatory::DefaultedReport",
	})
	if err != nil {
		t.Fatalf("RenderDocument failed: %v", err)
	}
	for _, want := range []string{
		"| mount | 15 |\n| segmentControl | 20 |",
		"- mount 15\n- optics 8.5\n- segmentControl 20",
		"- spectrograph 4",
	} {
		if !strings.Contains(resp.Markdown, want) {
			t.Errorf("markdown lacks %q:\n%s", want, resp.Markdown)
		}
	}
}

func TestRenderDocumentFailures(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)

	cases := []struct {
		name    string
		request *pb.RenderDocumentRequest
		code    connect.Code
	}{
		{"unknown model", &pb.RenderDocumentRequest{
			ModelHash: "deadbeef", DocumentId: "Observatory::MassReport",
		}, connect.CodeNotFound},
		{"unnamed document", &pb.RenderDocumentRequest{
			ModelHash: hash,
		}, connect.CodeInvalidArgument},
		{"unknown document", &pb.RenderDocumentRequest{
			ModelHash: hash, DocumentId: "Observatory::NoSuchReport",
		}, connect.CodeNotFound},
		{"not a document", &pb.RenderDocumentRequest{
			ModelHash: hash, DocumentId: "Observatory::SubsystemTable",
		}, connect.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.RenderDocument(context.Background(), tc.request)
			if connect.CodeOf(err) != tc.code {
				t.Errorf("code = %v (%v), want %v", connect.CodeOf(err), err, tc.code)
			}
		})
	}
}

func TestRenderDocumentRequiresCapability(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityRenderDocument)
	_, err := srv.RenderDocument(context.Background(), &pb.RenderDocumentRequest{
		ModelHash: "any", DocumentId: "Any",
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("code = %v, want Unimplemented", connect.CodeOf(err))
	}
}
