package grpc

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

const idFormModel = "package P { part def A; part a : A; }\n"

// id_form spells derived ids as `sysml -id` does: the service's uuid form is
// byte for byte the conversion's, and the default is the qualified form (#732).
func TestConvertIDFormIsTheCommandLines(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	for _, form := range []string{"", "qualified", "uuid"} {
		resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
			Source:     &pb.ConvertRequest_Content{Content: idFormModel},
			FromFormat: "sysml",
			ToFormat:   "api-json",
			IdForm:     form,
		})
		if err != nil || resp.Error != "" {
			t.Fatalf("id_form %q: %v %s", form, err, resp.GetError())
		}
		parsed, _ := export.ParseIDForm(form)
		want, err := convert.ConvertWith("<content>", []byte(idFormModel), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{ID: parsed})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Content != string(want) {
			t.Errorf("id_form %q: the service's output differs from the conversion's", form)
		}
		ids := apiJSONIDs(t, resp.Content)
		if _, qualified := ids["P::a"]; !qualified {
			t.Fatalf("id_form %q: no element P::a", form)
		}
		if got := ids["P::a"]; (form == "uuid") == (got == "P__a") {
			t.Errorf("id_form %q: P::a has @id %q", form, got)
		}
	}
}

// A model parsed from several documents takes id_form too.
func TestConvertModelOfSeveralDocumentsTakesIDForm(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"lib.sysml", "package Lib { part def Engine; }\n",
			"app.sysml", "package App { private import Lib::*; part e : Engine; }\n"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash},
		ToFormat: "api-json",
		IdForm:   "uuid",
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("Convert: %v %s", err, resp.GetError())
	}
	for name, id := range apiJSONIDs(t, resp.Content) {
		if id == "Lib__Engine" || id == "App__e" {
			t.Errorf("%s kept its qualified-form @id %q under id_form uuid", name, id)
		}
	}
}

// id_form is refused where `-id` is: for a notation target, and for a value
// that names no id form.
func TestConvertIDFormRefusals(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"lib.sysml", "package Lib { part def Engine; }\n",
			"app.sysml", "package App { private import Lib::*; part e : Engine; }\n"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	for name, req := range map[string]*pb.ConvertRequest{
		"notation target": {Source: &pb.ConvertRequest_Content{Content: idFormModel}, FromFormat: "sysml", ToFormat: "sysml", IdForm: "uuid"},
		// Judged before the several-document model's notation refusal, so the
		// same request is refused the same way for one document or several.
		"notation target, several documents": {Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "sysml", IdForm: "uuid"},
		"unknown form":                       {Source: &pb.ConvertRequest_Content{Content: idFormModel}, FromFormat: "sysml", ToFormat: "api-json", IdForm: "guid"},
	} {
		if _, err := srv.Convert(context.Background(), req); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want INVALID_ARGUMENT", name, err)
		}
	}
}

// apiJSONIDs maps each named element of an API JSON document to its @id.
func apiJSONIDs(t *testing.T, content string) map[string]string {
	t.Helper()
	var elements []map[string]any
	if err := json.Unmarshal([]byte(content), &elements); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range elements {
		if name, ok := e["qualifiedName"].(string); ok {
			out[name], _ = e["@id"].(string)
		}
	}
	return out
}
