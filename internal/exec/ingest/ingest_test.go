package ingest

import (
	"reflect"
	"strings"
	"testing"
)

func cells(rows []Row) []string {
	var out []string
	for _, r := range rows {
		for _, c := range r.Cells {
			out = append(out, r.Element+"::"+c.Feature+"="+c.Text+"["+c.Unit+"]@"+c.Where)
		}
	}
	return out
}

func TestReadDelimitedHeaders(t *testing.T) {
	want := []string{
		"P::a::mass=180[kg]@d line 2, column mass [kg]",
		"P::a::supplier=Acme, Inc.[]@d line 2, column supplier",
		"P::b::supplier=Volt[]@d line 3, column supplier",
	}
	csv := "element,mass [kg],supplier\nP::a,180,\"Acme, Inc.\"\nP::b, ,Volt\n\n"
	rows, err := Read("d", []byte(csv), FormatCSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := cells(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("csv:\n got %q\nwant %q", got, want)
	}
	tsv := "\ufeffelement\tmass [kg]\tsupplier\nP::a\t180\tAcme, Inc.\nP::b\t\tVolt\n"
	rows, err = Read("d", []byte(tsv), FormatTSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := cells(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("tsv:\n got %q\nwant %q", got, want)
	}
}

func TestReadKeepsStringWhitespace(t *testing.T) {
	rows, err := Read("d", []byte("element,label,n\nP::a,\"  Acme  \", 7 \n"), FormatCSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	label, err := Literal(rows[0].Cells[0], ClassString, "")
	if err != nil || label != `"  Acme  "` {
		t.Errorf("label %q, %v; want the quoted field's spaces kept", label, err)
	}
	n, err := Literal(rows[0].Cells[1], ClassInteger, "")
	if err != nil || n != "7" {
		t.Errorf("n %q, %v; want 7", n, err)
	}
}

func TestReadLongForm(t *testing.T) {
	rows, err := Read("d", []byte("element,feature,value,unit\nP::a,mass,180,kg\nP::a,count,3,\n"), FormatCSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"P::a::mass=180[kg]@d line 2, column value", "P::a::count=3[]@d line 3, column value"}
	if got := cells(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestReadMapping(t *testing.T) {
	m, err := ParseMap([]byte(`{"delimiter": ";", "element": {"column": "id", "prefix": "P::"},
		"features": {"mass": {"column": "m", "unitColumn": "u"}, "label": {"column": "n", "type": "string"}},
		"ignore": ["note"]}`))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Read("d", []byte("id;m;u;n;note\na;1.5;t;7;x\n"), FormatCSV, m)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"P::a::mass=1.5[t]@d line 2, column m", "P::a::label=7[]@d line 2, column n"}
	if got := cells(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if rows[0].Cells[1].Type != "string" {
		t.Errorf("label type %q, want string", rows[0].Cells[1].Type)
	}
}

func TestReadJSON(t *testing.T) {
	m := &Map{Records: "/runs", Features: map[string]Field{"power": {Path: "/perf/p", UnitPath: "/perf/u"}}}
	doc := `{"runs": [{"element": "P::a", "count": 3, "ok": true, "perf": {"p": 9.5e3, "u": "W"}, "skip": null}]}`
	rows, err := Read("r.json", []byte(doc), FormatJSON, m)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"P::a::power=9.5e3[W]@r.json record 1, /perf/p",
		"P::a::count=3[]@r.json record 1, field count",
		"P::a::ok=true[]@r.json record 1, field ok",
	}
	if got := cells(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if k := rows[0].Cells[1].Kind; k != KindNumber {
		t.Errorf("count kind %v, want number", k)
	}
	lines := "{\"element\": \"P::a\", \"mass [kg]\": 2}\n\n{\"element\": \"P::b\", \"name\": \"x\"}\n"
	rows, err = Read("r.jsonl", []byte(lines), FormatJSONL, nil)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"P::a::mass=2[kg]@r.jsonl line 1, field mass [kg]", "P::b::name=x[]@r.jsonl line 3, field name"}
	if got := cells(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestReadJSONPathsConsumeTheirMembers(t *testing.T) {
	m := &Map{Element: Field{Path: "/id", Prefix: "P::"}, Features: map[string]Field{"mass": {Path: "/m", UnitPath: "/u"}}}
	rows, err := Read("r.json", []byte(`[{"id": "a", "m": 2, "u": "kg", "count": 1}]`), FormatJSON, m)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"P::a::mass=2[kg]@r.json record 1, /m", "P::a::count=1[]@r.json record 1, field count"}
	if got := cells(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestReadErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		data   string
		format Format
		m      *Map
		want   string
	}{
		"no element column": {"id,mass\na,1\n", FormatCSV, nil, "d line 1: no element column"},
		"blank element":     {"element,mass\n,1\n", FormatCSV, nil, "d line 2, column element: no element is named"},
		"ragged row":        {"element,mass\na,1,2\n", FormatCSV, nil, "wrong number of fields"},
		"duplicate":         {"element,mass\na,1\na,2\n", FormatCSV, nil, "d line 3, column mass: a::mass is already set at d line 2, column mass"},
		"mapped column":     {"element,mass\na,1\n", FormatCSV, &Map{Features: map[string]Field{"mass": {Column: "m"}}}, "feature mass: there is no column m"},
		"json path in csv":  {"element,mass\na,1\n", FormatCSV, &Map{Features: map[string]Field{"mass": {Path: "/m"}}}, "a path is a JSON Pointer"},
		"not an array":      {`{"element": "a"}`, FormatJSON, nil, "the records are not a JSON array"},
		"nested value":      {`[{"element": "a", "perf": {"m": 1}}]`, FormatJSON, nil, "d record 1, field perf: the value is not a number"},
		"bad json line":     {"{\"element\": \"a\"}\n{bad\n", FormatJSONL, nil, "d line 2:"},
		"json line garbage": {"{\"element\": \"a\", \"n\": 1} GARBAGE\n", FormatJSONL, nil, "d line 1: a line holds one JSON value"},
		"json line two":     {"{\"element\": \"a\"} {\"element\": \"b\"}\n", FormatJSONL, nil, "d line 1: a line holds one JSON value"},
		"json trailing":     {"[{\"element\": \"a\"}]]", FormatJSON, nil, "d: text follows the JSON value"},
		"unsupported as":    {"element\n", FormatCSV, &Map{As: "elements"}, `as "elements" is not supported`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Read("d", []byte(tc.data), tc.format, tc.m)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v, want %q", err, tc.want)
			}
		})
	}
	for data, want := range map[string]string{
		`{"colum": "x"}`: "unknown field",
		`{"element": {"column": "id"}} {"features": {"n": {"column": "m"}}}`:       "text follows the mapping object",
		`{"features": {"m": {"path": "/m", "unitPath": "/u", "unitColumn": "u"}}}`: "name its unit once",
	} {
		if _, err := ParseMap([]byte(data)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParseMap(%s): %v, want %q", data, err, want)
		}
	}
}

func TestLiteral(t *testing.T) {
	for _, tc := range []struct {
		cell  Cell
		class Class
		enum  string
		want  string
		err   string
	}{
		{Cell{Text: "180", Unit: "kg"}, ClassReal, "", "180 [kg]", ""},
		{Cell{Text: "+1.", Kind: KindText}, ClassReal, "", "1.0", ""},
		{Cell{Text: "1e3"}, ClassUnknown, "", "1e3", ""},
		{Cell{Text: "7"}, ClassInteger, "", "7", ""},
		{Cell{Text: "7.5"}, ClassInteger, "", "", "not an integer"},
		{Cell{Text: "TRUE"}, ClassBoolean, "", "true", ""},
		{Cell{Text: "yes"}, ClassBoolean, "", "", "not a boolean"},
		{Cell{Text: `say "hi"`}, ClassString, "", `"say \"hi\""`, ""},
		{Cell{Text: "42", Kind: KindNumber}, ClassString, "", `"42"`, ""},
		{Cell{Text: "42", Type: "string"}, ClassUnknown, "", `"42"`, ""},
		{Cell{Text: "high"}, ClassEnum, "P::Grade", "P::Grade::high", ""},
		{Cell{Text: "NaN"}, ClassReal, "", "", "not a finite number"},
		{Cell{Text: "x", Unit: "kg"}, ClassString, "", "", "not a number"},
		{Cell{Text: "1", Unit: "kg]; part p"}, ClassReal, "", "", "is not a unit"},
	} {
		got, err := Literal(tc.cell, tc.class, tc.enum)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%+v: got %q, %v; want error %q", tc.cell, got, err, tc.err)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%+v: got %q, %v; want %q", tc.cell, got, err, tc.want)
		}
	}
}
