package grpc

import (
	"context"
	"sync"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/proto"
)

func TestConvertPreparedModelLifecycle(t *testing.T) {
	srv := mustNewService(t, 2)
	defer srv.Close()
	parse := func(s *Service, revision string) string {
		t.Helper()
		response, err := s.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments("lib.sysml", "package Lib { part def Motor; }", "app.sysml", "package App { private import Lib::*; part "+revision+" : Motor; }")})
		if err != nil {
			t.Fatal(err)
		}
		return response.ModelHash
	}
	convert := func(s *Service, hash string, options *pb.ConvertRequest) *pb.ConvertResponse {
		t.Helper()
		req := proto.Clone(options).(*pb.ConvertRequest)
		req.Source = &pb.ConvertRequest_ModelHash{ModelHash: hash}
		req.ToFormat = "api-json"
		response, err := s.Convert(context.Background(), req)
		if err != nil {
			t.Error(err)
			return nil
		}
		if response.Error != "" {
			t.Error(response.Error)
			return nil
		}
		return response
	}
	oldHash := parse(srv, "motorA")
	old := convert(srv, oldHash, &pb.ConvertRequest{})
	oldModel, _ := srv.cache.Get(oldHash)
	newHash := parse(srv, "motorB")
	fresh := mustNewService(t, 2)
	defer fresh.Close()
	freshHash := parse(fresh, "motorB")
	var wg sync.WaitGroup
	for _, options := range []*pb.ConvertRequest{{}, {Documents: []string{"app.sysml"}}, {Documents: []string{"lib.sysml"}, IdForm: "uuid"}, {Compact: true, OmitDerived: true, KeepDerived: []string{"name", "shortName", "qualifiedName"}}} {
		want := convert(fresh, freshHash, options)
		for range 2 {
			wg.Go(func() {
				got := convert(srv, newHash, options)
				if !proto.Equal(got, want) {
					t.Errorf("concurrent export differs: %v", options)
				}
			})
		}
	}
	wg.Wait()
	if !proto.Equal(convert(srv, oldHash, &pb.ConvertRequest{}), old) {
		t.Fatal("new model changed old model export")
	}
	newModel, _ := srv.cache.Get(newHash)
	if oldModel.exportModel == nil || newModel.exportModel == nil || oldModel.exportModel == newModel.exportModel {
		t.Fatal("prepared state shared across model revisions")
	}
	// Retain the new model, then insert a third: the older model must be evicted.
	parse(srv, "motorC")
	_, err := srv.Convert(context.Background(), &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: oldHash}, ToFormat: "api-json"})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("evicted model: %v", err)
	}
	reparsed := parse(srv, "motorA")
	if !proto.Equal(convert(srv, reparsed, &pb.ConvertRequest{}), old) {
		t.Fatal("reparse differs after eviction")
	}
	replacement, _ := srv.cache.Get(reparsed)
	if replacement.exportModel == oldModel.exportModel {
		t.Fatal("evicted preparation reused")
	}
}
