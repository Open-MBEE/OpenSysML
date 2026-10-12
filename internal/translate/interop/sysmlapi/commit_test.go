package sysmlapi

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func triple(subject, predicate string, object rdf.Term) rdf.Triple {
	return rdf.Triple{Subject: rdf.IRI(subject), Predicate: rdf.IRI(predicate), Object: object}
}

func TestCommitRequestSpellsEveryChangeKind(t *testing.T) {
	x := rdf.Element + "X1"
	changes := []reposync.ElementChange{
		{Kind: reposync.KindUpdate, ID: "X1", Content: []rdf.Triple{
			triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"PartDefinition")),
			triple(x, rdf.SysML+"declaredName", rdf.String("renamed")),
			triple(x, rdf.SysML+"isAbstract", rdf.TypedLiteral("true", rdf.XSD+"boolean")),
			triple(x, rdf.SysML+"ownedMember", rdf.IRI(rdf.Element+"Y1")),
			triple(x, rdf.SysML+"ownedMember", rdf.IRI(rdf.Expression+"Y2")),
			triple(x, rdf.SysML+"count", rdf.TypedLiteral("3", rdf.XSD+"integer")),
			triple(x, rdf.SysML+"mass", rdf.TypedLiteral("1.5", rdf.XSD+"decimal")),
			triple(x, rdf.SysML+"owner", rdf.IRI(rdf.RDFNS+"nil")),
		}},
		{Kind: reposync.KindCreate, ID: "N1", Content: []rdf.Triple{
			triple(rdf.Element+"N1", rdf.RDFType, rdf.IRI(rdf.SysML+"Package")),
		}},
		{Kind: reposync.KindDelete, ID: "D1"},
	}
	body, err := CommitRequest(changes, "sync")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"@type":       "Commit",
		"description": "sync",
		"change": []any{
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "X1"},
				"payload": map[string]any{
					"@id":          "X1",
					"@type":        "PartDefinition",
					"declaredName": "renamed",
					"isAbstract":   true,
					"ownedMember":  []any{map[string]any{"@id": "Y1"}, map[string]any{"@id": "Y2"}},
					"count":        float64(3),
					"mass":         1.5,
					"owner":        nil,
				},
			},
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "N1"},
				"payload":  map[string]any{"@id": "N1", "@type": "Package"},
			},
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "D1"},
				"payload":  nil,
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("commit request:\n got %s\nwant %+v", body, want)
	}
}

func TestCommitRequestRefusesWhatTheServiceCannotHold(t *testing.T) {
	x := rdf.Element + "X1"
	typed := triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"PartDefinition"))
	cases := []struct {
		name    string
		content []rdf.Triple
		want    error
	}{
		{"untyped", []rdf.Triple{triple(x, rdf.SysML+"declaredName", rdf.String("a"))}, ErrUntyped},
		{"two types", []rdf.Triple{typed, triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"Package"))}, ErrUntyped},
		{"foreign property", []rdf.Triple{typed, triple(x, "urn:opensysml:sysml:memberIndex", rdf.String("0"))}, &UnrepresentableError{}},
		{"language tag", []rdf.Triple{typed, triple(x, rdf.SysML+"declaredName", rdf.Term{Kind: rdf.TermLiteral, Value: "a", Lang: "en"})}, &UnrepresentableError{}},
		{"foreign IRI", []rdf.Triple{typed, triple(x, rdf.SysML+"owner", rdf.IRI("http://example.org/x"))}, &UnrepresentableError{}},
		{"odd datatype", []rdf.Triple{typed, triple(x, rdf.SysML+"when", rdf.TypedLiteral("2020-01-01", rdf.XSD+"date"))}, &UnrepresentableError{}},
		{"non-numeric integer", []rdf.Triple{typed, triple(x, rdf.SysML+"n", rdf.TypedLiteral("+07", rdf.XSD+"integer"))}, &UnrepresentableError{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CommitRequest([]reposync.ElementChange{{Kind: reposync.KindCreate, ID: "X1", Content: tc.content}}, "")
			if err == nil {
				t.Fatal("accepted")
			}
			var unrepresentable *UnrepresentableError
			switch tc.want.(type) {
			case *UnrepresentableError:
				if !errors.As(err, &unrepresentable) {
					t.Errorf("want an UnrepresentableError, got %v", err)
				}
			default:
				if !errors.Is(err, tc.want) {
					t.Errorf("want %v, got %v", tc.want, err)
				}
			}
		})
	}
}

func TestRepresentationCarriesWhatTheServiceStores(t *testing.T) {
	x := rdf.Element + "X1"
	rep := Representation{}
	dropped, ok, err := rep.Carry(triple(x, "urn:opensysml:sysml:memberIndex", rdf.String("0")))
	if err != nil || ok {
		t.Errorf("a foreign property was carried: %+v, %v", dropped, err)
	}
	cases := []struct {
		in, want rdf.Term
	}{
		{rdf.TypedLiteral("a", rdf.XSD+"string"), rdf.String("a")},
		{rdf.TypedLiteral("2", rdf.XSD+"decimal"), rdf.TypedLiteral("2", rdf.XSD+"integer")},
		{rdf.TypedLiteral("2.5", rdf.XSD+"decimal"), rdf.TypedLiteral("2.5", rdf.XSD+"decimal")},
		{rdf.TypedLiteral("-4", rdf.XSD+"integer"), rdf.TypedLiteral("-4", rdf.XSD+"integer")},
		{rdf.TypedLiteral("false", rdf.XSD+"boolean"), rdf.TypedLiteral("false", rdf.XSD+"boolean")},
		{rdf.IRI(rdf.Expression + "E"), rdf.IRI(rdf.Expression + "E")},
		{rdf.IRI(rdf.RDFNS + "nil"), rdf.IRI(rdf.RDFNS + "nil")},
	}
	for _, tc := range cases {
		got, ok, err := rep.Carry(triple(x, rdf.SysML+"p", tc.in))
		if err != nil || !ok {
			t.Errorf("%s: not carried: %v", tc.in, err)
			continue
		}
		if got.Object != tc.want {
			t.Errorf("%s: carried as %s, want %s", tc.in, got.Object, tc.want)
		}
	}
	if _, _, err := rep.Carry(triple(x, rdf.SysML+"p", rdf.TypedLiteral("1e3", rdf.XSD+"decimal"))); err == nil {
		t.Error("a non-JSON number was carried")
	}
}
