package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const importModelSource = `package V {
	private import ScalarValues::*;
	part def Vehicle {
		attribute count : Integer default 1;
		attribute label : String default "plain";
	}
	part vehicle : Vehicle;
	part car : Vehicle {
		attribute :>> count = 2;
	}
}`

func importData(t *testing.T, srv *Service, req *pb.ImportDataRequest) *pb.ImportDataResponse {
	t.Helper()
	resp, err := srv.ImportData(context.Background(), req)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	return resp
}

func TestImportDataSetsDeclaredAndInheritedValues(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, importModelSource)
	resp := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{
		{Name: "data.csv", Content: "element,count,label\nV::vehicle,7,fast\nV::car,9,\n"},
	}})
	if resp.Error != "" {
		t.Fatalf("ImportData refused: %s", resp.Error)
	}
	if len(resp.Files) != 1 || resp.Files[0].Values != 3 || resp.Files[0].Elements != 2 {
		t.Fatalf("files = %v; want 3 values into 2 elements", resp.Files)
	}
	var descriptions []string
	for _, a := range resp.Files[0].Assignments {
		descriptions = append(descriptions, a.Description)
	}
	want := []string{"V::vehicle::count = 7 (redefines V::Vehicle::count)", "V::vehicle::label = \"fast\" (redefines V::Vehicle::label)", "V::car::count = 9"}
	if strings.Join(descriptions, "\n") != strings.Join(want, "\n") {
		t.Errorf("assignments:\n%s\nwant:\n%s", strings.Join(descriptions, "\n"), strings.Join(want, "\n"))
	}
	if first := resp.Files[0].Assignments[0]; first.Element != "V::vehicle" || first.Feature != "count" || first.Value != "7" || first.Redefines != "V::Vehicle::count" || first.Where == "" {
		t.Errorf("first assignment = %v; want V::vehicle count 7 redefining V::Vehicle::count with where it came from", first)
	}
	if resp.ModelHash == "" || resp.ModelHash == hash || resp.Content == "" {
		t.Fatalf("model hash %q, content %q; want the imported model", resp.ModelHash, resp.Content)
	}
	if !strings.Contains(resp.Content, ":>> count = 7") || !strings.Contains(resp.Content, "attribute :>> count = 9;") {
		t.Errorf("content:\n%s\nwant vehicle's count redefined to 7 and car's set to 9", resp.Content)
	}
	if len(resp.Documents) != 1 || resp.Documents[0].Content != resp.Content {
		t.Errorf("documents = %v; want the one edited document", resp.Documents)
	}
	if _, ok := srv.cache.Get(resp.ModelHash); !ok {
		t.Errorf("the imported model %s is not cached", resp.ModelHash)
	}
}

func TestImportDataDryRunChangesNothing(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, importModelSource)
	resp := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, ImportDryRun: true, Imports: []*pb.DataImport{
		{Name: "data.txt", ImportFormat: "csv", Content: "element,count\nV::car,9\n"},
	}})
	if resp.Error != "" || len(resp.Files) != 1 || resp.Files[0].Values != 1 {
		t.Fatalf("dry run = %v; want one value reported", resp)
	}
	if resp.ModelHash != "" || resp.Content != "" || len(resp.Documents) != 0 {
		t.Errorf("dry run answered hash %q, content %q, documents %v; want none", resp.ModelHash, resp.Content, resp.Documents)
	}
}

func TestImportDataIsAtomic(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, importModelSource)
	for _, tc := range []struct{ content, want string }{
		{"element,count,label\nV::car,seven,x\n", "seven is not an integer"},
		{"element,count,wheels\nV::car,7,4\n", "V::car has no feature named wheels"},
		{"element,count\nV::car,7\nV::car,8\n", "V::car::count is already set"},
		{"element,count\nScalarValues::Integer,7\n", "is a library element"},
	} {
		resp := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{{Name: "bad.csv", Content: tc.content}}})
		if !strings.Contains(resp.Error, tc.want) {
			t.Errorf("import of %q: error %q; want %q", tc.content, resp.Error, tc.want)
		}
		if resp.ModelHash != "" || resp.Content != "" || len(resp.Documents) != 0 {
			t.Errorf("import of %q answered a model after refusing: %v", tc.content, resp)
		}
	}
}

func TestGRPCRobustnessImportData(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, importModelSource)
	cases := map[string]struct {
		req  *pb.ImportDataRequest
		code connect.Code
	}{
		"no imports":   {&pb.ImportDataRequest{ModelHash: hash}, connect.CodeInvalidArgument},
		"unnamed file": {&pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{{Content: "element,count\n"}}}, connect.CodeInvalidArgument},
		"import_as":    {&pb.ImportDataRequest{ModelHash: hash, ImportAs: "rows", Imports: []*pb.DataImport{{Name: "d.csv", Content: "element,count\n"}}}, connect.CodeInvalidArgument},
		"no model":     {&pb.ImportDataRequest{Imports: []*pb.DataImport{{Name: "d.csv", Content: "element,count\n"}}}, connect.CodeInvalidArgument},
		"stale model":  {&pb.ImportDataRequest{ModelHash: strings.Repeat("0", 64), Imports: []*pb.DataImport{{Name: "d.csv", Content: "element,count\n"}}}, connect.CodeNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := srv.ImportData(context.Background(), tc.req)
			if connect.CodeOf(err) != tc.code {
				t.Fatalf("error %v; want code %v", err, tc.code)
			}
		})
	}
	t.Run("unreadable data answers an error", func(t *testing.T) {
		for _, in := range []*pb.DataImport{
			{Name: "data.bin", Content: "element,count\nV::car,1\n"},
			{Name: "data.csv", ImportFormat: "xml", Content: "x"},
			{Name: "data.json", Content: "{"},
			{Name: "data.csv", Content: "element,count\nV::car,1\n", ImportMap: "{"},
		} {
			resp := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{in}})
			if resp.Error == "" || resp.ModelHash != "" {
				t.Errorf("import of %v: %v; want a refusal", in, resp)
			}
		}
	})
	t.Run("withheld capability", func(t *testing.T) {
		without := mustNewServiceWithout(t, CapabilityImportData)
		_, err := without.ImportData(context.Background(), &pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{{Name: "d.csv", Content: "element,count\n"}}})
		if connect.CodeOf(err) != connect.CodeUnimplemented {
			t.Fatalf("error %v; want unimplemented", err)
		}
	})
}

func TestImportDataStagesNothingInTheCache(t *testing.T) {
	srv := mustNewService(t, 1)
	hash := mustParse(t, srv, importModelSource)
	resp := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, ImportDryRun: true, Imports: []*pb.DataImport{
		{Name: "first.csv", Content: "element,count\nV::car,9\n"},
		{Name: "second.csv", Content: "element,count\nV::vehicle,7\n"},
	}})
	if resp.Error != "" || len(resp.Files) != 2 || resp.ModelHash != "" {
		t.Fatalf("dry run = %v; want two files previewed and no hash", resp)
	}
	if _, ok := srv.cache.Get(hash); !ok {
		t.Fatalf("the dry run evicted the model it previewed on from a cache of one")
	}
	applied := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{
		{Name: "first.csv", Content: "element,count\nV::car,9\n"},
		{Name: "second.csv", Content: "element,count\nV::vehicle,7\n"},
	}})
	if applied.Error != "" || applied.ModelHash == "" || applied.ModelHash == hash {
		t.Fatalf("import = %v; want a new model", applied)
	}
	if _, ok := srv.cache.Get(applied.ModelHash); !ok {
		t.Errorf("the imported model %s is not cached", applied.ModelHash)
	}
}

func TestImportDataWithoutValuesKeepsTheModel(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, importModelSource)
	resp := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{{Name: "empty.csv", Content: "element,count\n"}}})
	if resp.Error != "" || len(resp.Files) != 1 || resp.Files[0].Values != 0 {
		t.Fatalf("import = %v; want one file of no values", resp)
	}
	if resp.ModelHash != hash {
		t.Errorf("model hash %q; want the unchanged model's %q", resp.ModelHash, hash)
	}
}

func TestImportDataRefusalReportsNoEarlierFiles(t *testing.T) {
	srv := mustNewService(t, 1)
	hash := mustParse(t, srv, importModelSource)
	resp := importData(t, srv, &pb.ImportDataRequest{ModelHash: hash, Imports: []*pb.DataImport{
		{Name: "first.csv", Content: "element,count\nV::car,9\n"},
		{Name: "second.csv", Content: "element,count\nV::Nowhere,1\n"},
	}})
	if resp.Error == "" || !strings.Contains(resp.Error, "V::Nowhere") {
		t.Fatalf("error %q; want the second file refused", resp.Error)
	}
	if len(resp.Files) != 0 || resp.ModelHash != "" || resp.Content != "" || len(resp.Documents) != 0 {
		t.Fatalf("refusal = %v; want nothing reported as imported", resp)
	}
	if _, ok := srv.cache.Get(hash); !ok {
		t.Fatalf("the refused import evicted the model from a cache of one")
	}
}
