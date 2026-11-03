package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestGRPCRobustnessQueryOperators gives the query surface malformed shapes of
// the inclusive comparisons, `in`, `instanceOf` and `orderBy`: each refuses the
// call as INVALID_ARGUMENT naming the fault, over a model with elements and
// over one with none, and nothing panics or answers "nothing matched".
func TestGRPCRobustnessQueryOperators(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, queryModel)
	empty := mustParse(t, srv, "package Empty;")
	shapes := map[string]struct {
		query *pb.Query
		fault string
	}{
		"<= with a value list":        {&pb.Query{Where: primitive(QueryPropMultiplicityLower, opLessEqual, false, "1", "2")}, "exactly one value"},
		">= against a non-number":     {&pb.Query{Where: primitive(QueryPropMultiplicityUpper, opGreaterEqual, false, "lots")}, "needs a number"},
		"<= on an unordered property": {&pb.Query{Where: primitive(QueryPropDocumentation, opLessEqual, false, "1")}, "not ordered"},
		"in with no list":             {&pb.Query{Where: primitive(QueryPropType, opIn, false)}, "no value"},
		"instanceOf on name":          {&pb.Query{Where: primitive(QueryPropName, opInstanceOf, false, "Usage")}, "instanceOf applies to"},
		"instanceOf of nothing":       {&pb.Query{Where: primitive(QueryPropType, opInstanceOf, false, "")}, "metaclass name"},
		"instanceOf of no metaclass":  {&pb.Query{Where: primitive(QueryPropType, opInstanceOf, false, "Widget")}, "library declares"},
		"instanceOf nested deep":      {&pb.Query{Where: composite(pb.CompositeOperator_COMPOSITE_OPERATOR_AND, primitive(QueryPropName, opEqual, false, "x"), composite(pb.CompositeOperator_COMPOSITE_OPERATOR_OR, primitive(QueryPropName, opEqual, false, "y"), primitive(QueryPropType, opInstanceOf, false, "Widget")))}, "library declares"},
		"orderBy unknown property":    {&pb.Query{OrderBy: []string{"colour"}}, "unknown query property"},
		"orderBy empty name":          {&pb.Query{OrderBy: []string{""}}, "unknown query property"},
		"orderBy unknown after known": {&pb.Query{OrderBy: []string{QueryPropName, QueryPropType, "size"}}, "unknown query property"},
	}
	for name, shape := range shapes {
		for model, modelHash := range map[string]string{"populated": hash, "empty": empty} {
			_, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: modelHash, Query: shape.query})
			var cerr *connect.Error
			if !errors.As(err, &cerr) || cerr.Code() != connect.CodeInvalidArgument {
				t.Errorf("%s over %s model: err = %v, want INVALID_ARGUMENT", name, model, err)
				continue
			}
			if !strings.Contains(cerr.Message(), shape.fault) {
				t.Errorf("%s over %s model: message %q does not name the fault %q", name, model, cerr.Message(), shape.fault)
			}
		}
	}
	// Repeating a sort property and sorting an empty answer are not faults.
	for name, query := range map[string]*pb.Query{
		"repeated orderBy":        {OrderBy: []string{QueryPropName, QueryPropName}},
		"orderBy over no matches": {Where: primitive(QueryPropName, opEqual, false, "nobody"), OrderBy: []string{QueryPropMultiplicityUpper}},
	} {
		if _, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: query}); err != nil {
			t.Errorf("%s: err = %v, want success", name, err)
		}
	}
}
