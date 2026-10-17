package codes

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
)

// documentCodes are the codes of the document and query planning passes: two
// families whose members name the planning rule the message states.
var documentCodes = []Entry{
	{
		Code: "document-plan", Tier: TierDocument, Severity: "error",
		Summary: "a document definition the document planner cannot compile",
		Meaning: "A document definition (one typed by `DocumentDefinitions::Document` or a specialization of it) is compiled into a document plan before it is rendered; the member code names the rule that failed — a content with no title, an attribute that is not one string literal, a nested document, a query the document does not declare, a diagram kind no rendering supports — and the message states where. " +
			"Correct the member the message names; the plain `document-plan` code reports a failure no rule classifies, with the planner's own message.",
		Spec:  "OpenSysML DocumentDefinitions library",
		Kinds: docplanKinds(),
	},
	{
		Code: "document-query", Tier: TierDocument, Severity: "error",
		Summary: "a document query definition the query planner cannot compile",
		Meaning: "A query definition (one typed by `DocumentQueries::Query` or a specialization of it) is compiled into a query plan; the member code names the rule that failed — a missing result parameter, an argument of the wrong type or count, a composition cycle, a column with no source — and the message states where. " +
			"Correct the member the message names; the plain `document-query` code reports a failure no rule classifies, with the planner's own message.",
		Spec:  "OpenSysML DocumentQueries library",
		Kinds: queryplanKinds(),
	},
}

// docplanKinds lists the document planning error kinds, as docplan.ErrorKind spells them.
func docplanKinds() []string {
	kinds := make([]string, 0, len(docplan.ErrorKinds()))
	for _, k := range docplan.ErrorKinds() {
		kinds = append(kinds, string(k))
	}
	return kinds
}

// queryplanKinds lists the query planning error kinds, as queryplan.ErrorKind spells them.
func queryplanKinds() []string {
	kinds := make([]string, 0, len(queryplan.ErrorKinds()))
	for _, k := range queryplan.ErrorKinds() {
		kinds = append(kinds, string(k))
	}
	return kinds
}
