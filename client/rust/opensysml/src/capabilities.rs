//! Capability names a service reports in `GetServerInfo`, and the remedy for a missing one.

use crate::binary::Downloader;

/// Static type facts on `SymbolInfo`: `type_info`, `multiplicity` and `specializations`.
pub const CAPABILITY_TYPE_FACTS: &str = "type_facts";
/// The `Convert` RPC, writing a model out as notation, Turtle or API JSON.
pub const CAPABILITY_CONVERT: &str = "convert";
/// The `Migrate` RPC, migrating a SysML v1 model to notation or Turtle with its report.
pub const CAPABILITY_MIGRATE: &str = "migrate";
/// `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`, `ValidateInstance`,
/// `EvaluateCalc`, `RunAnalysis` and `RunSweep`.
pub const CAPABILITY_VERIFICATION: &str = "verification";
/// The `question` field of the verification RPCs and the verdict's `status` and `witness`.
pub const CAPABILITY_VERIFICATION_QUESTIONS: &str = "verification_questions";
/// `arguments` and `named_arguments` on VerifyConstraint and VerifyRequirement.
pub const CAPABILITY_VERIFICATION_ARGUMENTS: &str = "verification_arguments";
/// The `Query` RPC.
pub const CAPABILITY_QUERY: &str = "query";
/// OSLC Query 3.0 parameter text on the `Query` RPC.
pub const CAPABILITY_OSLC_QUERY: &str = "oslc_query";
/// The `RunDocumentQuery` RPC.
pub const CAPABILITY_DOCUMENT_QUERY: &str = "document_query";
/// The `RenderDocument` RPC, rendering Markdown.
pub const CAPABILITY_RENDER_DOCUMENT: &str = "render_document";
/// `RenderDocumentRequest.form`, asking for the HTML page instead of Markdown.
pub const CAPABILITY_RENDER_DOCUMENT_HTML: &str = "render_document_html";
/// `form` on `RenderDocumentRequest` asking for AsciiDoc, answered in `asciidoc`.
pub const CAPABILITY_RENDER_DOCUMENT_ASCIIDOC: &str = "render_document_asciidoc";
/// The `RenderView` RPC, rendering named views as machine-readable diagram data.
pub const CAPABILITY_RENDER_VIEW: &str = "render_view";
/// The `ExportGraphs` RPC, exporting a behavior's lowered graph as `graphs:1` JSON.
pub const CAPABILITY_EXPORT_GRAPHS: &str = "export_graphs";
/// The ImportData RPC, which sets the feature values data files assign.
pub const CAPABILITY_IMPORT_DATA: &str = "import_data";
/// The CompareResults RPC, which compares a tool's stored results with the runs.
pub const CAPABILITY_COMPARE_RESULTS: &str = "compare_results";
/// The `CheckBehavior` RPC, searching every schedule of a behavior for violations and divergent values.
pub const CAPABILITY_CHECK: &str = "check";
/// An enumeration literal as `Value.enum_literal`.
pub const CAPABILITY_ENUM_VALUES: &str = "enum_values";
/// Evaluating an expression against an instantiated subject.
pub const CAPABILITY_EVALUATE_SUBJECT: &str = "evaluate_subject";
/// Populated `SymbolInfo.attributes`.
pub const CAPABILITY_SYMBOL_ATTRIBUTES: &str = "symbol_attributes";
/// A valueless feature of a value type as `Value.unset`.
pub const CAPABILITY_UNSET_VALUE: &str = "unset_value";
/// An object's values as `Instance.feature_values`.
pub const CAPABILITY_FEATURE_VALUES: &str = "feature_values";
/// The `ApplyEdits` RPC.
pub const CAPABILITY_APPLY_EDITS: &str = "apply_edits";
/// Source-preserving add-member and delete authoring operations.
pub const CAPABILITY_AUTHORING: &str = "authoring";
/// The `ApplyEdits` `add_connection` operation.
pub const CAPABILITY_CONNECTION_AUTHORING: &str = "connection_authoring";
/// The `ApplyEdits` `add_satisfy` operation.
pub const CAPABILITY_SATISFY_AUTHORING: &str = "satisfy_authoring";
/// The `ApplyEdits` `add_requirement_constraint` operation.
pub const CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING: &str = "requirement_constraint_authoring";
/// The `ApplyEdits` `add_transition` operation.
pub const CAPABILITY_TRANSITION_AUTHORING: &str = "transition_authoring";
/// The `ApplyEdits` `add_verify` operation and anonymous objectives.
pub const CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING: &str = "verification_objective_authoring";
/// The `ApplyEdits` `add_metadata` operation and metadata prefixes on added members.
pub const CAPABILITY_METADATA_AUTHORING: &str = "metadata_authoring";
/// The `ApplyEdits` `add_metadata_prefix` operation.
pub const CAPABILITY_METADATA_PREFIX_AUTHORING: &str = "metadata_prefix_authoring";
/// The `ApplyEdits` `add_sequence` operation.
pub const CAPABILITY_SEQUENCE_AUTHORING: &str = "sequence_authoring";
/// Capability for `ApplyEdits` action-body statements and succession source multiplicities.
pub const CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING: &str = "action_body_statement_authoring";
/// The `ApplyEdits` `add_import` operation.
pub const CAPABILITY_IMPORT_AUTHORING: &str = "import_authoring";
/// The `ApplyEdits` `add_documentation` operation and `AddMemberEdit.doc`.
pub const CAPABILITY_DOCUMENTATION_AUTHORING: &str = "documentation_authoring";
/// The `ApplyEdits` `add_comment` and `add_note` operations.
pub const CAPABILITY_COMMENT_AUTHORING: &str = "comment_authoring";
/// The additional `AddMemberEdit` modifiers and the `ref` and `return` kinds.
pub const CAPABILITY_MEMBER_MODIFIERS: &str = "member_modifiers";
/// `ApplyEdits` adding a directed usage with no kind keyword (`in x : T;`).
pub const CAPABILITY_IMPLICIT_PARAMETERS: &str = "implicit_parameters";
/// Constraint body expressions and asserted constraints in `ApplyEdits`.
pub const CAPABILITY_CONSTRAINT_BODY_AUTHORING: &str = "constraint_body_authoring";
/// State behavior member kinds (`entry`, `do`, `exit` actions) in `ApplyEdits`.
pub const CAPABILITY_STATE_ACTION_AUTHORING: &str = "state_action_authoring";
/// `ApplyEdits` over a model of several documents, answering each edited document by name.
pub const CAPABILITY_EDIT_DOCUMENTS: &str = "edit_documents";
/// `ParseSources`, parsing several named documents as one model.
pub const CAPABILITY_PARSE_SOURCES: &str = "parse_sources";
/// The declared language of inline content passed to `ParseFile` and `ParseSources`.
pub const CAPABILITY_INLINE_LANGUAGE: &str = "inline_language";
/// `strict_conformance` on the parse requests.
pub const CAPABILITY_STRICT_CONFORMANCE: &str = "strict_conformance";
/// A complex number as `Value.complex`.
pub const CAPABILITY_COMPLEX_VALUES: &str = "complex_values";
/// Arrays, vectors and vector quantities as values.
pub const CAPABILITY_STRUCTURED_VALUES: &str = "structured_values";
/// A bare measurement reference as `Value.measurement_ref`.
pub const CAPABILITY_MEASUREMENT_REFS: &str = "measurement_refs";
/// A calc held as a value, `Value.function`.
pub const CAPABILITY_FUNCTION_VALUES: &str = "function_values";
/// A unique, unordered collection as `Value.set`.
pub const CAPABILITY_SET_VALUES: &str = "set_values";
/// A tensor quantity of any rank as `Value.tensor_quantity`.
pub const CAPABILITY_TENSOR_VALUES: &str = "tensor_values";
/// An element reflected on under its metaclass, `Value.metaobject`.
pub const CAPABILITY_METAOBJECT_VALUES: &str = "metaobject_values";
/// What a verification case's body answered, as `verification_verdicts`.
pub const CAPABILITY_VERIFICATION_VERDICTS: &str = "verification_verdicts";
/// The unbounded value `*` as `Value.infinity`.
pub const CAPABILITY_INFINITY_VALUE: &str = "infinity_value";
/// A populated `Diagnostic.code`.
pub const CAPABILITY_DIAGNOSTIC_CODES: &str = "diagnostic_codes";
/// The `schedule` field of an action, state or analysis run.
pub const CAPABILITY_SCHEDULE: &str = "schedule";
/// Each application an analysis made of a case's calcs, as `evaluations`.
pub const CAPABILITY_CASE_EVALUATIONS: &str = "case_evaluations";
/// The `explore[:runs=<n>,depth=<d>]` schedule and the `explore` engine.
pub const CAPABILITY_SCHEDULE_EXPLORE: &str = "schedule_explore";
/// `performer_symbol_id` on the action and state requests.
pub const CAPABILITY_PERFORMER: &str = "performer";
/// The `trace` field of an `ExecuteStateRequest`.
pub const CAPABILITY_STATE_TRACE: &str = "state_trace";
/// `final_time` populated on an action or state run's response.
pub const CAPABILITY_FINAL_TIME: &str = "final_time";
/// `ListEngines`, the `engine` selection and the standing reported on responses.
pub const CAPABILITY_ENGINES: &str = "engines";
/// The service runs the external engines its manifest names, rather than only listing them.
pub const CAPABILITY_ENGINES_EXTERNAL: &str = "engines_external";
/// A model-level result the model leaves open, as `Value.undetermined`.
pub const CAPABILITY_UNDETERMINED_VALUE: &str = "undetermined_value";
/// An Integer beyond int64 as `Value.big_int_value`, `Quantity.big_int_magnitude` and
/// `DocumentValue.big_int_value`; a service without it reads one sent to it as null.
pub const CAPABILITY_BIG_INT_VALUES: &str = "big_int_values";
/// An exact Rational no f64 holds as `Value.rational_value`, `Quantity.rational_magnitude` and
/// `DocumentValue.rational_value`; a service without it reads one sent to it as null.
pub const CAPABILITY_RATIONAL_VALUES: &str = "rational_values";
/// `ConvertRequest.documents`: a model converted with only the named documents written, the
/// references into the others linked by id.
pub const CAPABILITY_CONVERT_DOCUMENTS: &str = "convert_documents";
/// `ConvertRequest.compact`, `omit_derived` and `keep_derived`: api-json written as the compact
/// document (an id table and handles), optionally without derived properties.
pub const CAPABILITY_CONVERT_COMPACT: &str = "convert_compact";
/// `ConvertRequest.include_check_diagnostics`: the semantic check's diagnostics, each with its
/// own severity, reported alongside the converted output.
pub const CAPABILITY_CONVERT_CHECK_DIAGNOSTICS: &str = "convert_check_diagnostics";
/// `ParseSourcesRequest.base_model_hash` and `ParseSourcesResponse.affected`: the documents whose
/// results may differ from the base model's, so a client re-reads only those after an edit.
pub const CAPABILITY_PARSE_SOURCES_AFFECTED: &str = "parse_sources_affected";
/// `RenderDocumentResponse.file_name`: the file a rendered set writes the document to, which the
/// other documents' links to it name.
pub const CAPABILITY_RENDER_DOCUMENT_FILE_NAME: &str = "render_document_file_name";

/// The remedy for a service lacking `capability`, naming both routes to one that has it.
pub fn upgrade_remedy(capability: &str) -> String {
    let cached = Downloader::from_env()
        .map(|downloader| format!("cached at {}", downloader.binary_path().display()))
        .unwrap_or_else(|_| "cached locally".to_owned());
    format!(
        "run a sysml-grpc whose GetServerInfo reports {capability:?}: set \
         $OPENSYSML_GRPC_VERSION to a release that has it, which replaces the binary \
         {cached} when that is another release, or build one with `make build-grpc` \
         and start it yourself"
    )
}
/// The `RunRuns` RPC: an action or a Monte Carlo case run repeatedly, the runs tabled.
pub const CAPABILITY_RUNS: &str = "runs";
/// The `SelfCheck` RPC: the reflective SysMLValidation check of every element.
pub const CAPABILITY_SELF_CHECK: &str = "self_check";
/// The `RecordRun` RPC: runs of an analysis case recorded into the model.
pub const CAPABILITY_RECORD_RUN: &str = "record_run";

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_remedy_names_the_capability_and_both_routes() {
        let remedy = upgrade_remedy(CAPABILITY_QUERY);
        assert!(remedy.contains("\"query\""), "{remedy}");
        assert!(remedy.contains("$OPENSYSML_GRPC_VERSION"), "{remedy}");
        assert!(remedy.contains("make build-grpc"), "{remedy}");
    }
}
