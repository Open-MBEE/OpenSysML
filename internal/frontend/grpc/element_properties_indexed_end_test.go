package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// indexedEndsModel holds plain and indexed connections and flows, with and
// without a reference subsetting on the indexed end.
const indexedEndsModel = `package P {
	port def Port { attribute v : ScalarValues::Integer; }
	part def C {
		port p[3] : Port;
		port q : Port;
		part a[3];
		part b;
		part s { port y[3] : Port; }
		part k { port u : Port; }
		connect p to q;
		connect p#(1) to q;
		connect q to p#(2);
		connect s.y#(1) to k.u;
		flow s.y#(2) to k.u;
		flow a#(1) to b;
	}
}
`

// connectorProperties are a Connector's derived endpoints: the export and
// reflection must agree; the selector lives on connectorEnd, not here.
var connectorProperties = []string{
	"relatedFeature", "sourceFeature", "targetFeature", "source", "target", "connectorEnd",
}

// API JSON and GetElementProperties agree on every connection and flow end,
// with and without a reference subsetting on the indexed end.
func TestIndexedEndsAgreeBetweenAPIJSONAndGetElementProperties(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: indexedEndsModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "api-json",
	})
	if err != nil || converted.Error != "" {
		t.Fatalf("Convert: %v %s", err, converted.Error)
	}
	elements, byID := apiJSONElements(t, converted.Content)

	compared := 0
	for _, el := range elements {
		if _, isConnector := el["connectorEnd"]; !isConnector {
			continue
		}
		id, _ := el["@id"].(string)
		resp, err := srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{
			ModelHash: parsed.ModelHash, ElementId: id, Properties: connectorProperties,
		})
		if err != nil || resp.Error != "" {
			t.Fatalf("GetElementProperties(%s): %v %s", id, err, resp.Error)
		}
		answered := map[string]*pb.ElementProperty{}
		for _, property := range resp.Properties {
			answered[property.Name] = property
		}
		for _, name := range connectorProperties {
			want := apiJSONSpelling(el[name])
			got, ok := answered[name]
			if !ok {
				t.Errorf("%s.%s: reflection answered no property, export says %q", id, name, want)
				continue
			}
			if got.Unsupported != "" {
				t.Errorf("%s.%s: reflection is unsupported (%s), export says %q", id, name, got.Unsupported, want)
				continue
			}
			if have := propertySpelling(got); have != want {
				t.Errorf("%s.%s: export says %q, reflection says %q", id, name, want, have)
			}
		}
		compared++
	}
	if compared != 6 {
		t.Fatalf("compared %d connector heads, want the six of the model", compared)
	}

	// The indexed and plain connections answer the same source port; the index
	// is on the end reached through connectorEnd, where reflection does not look.
	plain := elementBySourceText(t, elements, "connect p to q;")
	indexed := elementBySourceText(t, elements, "connect p#(1) to q;")
	if plain["sourceFeature"] == nil || indexed["sourceFeature"] == nil {
		t.Fatalf("the connections state no sourceFeature\n%s", converted.Content)
	}
	if fmt.Sprint(plain["sourceFeature"]) != fmt.Sprint(indexed["sourceFeature"]) {
		t.Errorf("indexed sourceFeature = %v, want the plain connection's %v", indexed["sourceFeature"], plain["sourceFeature"])
	}
	ends, _ := indexed["connectorEnd"].([]any)
	if len(ends) != 2 {
		t.Fatalf("connectorEnd = %v, want two ends", ends)
	}
	assertIndexedEnd(t, byID, ends[0], "p#(1)", "1")
	assertPlainEnd(t, byID, ends[1])

	// The flow end with a chain prefix states the prefix as its source, exactly
	// as reflection does; the lone-name indexed end states no source at all.
	flow := elementBySourceText(t, elements, "flow s.y#(2) to k.u;")
	lone := elementBySourceText(t, elements, "flow a#(1) to b;")
	if flow["sourceFeature"] == nil || lone["sourceFeature"] != nil {
		t.Errorf("flow sourceFeature = %v, lone = %v; want a prefix and none", flow["sourceFeature"], lone["sourceFeature"])
	}
	resp := mustReadProperties(t, srv, parsed.ModelHash, lone["@id"].(string), "sourceFeature", "relatedFeature", "connectorEnd")
	for _, property := range resp.Properties {
		if property.Name != "connectorEnd" && len(property.Values) != 0 {
			t.Errorf("lone indexed flow %s = %v, want no referenced feature", property.Name, property.Values)
		}
		if property.Name == "connectorEnd" && len(property.Values) != 2 {
			t.Errorf("lone indexed flow connectorEnd = %d ends, want 2", len(property.Values))
		}
	}
	loneEnds, _ := lone["connectorEnd"].([]any)
	if len(loneEnds) != 2 {
		t.Fatalf("lone flow connectorEnd = %v, want two ends", loneEnds)
	}
	assertIndexedEnd(t, byID, loneEnds[0], "a#(1)", "1")
	assertPlainEnd(t, byID, loneEnds[1])

	// Notation still reads back byte for byte from the element form.
	back, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_Content{Content: converted.Content}, FromFormat: "api-json", ToFormat: "sysml",
	})
	if err != nil || back.Error != "" {
		t.Fatalf("Convert back: %v %s", err, back.Error)
	}
	for _, want := range []string{"connect p#(1) to q;", "connect q to p#(2);", "connect s.y#(1) to k.u;", "flow s.y#(2) to k.u;", "flow a#(1) to b;"} {
		if !strings.Contains(back.Content, want) {
			t.Errorf("the element form should read back with %q\n%s", want, back.Content)
		}
	}
}

func assertIndexedEnd(t *testing.T, byID map[string]map[string]any, ref any, text, index string) {
	t.Helper()
	end := byID[ref.(map[string]any)["@id"].(string)]
	if end["sysx:sourceText"] != text || end["isEnd"] != true {
		t.Errorf("end = %v, want an end written %q", end, text)
	}
	element, ok := end["sysx:endElement"].(map[string]any)
	if !ok {
		t.Fatalf("%v states no sysx:endElement", end)
	}
	expr := byID[element["@id"].(string)]
	if expr["sysx:sourceText"] != index {
		t.Errorf("index expression = %v, want %q", expr, index)
	}
}

func assertPlainEnd(t *testing.T, byID map[string]map[string]any, ref any) {
	t.Helper()
	end := byID[ref.(map[string]any)["@id"].(string)]
	if _, indexed := end["sysx:endElement"]; indexed {
		t.Errorf("a plain end carries an index: %v", end)
	}
}

func mustReadProperties(t *testing.T, srv *Service, hash, id string, names ...string) *pb.GetElementPropertiesResponse {
	t.Helper()
	resp, err := srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{
		ModelHash: hash, ElementId: id, Properties: names,
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("GetElementProperties(%s, %v): %v %s", id, names, err, resp.Error)
	}
	return resp
}

func apiJSONElements(t *testing.T, content string) ([]map[string]any, map[string]map[string]any) {
	t.Helper()
	var elements []map[string]any
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, el := range elements {
		id, _ := el["@id"].(string)
		byID[id] = el
	}
	return elements, byID
}

func elementBySourceText(t *testing.T, elements []map[string]any, text string) map[string]any {
	t.Helper()
	for _, el := range elements {
		if strings.TrimSpace(fmt.Sprint(el["sysx:sourceText"])) == text {
			return el
		}
	}
	t.Fatalf("no element written %q", text)
	return nil
}

// apiJSONSpelling flattens an exported property the way GetElementProperties'
// values are flattened below: element references by id, in collection order.
func apiJSONSpelling(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []any:
		parts := make([]string, 0, len(v))
		for _, member := range v {
			parts = append(parts, apiJSONSpelling(member))
		}
		sort.Strings(parts) // relatedFeature is a set in one surface, a list in the other
		return strings.Join(parts, " | ")
	case map[string]any:
		id, _ := v["@id"].(string)
		return "element " + id
	case json.Number:
		return "number " + v.String()
	case string:
		return "string " + v
	case bool:
		return fmt.Sprintf("bool %t", v)
	}
	return fmt.Sprintf("%T %v", value, value)
}

func propertySpelling(property *pb.ElementProperty) string {
	parts := make([]string, 0, len(property.Values))
	for _, value := range property.Values {
		if el := value.GetElement(); el != nil {
			parts = append(parts, "element "+el.Id)
			continue
		}
		parts = append(parts, fmt.Sprintf("%T", value.Value))
	}
	sort.Strings(parts)
	return strings.Join(parts, " | ")
}
