package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// objectFixture declares queries over the objects a service holds and the
// assertions on them, as the REPL's %run-query tests do.
const objectFixture = "../../doc/docrender/testdata/object_report.sysml"

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

// TestShortNameLookupSeesEveryDeclaration: a lone name is matched against every
// declaration of that name, so one instantiated among many is found and an
// ambiguity names them all, however many sort ahead of it.
func TestShortNameLookupSeesEveryDeclaration(t *testing.T) {
	var src strings.Builder
	src.WriteString("package Fleet {\n\tprivate import DocumentQueries::*;\n\tprivate import KerML::Root::Element;\n")
	src.WriteString("\tpart def Item;\n\tcalc def Self :> Query { in root : Element; Project(source = root, properties = (\"qualifiedName\")) }\n")
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&src, "\tpackage P%02d { part crate : Fleet::Item; }\n", i)
	}
	src.WriteString("\tpackage Z { part crate : Item; }\n}\n")
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, src.String())
	holdObject(t, srv, hash, "Fleet::Z::crate")

	_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Fleet::Self", Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath("crate"))},
	})
	if err == nil {
		t.Fatal("binding crate resolved, want ambiguity among 31 declarations")
	}
	for _, name := range []string{"Fleet::P01::crate", "Fleet::P30::crate", "Fleet::Z::crate"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("ambiguity %q does not name %s", err.Error(), name)
		}
	}
	resp := queryObjects(t, srv, hash, "Fleet::Self", binding("root", objectByPath("Fleet::Z::crate")))
	if got := objectLabels(resp); len(got) != 1 || got[0] != "Fleet::Z::crate (#1)" {
		t.Errorf("Self root=Fleet::Z::crate rows = %v, want [Fleet::Z::crate (#1)]", got)
	}
}

// TestHeldObjectsAreBounded: once a model holds the objects HeldObjectsEnvVar
// allows, Instantiate is refused with RESOURCE_EXHAUSTED naming the variable, and
// everything held stays bound; nothing is evicted behind a client's ids.
func TestHeldObjectsAreBounded(t *testing.T) {
	t.Setenv(HeldObjectsEnvVar, "5")
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)

	// A car is four objects (car, engine, two wheels): the first fits under the
	// bound, the second would pass it at its engine and is refused whole.
	exhausted := func(sym string, held int) {
		t.Helper()
		_, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{ModelHash: hash, SymbolId: sym})
		if err == nil {
			t.Fatalf("Instantiate %s succeeded, want RESOURCE_EXHAUSTED", sym)
		}
		if connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Errorf("code = %v, want %v: %v", connect.CodeOf(err), connect.CodeResourceExhausted, err)
		}
		for _, text := range []string{fmt.Sprintf("holds %d objects", held), HeldObjectsEnvVar, "the 5 that"} {
			if !strings.Contains(err.Error(), text) {
				t.Errorf("error %q lacks %q", err.Error(), text)
			}
		}
	}
	holdObject(t, srv, hash, "Garage::car")
	exhausted("Garage::car", 4)
	cached, _ := srv.cache.Get(hash)
	if got := srv.objects(cached).rt.InstanceCount(); got != 4 {
		t.Errorf("held after the refused car = %d, want 4", got)
	}

	// The population is as it was before the refused car, root and parts alike.
	resp := queryObjects(t, srv, hash, "Garage::Parts", binding("root", objectByID(1)))
	if got := objectLabels(resp); len(got) != 3 || got[0] != "#1.engine (#2)" {
		t.Errorf("Parts root=#1 rows = %v, want the car's three parts", got)
	}
	resp = queryObjects(t, srv, hash, "Garage::Wheels")
	want := []string{"Garage::car.wheels[1] (#3)", "Garage::car.wheels[2] (#4)"}
	if got := objectLabels(resp); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Errorf("Wheels rows = %v, want %v", got, want)
	}
	_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Garage::Parts", Bindings: []*pb.DocumentQueryBinding{binding("root", objectByID(5))},
	})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("binding the refused car's id: code = %v, want NOT_FOUND: %v", connect.CodeOf(err), err)
	}

	// One object fills the bound exactly; the next is one too many.
	holdObject(t, srv, hash, "Garage::spare")
	exhausted("Garage::spare", 5)

	// Another model is a population of its own under the same bound.
	other := mustParse(t, srv, "package Lot { part def Cone; part cone : Cone; }")
	holdObject(t, srv, other, "Lot::cone")
}

// TestMaxHeldObjectsFromEnv: the bound is the positive integer the variable
// holds, the default when it is unset, and anything else is refused at
// construction naming the variable.
func TestMaxHeldObjectsFromEnv(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{raw: "", want: DefaultMaxHeldObjects},
		{raw: "   ", want: DefaultMaxHeldObjects},
		{raw: "1", want: 1},
		{raw: " 250 ", want: 250},
		{raw: "0", wantErr: true},
		{raw: "-1", wantErr: true},
		{raw: "many", wantErr: true},
		{raw: "1.5", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.raw), func(t *testing.T) {
			t.Setenv(HeldObjectsEnvVar, tc.raw)
			got, err := maxHeldObjectsFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("%q was accepted as %d", tc.raw, got)
				}
				if !strings.Contains(err.Error(), HeldObjectsEnvVar) {
					t.Errorf("error does not name %s: %v", HeldObjectsEnvVar, err)
				}
				if _, serr := NewService(4, "test"); serr == nil {
					t.Error("NewService accepted an unusable held objects bound")
				}
				return
			}
			if err != nil {
				t.Fatalf("maxHeldObjectsFromEnv(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("bound %d, want %d", got, tc.want)
			}
		})
	}
}
