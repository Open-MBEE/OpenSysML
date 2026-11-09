package opensysml_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

func TestRunInlineDocumentQueryAnswersRows(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, documentSource)
	rows, err := client.RunInlineDocumentQuery(context.Background(), model,
		`Project(WhereType(Descendants(Observatory::telescope), "PartUsage"), ("name", "mass"))`)
	if err != nil {
		t.Fatalf("RunInlineDocumentQuery: %v", err)
	}
	if strings.Join(rows.Columns, ",") != "name,mass" {
		t.Errorf("columns = %v, want name, mass", rows.Columns)
	}
	if len(rows.Rows) == 0 {
		t.Fatal("no rows for the telescope's parts")
	}
	// A fault in the expression is the caller's argument, positioned within it.
	_, err = client.RunInlineDocumentQuery(context.Background(), model, `Project(Named("Observatory") ("name"))`)
	if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), "inline query 1:") {
		t.Errorf("err = %v, want CodeInvalidArgument positioned in the expression", err)
	}
}
