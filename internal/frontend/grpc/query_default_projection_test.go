package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// unevaluableNamesakes declares, per property of the default projection that reads a
// same-named feature first, a part whose feature of that name cannot be evaluated.
const unevaluableNamesakes = `package Namesakes {
	part withText { attribute text = 1 / 0; }
	part withBody { attribute body = 1 / 0; }
	part withLocale { attribute locale = 1 / 0; }
	part withLanguage { attribute language = 1 / 0; }
	part sound { attribute text = "stated"; }
	requirement stated { doc /* The mass is bounded. */ }
}
`

var namesakeProperties = []string{QueryPropText, "body", "locale", "language"}

func namesakeOwner(property string) string {
	return "Namesakes::with" + strings.ToUpper(property[:1]) + property[1:]
}

func TestQuerySelectingNothingToleratesAnUnevaluableNamesake(t *testing.T) {
	_, _, resp := runQueryOnSource(t, unevaluableNamesakes, &pb.Query{Scope: []string{"Namesakes"}})
	rows := map[string]map[string]string{}
	for _, element := range resp.Elements {
		rows[element.Id] = element.Properties
	}
	for _, property := range namesakeProperties {
		id := namesakeOwner(property)
		row, listed := rows[id]
		if !listed {
			t.Fatalf("%s is not listed", id)
		}
		if got, ok := row[property]; ok {
			t.Errorf("%s reports %s = %q, want it absent", id, property, got)
		}
		if row[QueryPropSymbolKind] != "partUsage" || row[QueryPropName] != id[len("Namesakes::"):] {
			t.Errorf("%s lost its other properties: %v", id, row)
		}
	}
	if got := rows["Namesakes::sound"][QueryPropText]; got != "stated" {
		t.Errorf("an evaluable text feature = %q, want stated", got)
	}
	if got := rows["Namesakes::stated"][QueryPropText]; got != "The mass is bounded." {
		t.Errorf("requirement text = %q", got)
	}
}

func TestQuerySelectingAnUnevaluableNamesakeStillFails(t *testing.T) {
	srv, hash, _ := runQueryOnSource(t, unevaluableNamesakes, &pb.Query{Scope: []string{"Namesakes"}})
	for _, property := range namesakeProperties {
		_, err := srv.Query(context.Background(), &pb.QueryRequest{
			ModelHash: hash,
			Query:     &pb.Query{Scope: []string{"Namesakes"}, Select: []string{property}},
		})
		if err == nil || !strings.Contains(err.Error(), "cannot evaluate feature "+property+" of "+namesakeOwner(property)) {
			t.Errorf("select %s: err = %v, want the unevaluable feature named", property, err)
		}
	}
}

func TestQueryOrderingByAnUnevaluableNamesakeStillFails(t *testing.T) {
	srv, hash, _ := runQueryOnSource(t, unevaluableNamesakes, &pb.Query{Scope: []string{"Namesakes"}})
	for _, property := range namesakeProperties {
		_, err := srv.Query(context.Background(), &pb.QueryRequest{
			ModelHash: hash,
			Query:     &pb.Query{Scope: []string{"Namesakes"}, OrderBy: []string{property}},
		})
		if err == nil || !strings.Contains(err.Error(), "cannot evaluate feature "+property+" of "+namesakeOwner(property)) {
			t.Errorf("orderBy %s: err = %v, want the unevaluable feature named", property, err)
		}
	}
}
