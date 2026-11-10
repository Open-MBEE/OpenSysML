using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The capability names a service advertises, and the names it can advertise. Negotiate on these,
/// never on <see cref="ServerInfo.Version"/>: a version is informational (<c>dev</c> for an
/// unreleased build) and versions of forks are not comparable, while a capability is an exact
/// contract the service either reports or does not. A name this release has no constant for, from
/// a newer service, is advertised like any other.
/// </summary>
public sealed class Capabilities : IEquatable<Capabilities>
{
    /// <summary>Names the extended action-body items and source-end multiplicities carried by add_sequence.</summary>
    public const string ActionBodyStatementAuthoring = "action_body_statement_authoring";

    /// <summary>Names the ExecuteAction trace request field.</summary>
    public const string ActionTrace = "action_trace";

    /// <summary>
    /// Names the capability of the ApplyEdits RPC, which edits a parsed model's own source, preserving
    /// everything the edit did not touch.
    /// </summary>
    public const string ApplyEdits = "apply_edits";

    /// <summary>
    /// Gates source-authoring ApplyEdits operations; dedicated capabilities further gate specialized
    /// authoring operations.
    /// </summary>
    public const string Authoring = "authoring";

    /// <summary>
    /// Names the capability of carrying an Integer beyond int64 as Value.big_int_value,
    /// Quantity.big_int_magnitude and DocumentValue.big_int_value, rather than as an unsupported null. A
    /// service without it reads the arm sent to it as null, so a client must not send one.
    /// </summary>
    public const string BigIntValues = "big_int_values";

    /// <summary>
    /// Names the capability of reporting each application an analysis run made of one of the case's calcs
    /// as a function value — a trade study's evaluation of each alternative — and of keeping what a failed
    /// run computed beside its error.
    /// </summary>
    public const string CaseEvaluations = "case_evaluations";

    /// <summary>
    /// Names the CheckBehavior RPC, which searches every schedule of a behavior for violated properties,
    /// deadlocks, failures and divergent values.
    /// </summary>
    public const string Check = "check";

    /// <summary>Names the ApplyEdits add_comment and add_note operations.</summary>
    public const string CommentAuthoring = "comment_authoring";

    /// <summary>
    /// Names the CompareResults RPC, which runs the configurations a tool's results sidecar indexes and
    /// tables the tool's distributions of their observables beside the runs'.
    /// </summary>
    public const string CompareResults = "compare_results";

    /// <summary>
    /// Names the capability of carrying a complex number as Value.complex, rather than reporting it as an
    /// unsupported null.
    /// </summary>
    public const string ComplexValues = "complex_values";

    /// <summary>Names the ApplyEdits add_connection operation.</summary>
    public const string ConnectionAuthoring = "connection_authoring";

    /// <summary>Names constraint body expressions and asserted constraints.</summary>
    public const string ConstraintBodyAuthoring = "constraint_body_authoring";

    /// <summary>
    /// Names the capability of the Convert RPC, which writes a model back out as SysML notation or RDF
    /// Turtle. The RDF direction is experimental, which the response says per conversion.
    /// </summary>
    public const string Convert = "convert";

    /// <summary>
    /// Names ConvertRequest.include_check_diagnostics: the semantic check's diagnostics, with their
    /// severities, reported alongside the converted output.
    /// </summary>
    public const string ConvertCheckDiagnostics = "convert_check_diagnostics";

    /// <summary>
    /// Names ConvertRequest.compact, omit_derived and keep_derived: api-json written as the compact
    /// document, with an id table and handles, and optionally without the metamodel's derived properties.
    /// </summary>
    public const string ConvertCompact = "convert_compact";

    /// <summary>
    /// Names ConvertRequest.documents, which writes only the named documents of a model and links the rest
    /// by id.
    /// </summary>
    public const string ConvertDocuments = "convert_documents";

    /// <summary>Names ConvertResponse.library_references.</summary>
    public const string ConvertLibraryReferences = "convert_library_references";

    /// <summary>Names ConvertRequest.include_library_stubs.</summary>
    public const string ConvertLibraryStubs = "convert_library_stubs";

    /// <summary>
    /// Names the capability of populating Diagnostic.code, so an empty code is a finding none was assigned
    /// rather than an older service.
    /// </summary>
    public const string DiagnosticCodes = "diagnostic_codes";

    /// <summary>
    /// Names the capability of the RunDocumentQuery RPC, which runs a named document query and answers with
    /// typed rows.
    /// </summary>
    public const string DocumentQuery = "document_query";

    /// <summary>Names the ApplyEdits add_documentation operation and the AddMember doc field.</summary>
    public const string DocumentationAuthoring = "documentation_authoring";

    /// <summary>
    /// Names the capability of editing a model of several documents as one batch, for a request accepting
    /// documents, and of answering each edited document by name in ApplyEditsResponse.documents.
    /// </summary>
    public const string EditDocuments = "edit_documents";

    /// <summary>
    /// Names the capability of the GetElementProperties RPC, which reads the metamodel properties of one element
    /// by their specification names, as the model's API element form spells them.
    /// </summary>
    public const string ElementProperties = "element_properties";

    /// <summary>
    /// Names the ListEngines RPC, the <c>engine</c> field of the verification and sweep requests, and the
    /// <c>engine</c>, <c>strength</c> and <c>bounds</c> fields of their responses and of every Verdict.
    /// </summary>
    public const string Engines = "engines";

    /// <summary>
    /// Says the service was started with -serve-external-engines and runs the manifest engines it names;
    /// ListEngines reports which with <c>served</c>.
    /// </summary>
    public const string EnginesExternal = "engines_external";

    /// <summary>
    /// Names the capability of carrying an enumeration literal as Value.enum_literal, rather than reporting
    /// it as an unsupported null.
    /// </summary>
    public const string EnumValues = "enum_values";

    /// <summary>Names the capability of evaluating an expression against an instantiated subject.</summary>
    public const string EvaluateSubject = "evaluate_subject";

    /// <summary>
    /// Names the capability of the ExportGraphs RPC, which exports the lowered graph of an action or state
    /// machine as <c>graphs:1</c> JSON.
    /// </summary>
    public const string ExportGraphs = "export_graphs";

    /// <summary>
    /// Names the capability of populating Instance.feature_values, which replaced the pre-0.1.0
    /// Instance.slots.
    /// </summary>
    public const string FeatureValues = "feature_values";

    /// <summary>
    /// Names the capability of populating final_time on ExecuteActionResponse and ExecuteStateResponse, the
    /// run's simulation clock when it ended. Without it the field is 0 whatever the run waited on.
    /// </summary>
    public const string FinalTime = "final_time";

    /// <summary>
    /// Names the capability of carrying a calc held as a value as Value.function, named by its declaration,
    /// rather than reporting it as an unsupported null.
    /// </summary>
    public const string FunctionValues = "function_values";

    /// <summary>
    /// Names the ApplyEdits add_member operation with no kind: an implicit directed usage (<c>in x :
    /// T;</c>).
    /// </summary>
    public const string ImplicitParameters = "implicit_parameters";

    /// <summary>Names the ApplyEdits add_import operation.</summary>
    public const string ImportAuthoring = "import_authoring";

    /// <summary>
    /// Names the ImportData RPC, which sets the feature values a CSV, TSV, JSON or JSON Lines file assigns,
    /// one row per element.
    /// </summary>
    public const string ImportData = "import_data";

    /// <summary>
    /// Names the capability of carrying the unbounded value <c>*</c> as Value.infinity, rather than
    /// reporting it as an unsupported null.
    /// </summary>
    public const string InfinityValue = "infinity_value";

    /// <summary>Names explicit language selection for inline content.</summary>
    public const string InlineLanguage = "inline_language";

    /// <summary>
    /// Names the capability of the InstancePaths RPC, which lists every root-relative path to one object the model's
    /// population holds, and of populating Instance.reached_by on every object an instantiation built.
    /// </summary>
    public const string InstancePaths = "instance_paths";

    /// <summary>
    /// Names the capability of carrying a bare measurement reference as Value.measurement_ref, rather than
    /// reporting it as an unsupported null. Distinct from structured_values, which predates the arm.
    /// </summary>
    public const string MeasurementRefs = "measurement_refs";

    /// <summary>Names the additional AddMember modifiers and kinds.</summary>
    public const string MemberModifiers = "member_modifiers";

    /// <summary>Names metadata usages and metadata prefixes.</summary>
    public const string MetadataAuthoring = "metadata_authoring";

    /// <summary>Names edits that add metadata to an existing declaration.</summary>
    public const string MetadataPrefixAuthoring = "metadata_prefix_authoring";

    /// <summary>
    /// Names the capability of carrying an element reflected on as an instance of its metaclass (<c>x meta
    /// T</c>, the last element of <c>x.metadata</c>) as Value.metaobject, rather than as an unsupported
    /// null.
    /// </summary>
    public const string MetaobjectValues = "metaobject_values";

    /// <summary>
    /// Names the capability of the Migrate RPC, which migrates a SysML v1 model to v2 and accounts for
    /// every element — mapped, approximated, unmapped or skipped — in its report. Migration is
    /// experimental, which the response says.
    /// </summary>
    public const string Migrate = "migrate";

    /// <summary>
    /// Names the capability of reporting what a run's values stand only up to: <c>order_dependent</c> on
    /// FeatureValue, ExecuteActionResponse and ExecuteStateResponse, and <c>order_dependent</c> and
    /// <c>independent</c> on a choice-point Diagnostic. Without it the fields are empty and a value prints
    /// as fixed whatever order the run took.
    /// </summary>
    public const string OrderDependence = "order_dependence";

    /// <summary>Names the capability of evaluating OSLC Query text.</summary>
    public const string OslcQuery = "oslc_query";

    /// <summary>
    /// Names the ParseSources RPC, which parses several documents as one model so a name one declares
    /// resolves in another.
    /// </summary>
    public const string ParseSources = "parse_sources";

    /// <summary>
    /// Names ParseSourcesRequest.base_model_hash and ParseSourcesResponse.affected: the documents whose
    /// results may differ from the base model's, so a client re-reads only those after an edit.
    /// </summary>
    public const string ParseSourcesAffected = "parse_sources_affected";

    /// <summary>
    /// Names the <c>performer_symbol_id</c> field of ExecuteActionRequest and ExecuteStateRequest; a
    /// service without it runs outside any object, so clients must not send it.
    /// </summary>
    public const string Performer = "performer";

    /// <summary>
    /// Names the capability of the Query RPC, which evaluates a SysML v2 API &amp; Services Query over a
    /// parsed model.
    /// </summary>
    public const string Query = "query";

    /// <summary>
    /// Names the capability of Query's order_by field, which sorts the matched elements; a service without
    /// it would ignore the field.
    /// </summary>
    public const string QueryOrderBy = "query_order_by";

    /// <summary>
    /// Names the capability of carrying an exact Rational that is no Integer as Value.rational_value,
    /// Quantity.rational_magnitude and DocumentValue.rational_value, rather than as an unsupported null. A
    /// service without it reads the arm sent to it as null, so a client must not send one.
    /// </summary>
    public const string RationalValues = "rational_values";

    /// <summary>
    /// Names the RecordRun RPC, which records runs of an analysis case into the model as AnalysisRecords
    /// declarations.
    /// </summary>
    public const string RecordRun = "record_run";

    /// <summary>Names the capability of the RenderDocument RPC, which renders a named document to Markdown.</summary>
    public const string RenderDocument = "render_document";

    /// <summary>
    /// Names the capability of RenderDocumentRequest's form field taking "asciidoc", answered in
    /// RenderDocumentResponse.asciidoc.
    /// </summary>
    public const string RenderDocumentAsciidoc = "render_document_asciidoc";

    /// <summary>
    /// Names RenderDocumentResponse.file_name: the file a rendered set writes the document to, which the
    /// other documents' links to it name. Without it the field is empty.
    /// </summary>
    public const string RenderDocumentFileName = "render_document_file_name";

    /// <summary>
    /// Names the capability of RenderDocumentRequest's form field, which asks RenderDocument for HTML
    /// instead of Markdown.
    /// </summary>
    public const string RenderDocumentHtml = "render_document_html";

    /// <summary>
    /// Names the capability of the RenderView RPC, which renders a declared or targeted pseudo-view as
    /// machine-readable diagram data.
    /// </summary>
    public const string RenderView = "render_view";

    /// <summary>Names the ApplyEdits add_requirement_constraint operation.</summary>
    public const string RequirementConstraintAuthoring = "requirement_constraint_authoring";

    /// <summary>
    /// Names the RunRuns RPC, which runs an action or a Monte Carlo case repeatedly and tables what the
    /// runs observed.
    /// </summary>
    public const string Runs = "runs";

    /// <summary>Names the ApplyEdits add_satisfy operation.</summary>
    public const string SatisfyAuthoring = "satisfy_authoring";

    /// <summary>
    /// Names the <c>schedule</c> field of ExecuteActionRequest, ExecuteStateRequest and RunAnalysisRequest,
    /// which selects the scheduling policy a run resolves its choice points under. A service without it
    /// runs every request under the default, so a client must not send the field to one.
    /// </summary>
    public const string Schedule = "schedule";

    /// <summary>
    /// Names the capability of answering a request under <c>explore[:runs=&lt;n&gt;,depth=&lt;d&gt;]</c>
    /// with <c>outcomes</c> and an <c>exploration</c> status.
    /// </summary>
    public const string ScheduleExplore = "schedule_explore";

    /// <summary>
    /// Names the SelfCheck RPC, which applies the reflective SysMLValidation constraints to every element
    /// of a model.
    /// </summary>
    public const string SelfCheck = "self_check";

    /// <summary>Names the ApplyEdits add_sequence operation.</summary>
    public const string SequenceAuthoring = "sequence_authoring";

    /// <summary>
    /// Names the capability of carrying a unique, unordered collection as Value.set, rather than reporting
    /// it as an unsupported null.
    /// </summary>
    public const string SetValues = "set_values";

    /// <summary>Names state behavior member kinds.</summary>
    public const string StateActionAuthoring = "state_action_authoring";

    /// <summary>Names the ExecuteState trace request field.</summary>
    public const string StateTrace = "state_trace";

    /// <summary>
    /// Names ParseFileRequest.strict_conformance, which asks whether the source is conforming SysML v2.
    /// </summary>
    public const string StrictConformance = "strict_conformance";

    /// <summary>
    /// Names the capability of carrying an array, a vector and a vector quantity as Value.array,
    /// Value.vector and Value.vector_quantity, rather than reporting them as unsupported nulls.
    /// </summary>
    public const string StructuredValues = "structured_values";

    /// <summary>Names the capability of populating SymbolInfo.attributes, rather than always reporting none.</summary>
    public const string SymbolAttributes = "symbol_attributes";

    /// <summary>
    /// Names the capability of populating SymbolInfo.members, every member of a symbol, own and inherited, with its
    /// provenance, and of honouring GetSymbolRequest.include_library_members. Without it only the owned members'
    /// ids, SymbolInfo.child_ids, are reported.
    /// </summary>
    public const string SymbolMembers = "symbol_members";

    /// <summary>
    /// Names the capability of carrying a tensor quantity of any rank as Value.tensor_quantity, rather than
    /// as an unsupported null.
    /// </summary>
    public const string TensorValues = "tensor_values";

    /// <summary>Names the ApplyEdits add_transition operation.</summary>
    public const string TransitionAuthoring = "transition_authoring";

    /// <summary>
    /// Names the capability of populating SymbolInfo.type_info, .multiplicity and .specializations, which
    /// typed code generation requires.
    /// </summary>
    public const string TypeFacts = "type_facts";

    /// <summary>
    /// Names the capability of carrying a result the model leaves open as Value.undetermined rather than as
    /// an unsupported null.
    /// </summary>
    public const string UndeterminedValue = "undetermined_value";

    /// <summary>
    /// Names the capability of reporting a valueless feature of a value type as Value.unset, rather than as
    /// the empty object it materializes.
    /// </summary>
    public const string UnsetValue = "unset_value";

    /// <summary>
    /// Names the <c>notes</c> field of every Verdict: the assumptions and omissions behind a solver's answer,
    /// as the REPL prints them.
    /// </summary>
    public const string VerdictNotes = "verdict_notes";

    /// <summary>
    /// Names the capability of the verification RPCs, which answer the questions the REPL's %constraint,
    /// %requirement, %satisfy and %calc answer.
    /// </summary>
    public const string Verification = "verification";

    /// <summary>
    /// Names the <c>arguments</c> and <c>named_arguments</c> fields of VerifyConstraintRequest and
    /// VerifyRequirementRequest, which bind the checked element's <c>in</c> parameters as
    /// RunAnalysisRequest binds a case's.
    /// </summary>
    public const string VerificationArguments = "verification_arguments";

    /// <summary>Names verification-case objective authoring.</summary>
    public const string VerificationObjectiveAuthoring = "verification_objective_authoring";

    /// <summary>
    /// Names the <c>question</c> field of the verification requests — the solver questions "holds" and
    /// "satisfiable" — and the <c>question</c>, <c>status</c> and <c>witness</c> fields of every Verdict.
    /// </summary>
    public const string VerificationQuestions = "verification_questions";

    /// <summary>
    /// Names the capability of reporting what the body of a verification case answered as
    /// VerificationVerdict, and of running a verification case through RunAnalysis.
    /// </summary>
    public const string VerificationVerdicts = "verification_verdicts";

    private readonly string serviceVersion;

    /// <summary>Creates a capability set.</summary>
    /// <param name="names">The capability names; a repeated name counts once.</param>
    /// <exception cref="ArgumentException">A name is null.</exception>
    public Capabilities(IEnumerable<string> names)
        : this(names, string.Empty)
    {
    }

    /// <summary>Creates the capability set a service of a version advertised.</summary>
    /// <param name="names">The capability names.</param>
    /// <param name="serviceVersion">The version the service reported, which a refusal quotes.</param>
    internal Capabilities(IEnumerable<string> names, string serviceVersion)
    {
        ArgumentNullException.ThrowIfNull(names);
        var copy = names.ToList();
        if (copy.Contains(null!))
        {
            throw new ArgumentException("A capability name is null.", nameof(names));
        }

        Names = copy.ToFrozenSet(StringComparer.Ordinal);
        this.serviceVersion = serviceVersion;
    }

    /// <summary>A set advertising nothing: what a service too old to answer GetServerInfo claims.</summary>
    public static Capabilities None { get; } = new([]);

    /// <summary>Every name advertised, compared ordinally.</summary>
    public IReadOnlySet<string> Names { get; }

    /// <summary>Whether a capability is advertised.</summary>
    /// <param name="capability">The capability's name.</param>
    /// <returns>Whether it is advertised.</returns>
    public bool Has(string capability)
    {
        ArgumentNullException.ThrowIfNull(capability);
        return Names.Contains(capability);
    }

    /// <summary>Requires a capability.</summary>
    /// <param name="capability">The capability's name.</param>
    /// <exception cref="CapabilityException">It is not advertised.</exception>
    public void Require(string capability)
    {
        if (Has(capability))
        {
            return;
        }

        var remedy = UpgradeRemedy(capability);
        var advertised = string.Join(", ", Names.Order(StringComparer.Ordinal));
        var version = serviceVersion.Length == 0 ? "(version unknown)" : serviceVersion;
        throw new CapabilityException(
            capability,
            $"The service does not advertise the {capability} capability, which was required of it; it advertises [{advertised}].\n"
                + $"  service: sysml-grpc {version}\n"
                + $"  fix:     {remedy}",
            remedy);
    }

    /// <summary>How to reach a service that advertises a capability.</summary>
    /// <param name="capability">The capability's name.</param>
    /// <returns>The remedy.</returns>
    public static string UpgradeRemedy(string capability)
    {
        ArgumentNullException.ThrowIfNull(capability);
        return $"connect to a sysml-grpc whose GetServerInfo reports '{capability}': a release that has it, "
            + "or a build of this checkout (make build-grpc)";
    }

    /// <inheritdoc/>
    public bool Equals(Capabilities? other) => other is not null && Names.SetEquals(other.Names);

    /// <inheritdoc/>
    public override bool Equals(object? obj) => Equals(obj as Capabilities);

    /// <inheritdoc/>
    public override int GetHashCode()
    {
        var hash = Names.Count;
        foreach (var name in Names)
        {
            hash ^= StringComparer.Ordinal.GetHashCode(name);
        }

        return hash;
    }

    /// <summary>The advertised names, in ordinal order.</summary>
    /// <returns>The names, as [a, b].</returns>
    public override string ToString() => $"[{string.Join(", ", Names.Order(StringComparer.Ordinal))}]";
}
