package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// objectFixture declares queries over the objects a service holds and the
// assertions on them, as the REPL's %run-query tests do.
const objectFixture = "../core/docrender/testdata/object_report.sysml"

// objectByID binds an object by the id Instantiate answered.
func objectByID(id int64) *pb.DocumentValue {
	return &pb.DocumentValue{Kind: &pb.DocumentValue_Object{Object: &pb.DocumentObject{InstanceId: id}}}
}

// objectByPath binds an object by the label a session reaches it under.
func objectByPath(path string) *pb.DocumentValue {
	return &pb.DocumentValue{Kind: &pb.DocumentValue_Object{Object: &pb.DocumentObject{Path: path}}}
}

// holdObject creates an object of sym and returns its root instance.
func holdObject(t *testing.T, srv *Service, hash, sym string) *pb.Instance {
	t.Helper()
	resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{ModelHash: hash, SymbolId: sym})
	if err != nil {
		t.Fatalf("Instantiate %s: %v", sym, err)
	}
	if resp.Error != "" {
		t.Fatalf("Instantiate %s: %s", sym, resp.Error)
	}
	return resp.Instance
}

// queryObjects runs a query with bindings and fails the test on a status error.
func queryObjects(t *testing.T, srv *Service, hash, query string, bindings ...*pb.DocumentQueryBinding) *pb.RunDocumentQueryResponse {
	t.Helper()
	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: query, Bindings: bindings,
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery %s: %v", query, err)
	}
	return resp
}

// objectLabels spells the rows of a response as "path (#id)".
func objectLabels(resp *pb.RunDocumentQueryResponse) []string {
	labels := make([]string, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		obj := row.Element.GetObject()
		labels = append(labels, fmt.Sprintf("%s (#%d)", obj.GetPath(), obj.GetInstanceId()))
	}
	return labels
}

// TestInstantiateHoldsObjectsForQueries: an object Instantiate created is bound
// by id, by name and by path, and answered as an object with its path and usage.
func TestInstantiateHoldsObjectsForQueries(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)

	car := holdObject(t, srv, hash, "Garage::car")
	if car.Id != 1 {
		t.Fatalf("car id = %d, want 1", car.Id)
	}

	for _, root := range []*pb.DocumentValue{objectByID(1), objectByPath("Garage::car"), objectByPath("car"), objectByPath("#1")} {
		resp := queryObjects(t, srv, hash, "Garage::Parts", binding("root", root))
		want := []string{"Garage::car.engine (#2)", "Garage::car.wheels[1] (#3)", "Garage::car.wheels[2] (#4)"}
		if root.GetObject().GetPath() == "#1" || root.GetObject().GetInstanceId() == 1 {
			want = []string{"#1.engine (#2)", "#1.wheels[1] (#3)", "#1.wheels[2] (#4)"}
		}
		if got := objectLabels(resp); strings.Join(got, ";") != strings.Join(want, ";") {
			t.Errorf("Parts root=%v rows = %v, want %v", root.GetObject(), got, want)
		}
		if len(resp.Rows) == 3 {
			wheel := resp.Rows[1].Element.GetObject()
			if wheel.GetElement().GetElementId() != "Garage::Car::wheels" || wheel.GetElement().GetElementType() != "PartUsage" {
				t.Errorf("wheel usage = %v, want Garage::Car::wheels PartUsage", wheel.GetElement())
			}
			if got := resp.Rows[1].Cells[2].Values; len(got) != 1 || got[0].GetIntValue() != 30 {
				t.Errorf("wheel pressure = %v, want 30", got)
			}
		}
	}

	// A path into the graph binds the nested object, which has no parts of its own.
	resp := queryObjects(t, srv, hash, "Garage::Parts", binding("root", objectByPath("car.wheels[2]")))
	if len(resp.Rows) != 0 {
		t.Errorf("Parts root=car.wheels[2] rows = %v, want none", objectLabels(resp))
	}
	resp = queryObjects(t, srv, hash, "Garage::Drive", binding("root", objectByPath("Garage::car")))
	if got := objectLabels(resp); len(got) != 1 || got[0] != "Garage::car (#1)" {
		t.Fatalf("Drive rows = %v, want [Garage::car (#1)]", got)
	}
	wheels := resp.Rows[0].Cells[2].Values
	if len(wheels) != 2 || wheels[0].GetObject().GetPath() != "Garage::car.wheels[1]" || wheels[1].GetObject().GetInstanceId() != 4 {
		t.Errorf("Drive wheels cell = %v, want the two wheel objects", wheels)
	}
}

// TestObjectsEnumeratesHeldObjects: Objects(type = T) answers over what the model
// holds, in the order the REPL reports, and a later Instantiate joins it.
func TestObjectsEnumeratesHeldObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)

	resp := queryObjects(t, srv, hash, "Garage::Wheels")
	if len(resp.Rows) != 0 {
		t.Fatalf("Wheels over no objects rows = %v, want none", objectLabels(resp))
	}
	holdObject(t, srv, hash, "Garage::car")
	holdObject(t, srv, hash, "Garage::spare")
	resp = queryObjects(t, srv, hash, "Garage::Wheels")
	want := []string{"Garage::spare (#5)", "Garage::car.wheels[1] (#3)", "Garage::car.wheels[2] (#4)"}
	if got := objectLabels(resp); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Errorf("Wheels rows = %v, want %v", got, want)
	}
	if got := resp.Rows[0].Cells[1].Values; len(got) != 1 || got[0].GetIntValue() != 20 {
		t.Errorf("spare pressure = %v, want 20", got)
	}
}

// TestRenderDocumentReadsHeldObjects: a document renders over the objects the
// model holds — none before Instantiate, the ones created after — as
// -render-document renders beside -instantiate.
func TestRenderDocumentReadsHeldObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)
	render := func() string {
		resp, err := srv.RenderDocument(context.Background(), &pb.RenderDocumentRequest{ModelHash: hash, DocumentId: "Garage::CarReport"})
		if err != nil {
			t.Fatalf("RenderDocument: %v", err)
		}
		return resp.Markdown
	}

	before := render()
	if strings.Contains(before, "car.wheels") || strings.Contains(before, "- spare 20") {
		t.Fatalf("report before Instantiate names objects:\n%s", before)
	}
	holdObject(t, srv, hash, "Garage::car")
	holdObject(t, srv, hash, "Garage::spare")
	after := render()
	for _, want := range []string{"| wheels\\[2\\] | Garage::car.wheels\\[2\\] | 30 |", "- Garage::spare 20", "- Garage::car.wheels\\[1\\] 30"} {
		if !strings.Contains(after, want) {
			t.Errorf("report after Instantiate lacks %q:\n%s", want, after)
		}
	}
}

// TestInstantiateAgainKeepsTheEarlierObject: the name denotes the newest
// object; the earlier one stays held, reached by id.
func TestInstantiateAgainKeepsTheEarlierObject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)

	first := holdObject(t, srv, hash, "Garage::spare")
	second := holdObject(t, srv, hash, "Garage::spare")
	if first.Id == second.Id {
		t.Fatalf("both objects have id %d", first.Id)
	}
	resp := queryObjects(t, srv, hash, "Garage::Wheels")
	want := []string{fmt.Sprintf("Garage::spare (#%d)", second.Id), fmt.Sprintf("#%d (#%d)", first.Id, first.Id)}
	if got := objectLabels(resp); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Errorf("Wheels rows = %v, want %v", got, want)
	}
	_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Garage::Parts",
		Bindings: []*pb.DocumentQueryBinding{binding("root", &pb.DocumentValue{Kind: &pb.DocumentValue_Object{
			Object: &pb.DocumentObject{Path: "spare", InstanceId: first.Id}}})},
	})
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("Garage::spare is object #%d, not #%d", second.Id, first.Id)) {
		t.Errorf("binding spare with the displaced id: %v, want the name to denote #%d", err, second.Id)
	}
}

// TestObjectBindingsRunConcurrently: requests over one model's objects serialize
// on the population without racing or deadlocking.
func TestObjectBindingsRunConcurrently(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)
	holdObject(t, srv, hash, "Garage::car")

	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
				ModelHash: hash, QueryId: "Garage::Parts",
				Bindings: []*pb.DocumentQueryBinding{binding("root", objectByID(1))},
			})
			if err == nil && len(resp.Rows) != 3 {
				err = fmt.Errorf("Parts rows = %d, want 3", len(resp.Rows))
			}
			if err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{ModelHash: hash, SymbolId: "Garage::spare"})
			if err == nil && resp.Error != "" {
				err = fmt.Errorf("Instantiate: %s", resp.Error)
			}
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	resp := queryObjects(t, srv, hash, "Garage::Wheels")
	if len(resp.Rows) != 18 {
		t.Errorf("Wheels rows = %d, want 2 of the car and 16 spares", len(resp.Rows))
	}
}

// TestVerdictsOverHeldObject: Verdicts bound to a held object answers one row per
// assertion on it and what it holds, each path from the object as bound.
func TestVerdictsOverHeldObject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, verdictFixture)
	holdObject(t, srv, hash, "Garage::car")

	for _, root := range []*pb.DocumentValue{objectByPath("car"), objectByID(1)} {
		resp := queryObjects(t, srv, hash, "Garage::Checks", binding("root", root))
		prefix := "Garage::car"
		if root.GetObject().GetInstanceId() == 1 {
			prefix = "#1"
		}
		got := make(map[string]string)
		for i, row := range resp.Rows {
			verdict := row.Element.GetVerdict()
			if verdict == nil {
				t.Fatalf("row %d element = %v, want a verdict", i, row.Element)
			}
			got[verdict.Text+" on "+verdict.Path] = verdict.Verdict
			if cell := stringCell(t, row.Cells[0]); cell != verdict.Path {
				t.Errorf("row %d path cell = %q, verdict path = %q", i, cell, verdict.Path)
			}
		}
		want := map[string]string{
			"assert constraint massOk on " + prefix:                    "holds",
			"assert constraint fits on " + prefix:                      "undecided",
			"assert constraint powerLow on " + prefix + ".engine":      "violated",
			"assert constraint pressureOk on " + prefix + ".wheels[1]": "violated",
			"assert constraint pressureOk on " + prefix + ".wheels[2]": "violated",
		}
		for text, verdict := range want {
			if got[text] != verdict {
				t.Errorf("root=%v: %s = %q, want %q (rows: %v)", root.GetObject(), text, got[text], verdict, got)
			}
		}
	}
	failing := queryObjects(t, srv, hash, "Garage::Failing", binding("root", objectByPath("car.wheels[2]")))
	if len(failing.Rows) != 1 || failing.Rows[0].Element.GetVerdict().GetPath() != "Garage::car.wheels[2]" {
		t.Errorf("Failing root=car.wheels[2] rows = %v, want the wheel's violated pressureOk", failing.Rows)
	}
}
