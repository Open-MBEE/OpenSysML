//go:build !sysml_re && !sysml_nov1

package grpc

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
)

// unlinkedCapabilities is empty: this build links the SysML v1 migration.
var unlinkedCapabilities []string

// Migrate writes a SysML v1 model as a v2 one, with the account of every v1 element.
// Argument faults fail the call; a refused model is reported in the response's error.
func (s *Service) Migrate(ctx context.Context, req *pb.MigrateRequest) (*pb.MigrateResponse, error) {
	if err := s.requireCapability(CapabilityMigrate); err != nil {
		return nil, err
	}
	from, err := migrateFrom(req)
	if err != nil {
		return nil, err
	}
	to, err := targetFormat(req.ToFormat, true)
	if err != nil {
		return nil, err
	}
	name, data, err := migrateSource(req)
	if err != nil {
		return nil, err
	}
	opts, err := migrateOptions(req)
	if err != nil {
		return nil, err
	}

	resp := &pb.MigrateResponse{
		FromFormat:         from.String(),
		ToFormat:           to.String(),
		Experimental:       true,
		ExperimentalNotice: convert.Notice(from, to),
	}
	migrated, err := convert.Migrate(name, data, to, opts)
	if err != nil {
		resp.Error = err.Error()
		return resp, nil
	}
	resp.Content = string(migrated.Output)
	resp.Report, err = migrationReport(migrated.Report, req.Report)
	if err != nil {
		return nil, statusErrorf(connect.CodeInternal, "writing the migration report: %v", err)
	}
	if req.Results {
		results, err := json.MarshalIndent(migrated.Results, "", "  ")
		if err != nil {
			return nil, statusErrorf(connect.CodeInternal, "writing the migration results: %v", err)
		}
		resp.Results = string(append(results, '\n'))
	}
	for _, path := range slices.Sorted(maps.Keys(migrated.Files)) {
		resp.Files = append(resp.Files, &pb.MigrationFile{Path: path, Content: migrated.Files[path]})
	}
	return resp, nil
}

// migrationReport carries a report's summary and counts, and its entries and
// text when the request asked for the full account.
func migrationReport(report *convert.MigrationReport, full bool) (*pb.MigrationReport, error) {
	counts := report.Count()
	out := &pb.MigrationReport{
		Source:       report.Source,
		Exporter:     report.Exporter,
		Summary:      report.Summary(),
		Mapped:       int32(counts[convert.Mapped]),       // #nosec G115 -- element counts fit
		Approximated: int32(counts[convert.Approximated]), // #nosec G115
		Unmapped:     int32(counts[convert.Unmapped]),     // #nosec G115
		Skipped:      int32(counts[convert.Skipped]),      // #nosec G115
	}
	if !full {
		return out, nil
	}
	out.Entries = make([]*pb.MigrationEntry, 0, len(report.Entries))
	for _, entry := range report.Entries {
		out.Entries = append(out.Entries, &pb.MigrationEntry{
			Id:      entry.ID,
			Kind:    entry.Kind,
			Name:    entry.Name,
			Target:  entry.Target,
			Verdict: entry.Verdict.String(),
			Note:    entry.Note,
		})
	}
	var text strings.Builder
	if err := report.WriteText(&text); err != nil {
		return nil, err
	}
	out.Text = text.String()
	return out, nil
}

// migrateSource reads the v1 model the request names, and the name to report
// it by.
func migrateSource(req *pb.MigrateRequest) (string, []byte, error) {
	switch src := req.Source.(type) {
	case *pb.MigrateRequest_Content:
		return "<content>", src.Content, nil
	case *pb.MigrateRequest_FilePath:
		// #nosec G304 -- reading the model file the client names is the point,
		// and the service runs with the caller's own privileges.
		data, err := os.ReadFile(src.FilePath)
		if err != nil {
			return "", nil, statusErrorf(connect.CodeNotFound, msgFileNotFound, err)
		}
		return src.FilePath, data, nil
	default:
		return "", nil, statusError(connect.CodeInvalidArgument, "source must be file_path or content")
	}
}

// migrateFrom resolves the v1 form, inferring it from the file name when unstated, and
// refuses a v2 format before the file is read, so a missing v2 file is refused as v2.
func migrateFrom(req *pb.MigrateRequest) (convert.Format, error) {
	name := req.GetFilePath()
	if name == "" {
		name = "inline content"
	}
	var from convert.Format
	if req.FromFormat != "" {
		parsed, err := convert.ParseFormat(req.FromFormat)
		if err != nil {
			return 0, statusError(connect.CodeInvalidArgument, err.Error())
		}
		from = parsed
	} else {
		if req.GetFilePath() == "" {
			return 0, statusError(connect.CodeInvalidArgument, "from_format is required for inline content: expected xmi, uml or mdzip")
		}
		inferred, err := convert.FormatOfPath(name)
		if err != nil {
			return 0, statusError(connect.CodeInvalidArgument, convert.Advise(err, "pass from_format, or "+convert.ExtensionAdvice).Error())
		}
		from = inferred
	}
	if from != convert.FormatXMI {
		refused := &convert.NotV1Error{Name: name, Format: from, Remedy: "call Convert with the same source"}
		return 0, statusError(connect.CodeInvalidArgument, refused.Error())
	}
	return from, nil
}

// migrateOptions reads the migration's augments as the command's -layout,
// -image-base-url and -strict read theirs.
func migrateOptions(req *pb.MigrateRequest) (convert.MigrateOptions, error) {
	opts := convert.MigrateOptions{ImageBaseURL: req.ImageBaseUrl, Strict: req.Strict}
	var data []byte
	switch layout := req.Layout.(type) {
	case nil:
		return opts, nil
	case *pb.MigrateRequest_LayoutContent:
		data = []byte(layout.LayoutContent)
		opts.LayoutSource = "<layout_content>"
	case *pb.MigrateRequest_LayoutPath:
		// #nosec G304 -- reading the layout file the client names is the point,
		// and the service runs with the caller's own privileges.
		read, err := os.ReadFile(layout.LayoutPath)
		if err != nil {
			return opts, statusErrorf(connect.CodeNotFound, msgFileNotFound, err)
		}
		data = read
		opts.LayoutSource = layout.LayoutPath
	}
	export, err := mtip.Parse(data)
	if err != nil {
		return opts, statusErrorf(connect.CodeInvalidArgument, "%s: %v", opts.LayoutSource, err)
	}
	opts.Layout = export
	return opts, nil
}
