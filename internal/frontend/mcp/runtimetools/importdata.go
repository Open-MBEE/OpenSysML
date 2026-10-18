package runtimetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// DataImportInput is one data file of import_data, the ImportDataRequest's DataImport.
type DataImportInput struct {
	Name         string `json:"name" jsonschema:"the file's name, which names its format by extension (csv, tsv, json, jsonl) and the file in what goes wrong"`
	Content      string `json:"content" jsonschema:"the file's text: one row per element, an element column naming the element and each other column a feature, as 'mass [kg]'"`
	ImportFormat string `json:"importFormat,omitempty" jsonschema:"read the content as csv, tsv, json or jsonl rather than as the name's extension says (the CLI's -import-format)"`
	ImportMap    string `json:"importMap,omitempty" jsonschema:"the JSON mapping onto elements and features the content is read by (the CLI's -import-map file); empty for the content's own header"`
}

// ImportDataInput is the input of import_data: the ImportDataRequest under proto JSON names.
type ImportDataInput struct {
	ModelInput
	Imports         []DataImportInput `json:"imports" jsonschema:"the data files, applied in order as one edit each"`
	ImportAs        string            `json:"importAs,omitempty" jsonschema:"what each row is made into (the CLI's -import-as): values, the default, sets the features of the element the row names"`
	ImportDryRun    bool              `json:"importDryRun,omitempty" jsonschema:"report the values the imports would set, changing nothing (the CLI's -import-dry-run)"`
	AcceptDocuments bool              `json:"acceptDocuments,omitempty" jsonschema:"accept every edited document in documents when the model has several"`
}

func (g *group) registerImportData(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "import_data",
		Description: "Set the feature values a CSV, TSV, JSON or JSON Lines file assigns, one row per element (the ImportData RPC, " +
			"the CLI's -import with -import-as, -import-map, -import-format and -import-dry-run): a feature the element declares " +
			"has its value set, one it inherits is redefined in the element's body with the value, as one edit per file; a value " +
			"the model refuses leaves every row unapplied and answers `error`. `files` carry each assignment with the element, " +
			"feature, value and the inherited feature it redefines; the edited text comes back as `content` (one document) or " +
			"`documents` with the `modelHash` of the imported model, nothing is written to disk. A dry run answers the assignments " +
			"alone. The result is the ImportDataResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Import data"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ImportDataInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.ImportDataRequest{}
		var resp *pb.ImportDataResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			var err error
			resp, err = g.svc.ImportData(ctx, req)
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			for _, f := range resp.GetFiles() {
				fmt.Fprintf(&b, "%s: %d value(s) into %d element(s)\n", f.GetName(), f.GetValues(), f.GetElements())
				for _, a := range f.GetAssignments() {
					fmt.Fprintf(&b, "  %s\n", a.GetDescription())
				}
			}
			if resp.GetModelHash() != "" {
				fmt.Fprintf(&b, "model hash %s\n", resp.GetModelHash())
			}
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			for _, d := range resp.GetDocuments() {
				fmt.Fprintf(&b, "--- %s\n%s", d.GetName(), d.GetContent())
				if !strings.HasSuffix(d.GetContent(), "\n") {
					b.WriteByte('\n')
				}
			}
			return b.String()
		})
	})
}
