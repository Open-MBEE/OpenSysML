package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const absentPropertyModel = `package Demo {
	part def Vehicle;
	part vehicle : Vehicle;
	comment about vehicle /*A note*/
	requirement def Safe { subject v : Vehicle; }
	requirement safe : Safe { subject v : Vehicle; }
	assert satisfy safe by vehicle;
	action def Watch { action wait accept after 5 [SI::s]; }
}`

// A property the metamodel declares is absent on the elements without it, never a refusal,
// even when no answered element carries it or OpenSysML does not derive it yet.
func TestQueryAnswersADeclaredPropertyNoElementCarriesAsAbsent(t *testing.T) {
	_, _, resp := runQueryOnSource(t, absentPropertyModel, &pb.Query{
		Where:  primitive("@type", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, false, "PartUsage"),
		Select: []string{"name", "body", "payloadArgument"},
	})
	if len(resp.Elements) != 1 || resp.Elements[0].Id != "Demo::vehicle" {
		t.Fatalf("elements = %v, want Demo::vehicle alone", resp.Elements)
	}
	if props := resp.Elements[0].Properties; len(props) != 1 || props["name"] != "vehicle" {
		t.Fatalf("properties = %v, want name alone", props)
	}
	_, _, resp = runQueryOnSource(t, absentPropertyModel, &pb.Query{Select: []string{"payloadArgument", "body"}})
	bodies := map[string]string{}
	for _, element := range resp.Elements {
		if _, ok := element.Properties["payloadArgument"]; ok {
			t.Fatalf("%s answers payloadArgument %q, want absent", element.Id, element.Properties["payloadArgument"])
		}
		if body, ok := element.Properties["body"]; ok {
			bodies[element.Type] = body
		}
	}
	if len(bodies) != 1 || bodies["Comment"] != "A note" {
		t.Fatalf("bodies = %v, want the comment's alone", bodies)
	}
}

// An empty select reports every property but elementId, those of particular
// metaclasses on the elements that carry them.
func TestQueryDefaultSelectionReportsTheMetaclassProperties(t *testing.T) {
	_, _, resp := runQueryOnSource(t, absentPropertyModel, &pb.Query{})
	byType := map[string]map[string]string{}
	for _, element := range resp.Elements {
		byType[element.Type] = element.Properties
	}
	if byType["Comment"]["body"] != "A note" {
		t.Fatalf("comment = %v, want body", byType["Comment"])
	}
	if got := byType["SatisfyRequirementUsage"]; got["satisfiedRequirement"] != "Demo::safe" || got["satisfyingFeature"] != "Demo::vehicle" {
		t.Fatalf("satisfy = %v, want satisfiedRequirement and satisfyingFeature", got)
	}
	if _, ok := byType["PartUsage"]["elementId"]; ok {
		t.Fatalf("part = %v, want no elementId without a select naming it", byType["PartUsage"])
	}
	if got := byType["PartUsage"]; got["type"] != "Demo::Vehicle" || got["isAbstract"] != "false" {
		t.Fatalf("part = %v, want type and isAbstract", got)
	}
}

// A row lacking the sort key sorts first ascending and last descending.
func TestOSLCQueryDescendingOrderPutsMissingKeysLast(t *testing.T) {
	srv, hash, _ := runQueryOnSource(t, absentPropertyModel, &pb.Query{})
	descending := oslcQuery(t, srv, hash, "oslc.select=sysml:name&oslc.orderBy=-sysml:name").Elements
	ascending := oslcQuery(t, srv, hash, "oslc.select=sysml:name&oslc.orderBy=%2Bsysml:name").Elements
	if len(descending) < 3 || len(ascending) != len(descending) {
		t.Fatalf("rows = %d descending, %d ascending", len(descending), len(ascending))
	}
	nameless := 0
	for _, element := range descending {
		if element.Properties["name"] == "" {
			nameless++
		}
	}
	if nameless == 0 {
		t.Fatalf("no nameless element among %v", descending)
	}
	for i, element := range descending {
		if _, named := element.Properties["name"]; named != (i < len(descending)-nameless) {
			t.Fatalf("descending puts %s at %d: %v", element.Id, i, descending)
		}
	}
	for i, element := range ascending {
		if _, named := element.Properties["name"]; named != (i >= nameless) {
			t.Fatalf("ascending puts %s at %d: %v", element.Id, i, ascending)
		}
	}
	if descending[0].Properties["name"] != ascending[len(ascending)-1].Properties["name"] {
		t.Fatalf("descending %v is not the reverse of ascending %v", descending, ascending)
	}
}
