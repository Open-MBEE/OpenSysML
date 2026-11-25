package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const mixedActivityFilterModel = `package Demo {
	private import ScalarValues::*;
	part a { attribute isActive : String = "ready"; }
	state def Machine {
		entry; then ready;
		state ready;
	}
}`

// A bare name reads a string feature on one row and Boolean activity on
// another; the state row is skipped rather than failing the request.
func TestQueryFiltersShadowedActivityAcrossMixedRows(t *testing.T) {
	_, _, resp := runQueryOnSource(t, mixedActivityFilterModel, &pb.Query{
		Where:  primitive(QueryPropIsActive, opEqual, false, "ready"),
		Select: []string{QueryPropName},
	})
	rows := rowsByID(t, resp)
	if rows["Demo::a"] == nil {
		t.Fatalf("filter on the string isActive = %v, want Demo::a", resp.Elements)
	}
}
