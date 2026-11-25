package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const orderedAnnotationsModel = `package Demo {
	private import DocumentQueries::*;
	private import ScalarValues::*;
	metadata def Outer {
		attribute nested {
			attribute value : String;
		}
	}
	metadata def Tag { attribute label : String; }
	part a {
		@Outer { nested { value = "first"; } }
		@Tag { label = "second"; }
	}
}`

// The Query RPC reports one value per cell, so the first binding written wins:
// an earlier annotation's nested binding outranks a later direct one.
func TestQueryRPCReportsFirstAnnotationInDeclarationOrder(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedAnnotationsModel, &pb.Query{
		Scope:  []string{"Demo::a"},
		Select: []string{QueryPropAnnotations},
	})
	rows := rowsByID(t, resp)
	if got := rows["Demo::a"].Properties[QueryPropAnnotations]; got != "Demo::Outer::nested.value=first" {
		t.Fatalf("first annotation of Demo::a = %q, want Demo::Outer::nested.value=first", got)
	}
}
