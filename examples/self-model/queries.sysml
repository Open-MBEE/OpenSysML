// The query path: QVTS — Query, View, Transformation for SysML v2 — as this
// implementation has it. A query is a calc def over the DocumentQueries
// library, compiled to an immutable plan and run step by step; each step
// either answers a value (an empty collection is a value), fails as a path
// error naming the frontier the model stopped at, or fails as a model error
// naming the constraint the model broke. Every surface that runs a query
// shows that same outcome record, and the recorded QVTS examples pin it.
package OpenSysMLQueries {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;
    private import OpenSysMLSurfaces::ConformanceOracle;

    // What travels along the query path, beside the rows of OpenSysMLArtifacts.
    item def QueryPlan;
    item def QueryOutcome;

    // --- The library -------------------------------------------------------

    // One calc def of the library and the plan operation it compiles to.
    abstract part def LibraryQuery {
        attribute calcDef : String;
        attribute operation : String;
    }

    // The non-normative query vocabulary a document or a -run-query uses:
    // every query specializes DocumentQueries::Query. Of its calc defs, the
    // compiler binds the operations below; the base, Distinct, and the two
    // column specs are not operations of a plan.
    part def QueryLibrary :> CodeUnit {
        attribute :>> goPackage = "internal/workspace/libs";
        attribute libraryFile : String =
            "internal/workspace/libs/stdlib/OpenSysML Libraries/DocumentQueries.sysml";
        attribute libraryPackage : String = "DocumentQueries";
        attribute queryBase : String = "DocumentQueries::Query";
        attribute normative : Boolean = false;
        attribute calcDefCount : Integer = 26;
        attribute operationCount : Integer = 22;
        attribute unboundCalcDefs : String = "Query, Distinct, Column, RelatedColumn";

        part ownedElements : LibraryQuery { attribute :>> calcDef = "OwnedElements"; attribute :>> operation = "owned-elements"; }
        part descendants : LibraryQuery { attribute :>> calcDef = "Descendants"; attribute :>> operation = "descendants"; }
        part ancestors : LibraryQuery { attribute :>> calcDef = "Ancestors"; attribute :>> operation = "ancestors"; }
        part named : LibraryQuery { attribute :>> calcDef = "Named"; attribute :>> operation = "named"; }
        part objects : LibraryQuery { attribute :>> calcDef = "Objects"; attribute :>> operation = "objects"; }
        part verdicts : LibraryQuery { attribute :>> calcDef = "Verdicts"; attribute :>> operation = "verdicts"; }
        part states : LibraryQuery { attribute :>> calcDef = "States"; attribute :>> operation = "states"; }
        part inState : LibraryQuery { attribute :>> calcDef = "InState"; attribute :>> operation = "in-state"; }
        part events : LibraryQuery { attribute :>> calcDef = "Events"; attribute :>> operation = "events"; }
        part relatedElements : LibraryQuery { attribute :>> calcDef = "RelatedElements"; attribute :>> operation = "related-elements"; }
        part whereType : LibraryQuery { attribute :>> calcDef = "WhereType"; attribute :>> operation = "where-type"; }
        part whereMetadata : LibraryQuery { attribute :>> calcDef = "WhereMetadata"; attribute :>> operation = "where-metadata"; }
        part whereName : LibraryQuery { attribute :>> calcDef = "WhereName"; attribute :>> operation = "where-name"; }
        part whereText : LibraryQuery { attribute :>> calcDef = "WhereText"; attribute :>> operation = "where-text"; }
        part whereFeature : LibraryQuery { attribute :>> calcDef = "WhereFeature"; attribute :>> operation = "where-feature"; }
        part orderBy : LibraryQuery { attribute :>> calcDef = "OrderBy"; attribute :>> operation = "order-by"; }
        part project : LibraryQuery { attribute :>> calcDef = "Project"; attribute :>> operation = "project"; }
        part whereRelated : LibraryQuery { attribute :>> calcDef = "WhereRelated"; attribute :>> operation = "where-related"; }
        part except : LibraryQuery { attribute :>> calcDef = "Except"; attribute :>> operation = "except"; }
        part union : LibraryQuery { attribute :>> calcDef = "Union"; attribute :>> operation = "union"; }
        part differ : LibraryQuery { attribute :>> calcDef = "Differ"; attribute :>> operation = "differ"; }
        part tree : LibraryQuery { attribute :>> calcDef = "Tree"; attribute :>> operation = "tree"; }
    }

    // --- The compiler and the executor -------------------------------------

    // A query is compiled once into an immutable plan: the library's calc defs
    // become the operations above, and the expressions between them the
    // operations of the plan itself (a parameter, a literal, a column, a lambda).
    part def QueryCompiler :> CodeUnit {
        attribute :>> goPackage = "internal/ir/queryplan";
        attribute immutablePlans : Boolean = true;
        attribute operationCount : Integer = 36;
        attribute libraryOperationCount : Integer = 22;
        out item plan : QueryPlan;
    }

    // The executor runs a plan to ordered rows, or to the outcome that says why
    // it could not: one failing element fails a fan-out, and every failing
    // element stays in the record; only Project and WhereFeature may tolerate.
    part def QueryExecutor :> CodeUnit {
        attribute :>> goPackage = "internal/doc/queryexec";
        attribute orderedRows : Boolean = true;
        attribute stepKindCount : Integer = 10;
        attribute constraintCount : Integer = 5;
        attribute emptyIsAValue : Boolean = true;
        attribute failsClosed : Boolean = true;
        attribute keepsEveryFailingElement : Boolean = true;
        attribute toleratingOperations : String = "Project, WhereFeature";
        in item plan : QueryPlan;
        out item rows : QueryRows;
        out item outcome : QueryOutcome;
    }

    // The ten kinds of step an outcome is reported at.
    enum def StepKind {
        Named;
        Owned;
        Descendants;
        Feature;
        Value;
        OfType;
        Related;
        WhereFeature;
        Project;
        Differ;
    }

    // The three outcomes of a step; the record carries the second and third.
    enum def OutcomeKind {
        value;
        pathError;
        modelError;
    }

    // The constraints a model error reports a model as breaking.
    enum def ModelConstraint {
        ambiguousName;
        multiplicity;
        typing;
        unit;
        duplicateKey;
    }

    // The outcome record, field for field.
    attribute def Step {
        attribute kind : StepKind;
        attribute argument : String;
    }

    // Where a path error stopped: the element reached and what it offers.
    attribute def Frontier {
        attribute 'at' : String;
        attribute members : String[0..*] ordered;
        attribute features : String[0..*] ordered;
        attribute kinds : String[0..*] ordered;
        attribute nearest : String[0..*] ordered;
    }

    attribute def Failure {
        attribute step : Step;
        attribute expected : String;
        attribute frontier : Frontier;
    }

    attribute def Finding {
        attribute step : Step;
        attribute element : String;
        attribute 'constraint' : ModelConstraint;
        attribute observed : String;
    }

    attribute def PathError {
        attribute failures : Failure[1..*] ordered;
    }

    attribute def ModelError {
        attribute findings : Finding[1..*] ordered;
        attribute pathFailures : Failure[0..*] ordered;
    }

    attribute def Outcome {
        attribute path : PathError[0..1];
        attribute model : ModelError[0..1];
    }

    // One step evaluated. A model error outranks a path error from the same
    // fan-out; a tolerated miss is a value (an empty cell, a dropped row).
    state def StepOutcome {
        attribute missing : Boolean = false;
        attribute violated : Boolean = false;
        attribute tolerated : Boolean = false;

        entry; then evaluating;

        state evaluating;
        state value;
        state pathError;
        state modelError;

        transition first evaluating if violated then modelError;
        transition first evaluating if missing and not violated and not tolerated then pathError;
        transition first evaluating if missing and tolerated and not violated then value;
        transition first evaluating if not missing and not violated then value;
    }

    // --- The surfaces a query is run from ----------------------------------

    // Each surface runs a query and shows its outcome in its own form.
    abstract part def QuerySurface :> CodeUnit {
        attribute command : String;
        attribute outcomeShown : String;
        in item rows : QueryRows;
        in item outcome : QueryOutcome;
    }

    // `sysml ... -run-query Pkg::Q`: rows on the console, the outcome at
    // checks[].outcome under -json, and exit status 2 for a query that could not run.
    part def CliQuerySurface :> QuerySurface {
        attribute :>> goPackage = "cmd/sysml";
        attribute :>> command = "-run-query";
        attribute :>> outcomeShown = "checks[].outcome";
        attribute jsonFlag : String = "-json";
        attribute failureExitStatus : Integer = 2;
    }

    part def ReplQuerySurface :> QuerySurface {
        attribute :>> goPackage = "internal/frontend/repl";
        attribute :>> command = "%run-query";
        attribute :>> outcomeShown = "outcome lines";
    }

    // The RPC answers rows, or fails the call with the outcome as an error detail.
    part def ServiceQuerySurface :> QuerySurface {
        attribute :>> goPackage = "internal/frontend/grpc";
        attribute :>> command = "RunDocumentQuery";
        attribute :>> outcomeShown = "QueryOutcome";
        attribute schema : String = "api/proto/sysml.proto";
    }

    // A document table is a query too; one that fails fails the render.
    part def TableQuerySurface :> QuerySurface {
        attribute :>> goPackage = "internal/ir/docplan";
        attribute :>> command = "Table";
        attribute :>> outcomeShown = "render error";
    }

    // --- The conformance oracle --------------------------------------------

    // The QVTS worked examples, each command's output and exit status recorded
    // and pinned against the guide; the reference is the QVTS project itself.
    #Oracle part def QvtsOracle :> ConformanceOracle {
        attribute :>> goPackage = "tests/qvts";
        attribute :>> reference = "Open-MBEE/qvts";
        attribute :>> baseline = "examples/qvts/expected";
        attribute :>> gating = true;
        attribute examples : String = "examples/qvts";
        attribute guide : String = "docs/guide/qvts.md";
        attribute designNote : String = "docs/internals/design/query-outcomes.md";
        attribute recordingCount : Integer = 24;
    }

    // --- The whole path ----------------------------------------------------

    part def QueryPath {
        part 'library' : QueryLibrary;
        part compiler : QueryCompiler;
        part executor : QueryExecutor;
        part cli : CliQuerySurface;
        part repl : ReplQuerySurface;
        part service : ServiceQuerySurface;
        part tables : TableQuerySurface;
        part oracle : QvtsOracle;

        flow of QueryPlan from compiler.plan to executor.plan;
        flow of QueryRows from executor.rows to cli.rows;
        flow of QueryRows from executor.rows to repl.rows;
        flow of QueryRows from executor.rows to service.rows;
        flow of QueryRows from executor.rows to tables.rows;
        flow of QueryOutcome from executor.outcome to cli.outcome;
        flow of QueryOutcome from executor.outcome to repl.outcome;
        flow of QueryOutcome from executor.outcome to service.outcome;
        flow of QueryOutcome from executor.outcome to tables.outcome;
    }

    part queryPath : QueryPath;

    // --- The invariant -----------------------------------------------------

    #Invariant requirement def OutcomesAreDistinct {
        doc /* An answer, a path error and a model error are never confused: an
             * empty collection is answered only as a value, never for a name
             * or feature the model lacks or a constraint it breaks; one failing
             * element fails its fan-out, and every failing element is kept. */
        subject executor : QueryExecutor;
        require constraint {
            executor.emptyIsAValue and executor.failsClosed
                and executor.keepsEveryFailingElement
        }
    }

    requirement outcomesAreDistinct : OutcomesAreDistinct {
        subject executor = queryPath.executor;
    }

    satisfy requirement queryOutcomesAreDistinct : OutcomesAreDistinct by queryPath.executor;

    // --- What the toolchain satisfies --------------------------------------
    // The invariants of the other packages, each asserted of the part that is
    // its subject, so that Verdicts tables the whole set from the model.

    satisfy requirement treeIsImmutable : OpenSysMLInvariants::ImmutableSyntaxTree by OpenSysMLSurfaces::opensysml.pipeline.tree;
    satisfy requirement parserRecovers : OpenSysMLInvariants::ParserNeverFails by OpenSysMLSurfaces::opensysml.pipeline.parser;
    satisfy requirement resolutionIsLazy : OpenSysMLInvariants::LazyMemoizedSemantics by OpenSysMLSurfaces::opensysml.pipeline.resolver;
    satisfy requirement tiersAreGated : OpenSysMLInvariants::TieredGating by OpenSysMLSurfaces::opensysml.pipeline.passes;
    satisfy requirement loweringIsLossless : OpenSysMLInvariants::LosslessLowering by OpenSysMLSurfaces::opensysml.pipeline.lowering;
    satisfy requirement executionIsBounded : OpenSysMLInvariants::BoundedExecution by OpenSysMLSurfaces::opensysml.pipeline.runtime;
    satisfy requirement libraryIsClean : OpenSysMLInvariants::LibraryConformance by OpenSysMLSurfaces::opensysml.pipeline.stdlib;
    satisfy requirement snapshotIsDerived : OpenSysMLInvariants::DerivedLibrarySnapshot by OpenSysMLSurfaces::opensysml.pipeline.stdlib;
    satisfy requirement evaluatorIsReference : OpenSysMLInvariants::ReferenceEvaluator by OpenSysMLSurfaces::opensysml.pipeline.runtime.evaluator;
    satisfy requirement exportRoundTrips : OpenSysMLInvariants::RoundTrippableExport by OpenSysMLSurfaces::opensysml.exporter;
    satisfy requirement questionsHaveOneContract : OpenSysMLInvariants::OneAnalysisContract by OpenSysMLSurfaces::opensysml.pipeline.engines;
    satisfy requirement evidenceIsHonest : OpenSysMLInvariants::HonestEvidence by OpenSysMLSurfaces::opensysml.pipeline.engines;
    satisfy requirement runsAreIsolated : OpenSysMLInvariants::IsolatedParallelRuns by OpenSysMLSurfaces::opensysml.pipeline.engines;
    satisfy requirement snapshotsAreRunState : OpenSysMLInvariants::SnapshotsAreRunState by OpenSysMLSurfaces::opensysml.pipeline.runtime;
    satisfy requirement documentsAreTraceable : OpenSysMLSurfaces::DocumentsCarryTheirProvenance by OpenSysMLSurfaces::opensysml.documents;
    satisfy requirement viewsAreHonest : OpenSysMLSurfaces::ViewsSayWhatTheyCannotRender by OpenSysMLSurfaces::opensysml.documents.views;
}
