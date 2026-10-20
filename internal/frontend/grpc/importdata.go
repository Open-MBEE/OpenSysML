package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/dataimport"
)

// ImportData sets the feature values each data file assigns, as one edit of the
// model per file, as `sysml -import` does: a feature the element declares has
// its value set, one it inherits is redefined in the element's body with the
// value. A file the model refuses leaves the model as it was.
func (s *Service) ImportData(ctx context.Context, req *pb.ImportDataRequest) (*pb.ImportDataResponse, error) {
	if err := s.requireCapability(CapabilityImportData); err != nil {
		return nil, err
	}
	if err := importRequestError(req); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound,
			"model %s is no longer cached: parse it again before importing into it", req.ModelHash)
	}
	documents := s.capabilities.has(CapabilityEditDocuments)
	if !documents && len(cached.Documents) != 1 {
		return nil, statusErrorf(connect.CodeFailedPrecondition,
			"the model has %d documents, and this service edits one alone: it lacks the %s capability",
			len(cached.Documents), CapabilityEditDocuments)
	}
	if err := acceptsDocuments(cached, req.AcceptDocuments); err != nil {
		return nil, err
	}

	own := make(map[string]bool, len(cached.Documents))
	for _, doc := range cached.Documents {
		own[doc.Source.Name()] = true
	}
	resp := &pb.ImportDataResponse{}
	current := cached
	for _, in := range req.Imports {
		if err := ctx.Err(); err != nil {
			return nil, statusError(connect.CodeCanceled, err.Error())
		}
		ops, file, err := lowerImport(current, own, in)
		if err != nil {
			return importRefused(resp, err), nil
		}
		resp.Files = append(resp.Files, file)
		if file.Values == 0 {
			continue
		}
		model, hash, err := s.importAssignments(current, ops, resp)
		if err != nil {
			return importRefused(resp, err), nil
		}
		current, resp.ModelHash = model, hash
	}
	if req.ImportDryRun {
		resp.ModelHash = ""
		return resp, nil
	}
	// An import that assigned nothing leaves the model as it was, under its hash;
	// one that changed it keeps the final model alone, none staged on the way.
	if resp.ModelHash == "" {
		resp.ModelHash = req.ModelHash
	} else {
		current = s.cache.Add(resp.ModelHash, current)
	}
	resp.Documents = changedDocuments(cached, current)
	if len(cached.Documents) == 1 && len(resp.Documents) == 1 {
		resp.Content = resp.Documents[0].Content
	}
	if !documents {
		resp.Documents = nil
	}
	return resp, nil
}

// importRequestError is the argument error of an import request, or nil.
func importRequestError(req *pb.ImportDataRequest) error {
	if req.ImportAs != "" && req.ImportAs != "values" {
		return statusErrorf(connect.CodeInvalidArgument, "import_as %q is not supported; only values is", req.ImportAs)
	}
	if len(req.Imports) == 0 {
		return statusError(connect.CodeInvalidArgument, "imports is required: name at least one data file")
	}
	for i, in := range req.Imports {
		if in.GetName() == "" {
			return statusErrorf(connect.CodeInvalidArgument, "imports[%d].name is required: it names the file, and its format by extension", i)
		}
	}
	if req.ModelHash == "" {
		return statusError(connect.CodeInvalidArgument, "model_hash is required")
	}
	return nil
}

// lowerImport reads one data file and lowers its rows to the assignments they
// make on the model, reported per value in the file's entry of the response.
func lowerImport(model *CachedModel, own map[string]bool, in *pb.DataImport) ([]dataimport.Assignment, *pb.ImportedFile, error) {
	opts := dataimport.Options{Format: in.GetImportFormat()}
	if in.GetImportMap() != "" {
		opts.Mapping = []byte(in.GetImportMap())
	}
	rows, err := dataimport.Read(in.GetName(), []byte(in.GetContent()), opts)
	if err != nil {
		return nil, nil, err
	}
	sc := model.SymbolContext()
	unlock := sc.Lock()
	ops, summary, err := dataimport.Lower(sc.Index, sc.Resolver, sc.Semantics, func(doc string) bool { return own[doc] }, rows)
	unlock()
	if err != nil {
		return nil, nil, err
	}
	file := &pb.ImportedFile{Name: in.GetName(), Values: int64(summary.Values), Elements: int64(summary.Elements)}
	for _, op := range ops {
		file.Assignments = append(file.Assignments, &pb.ImportedValue{
			Element: op.Element, Feature: op.Feature, Value: op.Value, Redefines: op.Redefines,
			Where: op.Where, Description: op.Description(),
		})
	}
	return ops, file, nil
}

// importAssignments applies one file's assignments and re-parses the model under its
// hash; values that make the model invalid are refused with the diagnostics they raised.
func (s *Service) importAssignments(current *CachedModel, ops []dataimport.Assignment, resp *pb.ImportDataResponse) (*CachedModel, string, error) {
	inputs, err := s.importEdits(current, ops)
	if err != nil {
		return nil, "", err
	}
	before := diagnosticErrorCounts(s.modelDiagnostics(current))
	hash, model := s.parseModelKeeping(inputs, current.Mode, true, false)
	after := s.modelDiagnostics(model)
	if problems := addedErrors(before, after); len(problems) > 0 {
		resp.Diagnostics = after
		return nil, "", fmt.Errorf("the imported values make the model invalid: %s", strings.Join(problems, "; "))
	}
	return model, hash, nil
}

// changedDocuments are the documents of edited whose content differs from the
// document at the same place in the model before the import.
func changedDocuments(before, edited *CachedModel) []*pb.EditedDocument {
	var docs []*pb.EditedDocument
	for i, doc := range edited.Documents {
		if i < len(before.Documents) && string(doc.Source.Bytes()) == string(before.Documents[i].Source.Bytes()) {
			continue
		}
		docs = append(docs, &pb.EditedDocument{Name: doc.Source.Name(), Content: string(doc.Source.Bytes())})
	}
	return docs
}

// importRefused answers an import the model refused: nothing was changed, so
// the files imported before the refused one are not reported either.
func importRefused(resp *pb.ImportDataResponse, err error) *pb.ImportDataResponse {
	resp.Error = err.Error()
	resp.Files, resp.Documents, resp.Content, resp.ModelHash = nil, nil, "", ""
	return resp
}

// importEdits applies one file's assignments, document by document, and
// returns the model's sources with the edited documents' content replaced.
func (s *Service) importEdits(cached *CachedModel, ops []dataimport.Assignment) ([]sourceInput, error) {
	inputs := make([]sourceInput, 0, len(cached.Documents))
	byName := make(map[string]int, len(cached.Documents))
	for i, doc := range cached.Documents {
		byName[doc.Source.Name()] = i
		inputs = append(inputs, sourceInput{name: doc.Source.Name(), content: string(doc.Source.Bytes()), kind: doc.Source.Kind()})
	}
	var order []string
	grouped := make(map[string][]int)
	for i, op := range ops {
		if _, ok := grouped[op.Doc]; !ok {
			order = append(order, op.Doc)
		}
		grouped[op.Doc] = append(grouped[op.Doc], i)
	}
	for _, name := range order {
		doc, err := editedDocument(cached, name)
		if err != nil {
			return nil, err
		}
		group := grouped[name]
		list := make([]edit.Operation, len(group))
		for j, i := range group {
			list[j] = ops[i].Op
		}
		model := s.editModel(cached, doc)
		result, err := edit.Apply(model, list)
		if err != nil {
			return nil, s.importFailure(model, ops, group, err)
		}
		inputs[byName[name]].content = string(result.Content)
		for _, other := range result.Others {
			if i, ok := byName[other.Name]; ok {
				inputs[i].content = string(other.Content)
			}
		}
	}
	return inputs, nil
}

// importFailure locates a refused edit at the input value it came from; an edit
// refused as a whole is retried an operation at a time to find it.
func (s *Service) importFailure(model edit.Model, ops []dataimport.Assignment, group []int, err error) error {
	var e *edit.Error
	if !errors.As(err, &e) {
		return err
	}
	if e.OperationIndex >= 0 && e.OperationIndex < len(group) {
		return fmt.Errorf("%s: %s", ops[group[e.OperationIndex]].Where, e.Message)
	}
	for _, i := range group {
		if _, one := edit.Apply(model, []edit.Operation{ops[i].Op}); one != nil {
			var oneErr *edit.Error
			if errors.As(one, &oneErr) {
				return fmt.Errorf("%s: %s", ops[i].Where, oneErr.Message)
			}
			return fmt.Errorf("%s: %w", ops[i].Where, one)
		}
	}
	return err
}

// diagnosticErrorCounts counts the error diagnostics by message.
func diagnosticErrorCounts(diags []*pb.Diagnostic) map[string]int {
	counts := make(map[string]int)
	for _, d := range diags {
		if strings.EqualFold(d.Severity, "error") {
			counts[d.Message]++
		}
	}
	return counts
}

// addedErrors are the error messages after that before did not have as many of.
func addedErrors(before map[string]int, after []*pb.Diagnostic) []string {
	seen := make(map[string]int)
	var problems []string
	for _, d := range after {
		if !strings.EqualFold(d.Severity, "error") {
			continue
		}
		seen[d.Message]++
		if seen[d.Message] > before[d.Message] {
			problems = append(problems, d.Message)
		}
	}
	return problems
}
