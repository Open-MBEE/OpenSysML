// The analysis pipeline of OpenSysML itself: what each stage consumes and
// produces, and the channels the artifacts travel over.
package OpenSysMLArtifacts {
    private import ScalarValues::*;

    // What moves between stages.
    item def SourceText;
    item def SourceSpan;
    item def TokenStream;
    item def SyntaxTree;
    item def SymbolIndex;
    item def LibrarySnapshot;
    item def SideTable;
    item def ResolutionTable :> SideTable;
    item def TypeTable :> SideTable;
    item def Diagnostic;
    item def QuickFix;
    item def SemanticToken;
    item def SourceEdit;
    item def ActionGraph;
    item def StateGraph;
    item def ExecutionTrace;
    item def InstanceGraph;
    item def RdfGraph;
    item def QueryRows;
    item def DocumentPlan;
    item def DocumentTree;
    item def AnalysisQuestion;
    item def AnalysisPlan;
    item def AnalysisResult;
    item def RunSnapshot;

    port def ArtifactPort;
    port def DiagnosticPort;
    port def RequestPort;

    connection def ArtifactChannel {
        end producer : ArtifactPort;
        end consumer : ArtifactPort;
    }

    connection def DiagnosticChannel {
        end reporter : DiagnosticPort;
        end collector : DiagnosticPort;
    }

    connection def RequestChannel {
        end asker : RequestPort;
        end answerer : RequestPort;
    }

    // Where a unit sits in the architecture, for filtered views.
    metadata def Frontend;
    metadata def SemanticEngine;
    metadata def Backend;
    metadata def Analysis;
    metadata def Surface;
    metadata def Oracle;
    metadata def Invariant;

    // Every modelled unit names the Go package that implements it.
    abstract part def CodeUnit {
        attribute goPackage : String;
        attribute lazy : Boolean default = false;
        attribute memoized : Boolean default = false;
    }

    abstract part def Stage :> CodeUnit {
        port diagnostics : DiagnosticPort;
    }
}

package OpenSysMLPipeline {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;

    // --- Front end -------------------------------------------------------

    #Frontend part def SourceStore :> Stage {
        attribute :>> goPackage = "internal/core/source";
        attribute tracksLineIndex : Boolean = true;
        out item text : SourceText;
        port textOut : ArtifactPort;
    }

    #Frontend part def Lexer :> Stage {
        attribute :>> goPackage = "internal/core/lexer";
        attribute keywordCount : Integer = 172;
        attribute tracksTrivia : Boolean = true;
        in item text : SourceText;
        out item tokens : TokenStream;
        port textIn : ArtifactPort;
        port tokensOut : ArtifactPort;
    }

    // Recursive descent with error recovery: a tree always comes back.
    #Frontend part def Parser :> Stage {
        attribute :>> goPackage = "internal/core/parser";
        attribute neverPanics : Boolean = true;
        attribute emitsErrorNodes : Boolean = true;
        in item tokens : TokenStream;
        out item tree : SyntaxTree;
        out item fixes : QuickFix;
        port tokensIn : ArtifactPort;
        port treeOut : ArtifactPort;
    }

    // Syntax only, immutable after parse.
    #Frontend part def SyntaxTreeStore :> Stage {
        attribute :>> goPackage = "internal/core/ast";
        attribute immutableAfterParse : Boolean = true;
        attribute carriesSemantics : Boolean = false;
        in item tree : SyntaxTree;
        port treeIn : ArtifactPort;
        port treeOut : ArtifactPort;
    }

    #Frontend part def SymbolBuilder :> Stage {
        attribute :>> goPackage = "internal/core/symbols";
        attribute buildsScopeTree : Boolean = true;
        in item tree : SyntaxTree;
        out item index : SymbolIndex;
        port treeIn : ArtifactPort;
        port indexOut : ArtifactPort;
    }

    // --- Semantic engine -------------------------------------------------

    #SemanticEngine part def Resolver :> Stage {
        attribute :>> goPackage = "internal/core/resolve";
        attribute :>> lazy = true;
        attribute :>> memoized = true;
        attribute expandsWildcardImports : Boolean = true;
        in item index : SymbolIndex;
        out item bindings : ResolutionTable;
        port indexIn : ArtifactPort;
        port bindingsOut : ArtifactPort;
    }

    // Supertypes, conformance, members, multiplicity, constant folding.
    #SemanticEngine part def SemanticModel :> Stage {
        attribute :>> goPackage = "internal/core/semantics";
        attribute :>> memoized = true;
        in item bindings : ResolutionTable;
        out item types : TypeTable;
        port bindingsIn : ArtifactPort;
        port typesOut : ArtifactPort;
    }

    // A validation pass: goType names its Go type in the passes package, level
    // the tier it runs at. An element-scoped pass gates itself per subject
    // through DownstreamOfFailure instead of being skipped with its tier.
    abstract part def ValidationPass :> CodeUnit {
        attribute :>> goPackage = "internal/core/passes";
        attribute goType : String;
        attribute level : String;
        attribute elementScoped : Boolean default = false;
        out item findings : Diagnostic;
    }

    part def SyntaxCheck :> ValidationPass {
        attribute :>> level = "syntax";
    }

    part def NameResolutionCheck :> ValidationPass {
        attribute :>> level = "name-resolution";
    }

    part def TypeCheck :> ValidationPass {
        attribute :>> level = "type";
    }

    part def ConstraintCheck :> ValidationPass {
        attribute :>> level = "constraint";
    }

    // Every pass DefaultRegistry registers, in registration order. Run sorts
    // them stably by level; a blocking finding skips the document-scoped passes
    // of the tiers above it and leaves the element-scoped ones to gate themselves.
    #SemanticEngine part def PassRegistry :> Stage {
        attribute :>> goPackage = "internal/core/passes";
        attribute tierCount : Integer = 4;
        attribute passCount : Integer = 56;
        attribute sortsByLevel : Boolean = true;
        attribute skipsDocumentScopedAboveFailure : Boolean = true;

        part syntax : SyntaxCheck { attribute :>> goType = "SyntaxPass"; }
        // Grammar violations the parser reads through as recoverable warnings.
        part grammarViolations : SyntaxCheck { attribute :>> goType = "GrammarViolationPass"; }
        // Non-standard notation is a syntax-level warning, not a constraint.
        part notation : SyntaxCheck { attribute :>> goType = "NonstandardNotationPass"; }
        part nameResolution : NameResolutionCheck { attribute :>> goType = "NameResolutionPass"; }
        part stateTransitions : NameResolutionCheck { attribute :>> goType = "StateTransitionPass"; }
        part actionEndpoints : NameResolutionCheck {
            attribute :>> goType = "ActionEndpointPass";
            attribute :>> elementScoped = true;
        }
        part typeCheck : TypeCheck { attribute :>> goType = "TypeCheckPass"; }
        part transitionGuards : TypeCheck {
            attribute :>> goType = "TransitionGuardPass";
            attribute :>> elementScoped = true;
        }
        part triggerArguments : TypeCheck {
            attribute :>> goType = "TriggerArgumentPass";
            attribute :>> elementScoped = true;
        }
        part sendActions : TypeCheck {
            attribute :>> goType = "SendActionPass";
            attribute :>> elementScoped = true;
        }
        part kermlSubsettingMetaclass : TypeCheck {
            attribute :>> goType = "KerMLSubsettingMetaclassPass";
            attribute :>> elementScoped = true;
        }
        part redefinitionDirection : TypeCheck { attribute :>> goType = "RedefinitionDirectionPass"; }
        part elementFilters : TypeCheck {
            attribute :>> goType = "ElementFilterPass";
            attribute :>> elementScoped = true;
        }
        part constraints : ConstraintCheck { attribute :>> goType = "ConstraintPass"; }
        part documentQueries : ConstraintCheck {
            attribute :>> goType = "DocumentQueryPass";
            attribute :>> elementScoped = true;
        }
        part documentPlans : ConstraintCheck {
            attribute :>> goType = "DocumentPlanPass";
            attribute :>> elementScoped = true;
        }
        part typeRelationships : ConstraintCheck { attribute :>> goType = "TypeRelationshipsPass"; }
        part conjugatedSpecialization : TypeCheck { attribute :>> goType = "W11EConjugatedSpecializationPass"; }
        part implicitBase : ConstraintCheck { attribute :>> goType = "ImplicitBasePass"; }
        part multiplicityBounds : ConstraintCheck { attribute :>> goType = "MultiplicityBoundsPass"; }
        part referenceSubsetting : ConstraintCheck { attribute :>> goType = "ReferenceSubsettingPass"; }
        part topLevelImports : ConstraintCheck { attribute :>> goType = "TopLevelImportPass"; }
        part associationEndTypes : ConstraintCheck { attribute :>> goType = "AssociationEndTypesPass"; }
        part variableFeatures : ConstraintCheck {
            attribute :>> goType = "VariableFeaturePass";
            attribute :>> elementScoped = true;
        }
        part endFeatures : ConstraintCheck {
            attribute :>> goType = "EndFeaturePass";
            attribute :>> elementScoped = true;
        }
        part assignmentReferents : ConstraintCheck {
            attribute :>> goType = "AssignmentReferentPass";
            attribute :>> elementScoped = true;
        }
        part resultExpressions : ConstraintCheck {
            attribute :>> goType = "ResultExpressionPass";
            attribute :>> elementScoped = true;
        }
        part featureReferences : ConstraintCheck { attribute :>> goType = "FeatureReferencePass"; }
        part metadataTypes : TypeCheck {
            attribute :>> goType = "MetadataTypePass";
            attribute :>> elementScoped = true;
        }
        part metadataAnnotations : TypeCheck { attribute :>> goType = "MetadataAnnotationPass"; }
        part annotationOwnership : ConstraintCheck {
            attribute :>> goType = "AnnotationOwnershipPass";
            attribute :>> elementScoped = true;
        }
        part occurrenceTyping : TypeCheck { attribute :>> goType = "W8DOccurrenceTypingPass"; }
        part connectorFeaturing : ConstraintCheck { attribute :>> goType = "W8DConnectorFeaturingPass"; }
        part flowEnds : ConstraintCheck { attribute :>> goType = "W8DFlowEndPass"; }
        part variability : ConstraintCheck { attribute :>> goType = "W8DVariabilityPass"; }
        part variantOwners : ConstraintCheck {
            attribute :>> goType = "VariantOwnerPass";
            attribute :>> elementScoped = true;
        }
        part relatedElements : ConstraintCheck { attribute :>> goType = "W10BRelatedElementsPass"; }
        part individualTyping : TypeCheck { attribute :>> goType = "W10BIndividualTypingPass"; }
        part endKinds : TypeCheck { attribute :>> goType = "W10BEndKindPass"; }
        part structuralElements : TypeCheck { attribute :>> goType = "W10BStructuralPass"; }
        part portionOwners : ConstraintCheck { attribute :>> goType = "W10BPortionOwnerPass"; }
        part verificationCases : ConstraintCheck { attribute :>> goType = "W8DVerificationPass"; }
        part viewRenderings : ConstraintCheck { attribute :>> goType = "W8DViewRenderingPass"; }
        part metadataUsages : TypeCheck { attribute :>> goType = "W8DMetadataUsagePass"; }
        part identityMetadata : ConstraintCheck { attribute :>> goType = "IdentityMetadataPass"; }
        part diagramLayout : ConstraintCheck { attribute :>> goType = "DiagramLayoutPass"; }
        part redefinitionConformance : ConstraintCheck { attribute :>> goType = "RedefinitionConformancePass"; }
        part shortNameDistinguishability : NameResolutionCheck { attribute :>> goType = "W9CShortNameDistinguishabilityPass"; }
        part userStandardLibrary : NameResolutionCheck { attribute :>> goType = "W9CUserStandardLibraryPass"; }
        part inheritedNameConflicts : TypeCheck { attribute :>> goType = "W9CInheritedNameConflictPass"; }
        part boundFeatureTypes : TypeCheck { attribute :>> goType = "W9CBoundFeatureTypesPass"; }
        part usageTyping : TypeCheck { attribute :>> goType = "W11AUsageTypingPass"; }
        part kermlSpecialization : TypeCheck { attribute :>> goType = "W11AKerMLSpecializationPass"; }
        part controlNodeSuccessions : ConstraintCheck {
            attribute :>> goType = "ControlNodeSuccessionPass";
            attribute :>> elementScoped = true;
        }
        part oosemMethod : ConstraintCheck { attribute :>> goType = "OOSEMMethodPass"; }
        part mosa : ConstraintCheck { attribute :>> goType = "MOSAPass"; }

        in item typed : TypeTable;
        out item findings : Diagnostic;
        port typesIn : ArtifactPort;
        port findingsOut : ArtifactPort;
    }

    // Owns the document set, the global index and the diagnostic cache.
    #SemanticEngine part def Workspace :> Stage {
        attribute :>> goPackage = "internal/core/model";
        attribute oneSessionPerWorkspace : Boolean = true;
        port documents : ArtifactPort;
        port requests : RequestPort;
    }

    // The compact binary encoding the library snapshot is written in.
    part def SnapshotCodec :> CodeUnit {
        attribute :>> goPackage = "internal/core/pack";
    }

    // Writes syntax trees into a snapshot and reads them back, node for node.
    part def TreeCodec :> CodeUnit {
        attribute :>> goPackage = "internal/core/ast/astcodec";
        in item tree : SyntaxTree;
    }

    // go generate writes the snapshot; the check target is what CI runs on it.
    part def SnapshotGenerator :> CodeUnit {
        attribute :>> goPackage = "internal/core/libs/gensnapshot";
        attribute makeTarget : String = "stdlib-snapshot";
        attribute checkTarget : String = "stdlib-snapshot-check";
        out item snapshotBlob : LibrarySnapshot;
    }

    // Bundled standard library. Its frozen index is decoded from an embedded
    // snapshot when that matches the files, and parsed from the files otherwise.
    #SemanticEngine part def StandardLibrary :> Stage {
        attribute :>> goPackage = "internal/core/libs";
        attribute bundledFileCount : Integer = 100;
        attribute cached : Boolean = true;
        attribute snapshotFile : String = "internal/core/libs/stdlib.snapshot";
        attribute snapshotEmbedded : Boolean = true;
        attribute snapshotChecksummed : Boolean = true;
        attribute fallsBackToParsing : Boolean = true;
        attribute overrideEnvVar : String = "OPENSYSML_LIBRARY_PATH";

        part encoding : SnapshotCodec;
        part trees : TreeCodec;
        part generator : SnapshotGenerator;

        in item snapshotBlob : LibrarySnapshot;
        out item index : SymbolIndex;
        port indexOut : ArtifactPort;
    }

    // --- Back end --------------------------------------------------------

    // Lossless: guards, triggers, effects and pseudostate edges all survive.
    #Backend part def Lowering :> Stage {
        attribute :>> goPackage = "internal/core/lower";
        attribute lossless : Boolean = true;
        in item types : TypeTable;
        out item actions : ActionGraph;
        out item states : StateGraph;
        port typesIn : ArtifactPort;
        port irOut : ArtifactPort;
    }

    // Succession-ordered token flow over the lowered action graph.
    part def ActionExecutor :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute detectsDeadlock : Boolean = true;
        attribute recordsTrace : Boolean = true;
        in item graph : ActionGraph;
        out item trace : ExecutionTrace;
    }

    // Event-driven, hierarchical, with orthogonal regions.
    part def StateExecutor :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute runToCompletion : Boolean = true;
        attribute defersUnhandledEvents : Boolean = true;
        in item graph : StateGraph;
        out item trace : ExecutionTrace;
    }

    // Scalar calc bodies — expressions and statements alike — compiled once per
    // model into closures held in a side table; the evaluator stays the reference.
    part def CalcCompiler :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute :>> memoized = true;
        attribute overrideEnvVar : String = "OPENSYSML_CALC_COMPILE";
        attribute fallsBackToEvaluator : Boolean = true;
    }

    // Literals and invocation targets are memoized per model, shared by its runs,
    // in side tables keyed by syntax node; the tree itself carries nothing.
    part def Evaluator :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute :>> memoized = true;
        attribute supportsCalcInvocation : Boolean = true;
        attribute compilesPureCalcs : Boolean = true;

        part compiled : CalcCompiler;
    }

    part def InstanceModel :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute lazyFeatureValues : Boolean = true;
        out item instances : InstanceGraph;
    }

    // A run's state between two steps, restored as often as asked; the model-derived
    // state the run is over is no part of it, and a snapshot mid-step is refused.
    part def SnapshotStore :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute capturesRunState : Boolean = true;
        attribute excludesModelState : Boolean = true;
        attribute refusesMidStep : String = "snapshot inside a step";
        attribute refusesPausedBody : String = "snapshot of a body paused mid-statement";
        out item mark : RunSnapshot;
    }

    // Every linearization of a behavior under an exploring policy, worked as a
    // queue of prefixes in plan order by however many jobs; the outcomes are the same at any count.
    part def ExplorationQueue :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute defaultRuns : Integer = 1024;
        attribute defaultDepth : Integer = 64;
        attribute planOrdered : Boolean = true;
        attribute independentOfJobs : Boolean = true;
        attribute freshContextPerRun : Boolean = true;
    }

    // One bound on one run. Each is exhausted on its own, with a typed error
    // that names it, and each is overridable through its environment variable.
    part def Budget {
        attribute counts : String;
        attribute envVar : String;
        attribute defaultBound : Integer;
        attribute exhaustion : String;
    }

    // Model-derived state (resolver, semantic model, memo tables) is built once per
    // worker and shared by its runs; run-derived state lives in a context of the run's own.
    #Backend part def Runtime :> Stage {
        attribute :>> goPackage = "internal/core/runtime";
        attribute stepBudgeted : Boolean = true;
        attribute budgetCount : Integer = 7;
        attribute splitsModelFromRun : Boolean = true;

        part instances : InstanceModel;
        part evaluator : Evaluator;
        part actions : ActionExecutor;
        part states : StateExecutor;
        part marks : SnapshotStore;
        part exploration : ExplorationQueue;

        part evaluationSteps : Budget {
            attribute :>> counts = "expression evaluation steps";
            attribute :>> envVar = "OPENSYSML_MAX_STEPS";
            attribute :>> defaultBound = 10000000;
            attribute :>> exhaustion = "evaluation step limit exceeded";
        }
        part actionSteps : Budget {
            attribute :>> counts = "action executor steps";
            attribute :>> envVar = "OPENSYSML_MAX_ACTION_STEPS";
            attribute :>> defaultBound = 1000000;
            attribute :>> exhaustion = "action step limit exceeded";
        }
        part stateEvents : Budget {
            attribute :>> counts = "state machine events";
            attribute :>> envVar = "OPENSYSML_MAX_EVENTS";
            attribute :>> defaultBound = 1000000;
            attribute :>> exhaustion = "state event limit exceeded";
        }
        part doSteps : Budget {
            attribute :>> counts = "do-activity steps";
            attribute :>> envVar = "OPENSYSML_MAX_DO_STEPS";
            attribute :>> defaultBound = 5000000;
            attribute :>> exhaustion = "state do-step limit exceeded";
        }
        part elements : Budget {
            attribute :>> counts = "collection elements materialized";
            attribute :>> envVar = "OPENSYSML_MAX_ELEMENTS";
            attribute :>> defaultBound = 1000000;
            attribute :>> exhaustion = "collection element limit exceeded";
        }
        part calcDepth : Budget {
            attribute :>> counts = "nested calculation depth";
            attribute :>> envVar = "OPENSYSML_MAX_CALC_DEPTH";
            attribute :>> defaultBound = 10000;
            attribute :>> exhaustion = "calc recursion limit exceeded";
        }
        part sweepRuns : Budget {
            attribute :>> counts = "runs one parameter sweep or sample makes";
            attribute :>> envVar = "OPENSYSML_MAX_SWEEP_RUNS";
            attribute :>> defaultBound = 1000;
            attribute :>> exhaustion = "sweep run budget exceeded";
        }

        in item actionGraph : ActionGraph;
        in item stateGraph : StateGraph;
        out item trace : ExecutionTrace;
        port irIn : ArtifactPort;
        port traceOut : ArtifactPort;
        port requests : RequestPort;
    }

    // Discharges constraint sets through an external SMT solver.
    #Backend part def Solver :> Stage {
        attribute :>> goPackage = "internal/core/solve";
        attribute engines : String = "z3, cvc5";
        port requests : RequestPort;
    }

    // --- Analysis framework -----------------------------------------------

    // One registered way of answering a question: the kinds it answers, the bounds
    // it takes, the strongest evidence it can produce, and whether it needs a process.
    abstract part def AnalysisEngine :> CodeUnit {
        attribute :>> goPackage = "internal/core/analysis";
        attribute engineName : String;
        attribute answers : String;
        attribute bounds : String;
        attribute authority : String;
        attribute replays : Boolean default = false;
        attribute needsProcess : Boolean default = false;
        attribute adapts : String;
    }

    // One execution of the interpreter under stated bindings and schedule.
    part def RunEngine :> AnalysisEngine {
        attribute :>> engineName = "run";
        attribute :>> answers = "evaluate";
        attribute :>> bounds = "steps, elements";
        attribute :>> authority = "observed";
        attribute :>> replays = true;
        attribute :>> adapts = "the interpreter";
    }

    // Every linearization under an exploring schedule; proved when the
    // exploration completes, observed when a budget cut it short.
    part def ExploreEngine :> AnalysisEngine {
        attribute :>> engineName = "explore";
        attribute :>> answers = "outcomes";
        attribute :>> bounds = "runs, depth";
        attribute :>> authority = "proved";
        attribute :>> replays = true;
        attribute :>> adapts = "runtime.ExploreWith";
    }

    // Every schedule of one action, one token advancing one node at a time, under
    // static partial-order reduction; bounded within its depth and states, never proved.
    part def CheckEngine :> AnalysisEngine {
        attribute :>> engineName = "check";
        attribute :>> answers = "outcomes, holds";
        attribute :>> bounds = "depth, states, deadline, steps, actionSteps, events, doSteps, elements, behaviors";
        attribute :>> authority = "bounded";
        attribute :>> replays = true;
        attribute :>> adapts = "runtime.CheckAction";
    }

    // One run per row of a domain, on the surface's own context.
    part def SweepEngine :> AnalysisEngine {
        attribute :>> engineName = "sweep";
        attribute :>> answers = "sweep";
        attribute :>> bounds = "runs";
        attribute :>> authority = "observed";
        attribute :>> adapts = "Context.RunSweep";
    }

    // Satisfiability through an external SMT solver; registered whether or not
    // one is found, and refusing through coverage when none is.
    part def SolveEngine :> AnalysisEngine {
        attribute :>> engineName = "solve";
        attribute :>> answers = "satisfiable";
        attribute :>> bounds = "runs, solver";
        attribute :>> authority = "proved";
        attribute :>> replays = true;
        attribute :>> needsProcess = true;
        attribute :>> adapts = "internal/core/solve";
    }

    // What can be asked of a subject, and what a question may leave free.
    part def QuestionKinds {
        attribute kinds : String = "evaluate, outcomes, holds, sensitive, satisfiable, sweep, compute";
        attribute kindCount : Integer = 7;
        attribute freedoms : String = "nothing, the schedule, the inputs";
    }

    // What an answer claims and how strongly; a greater strength is never weaker
    // evidence, and an existential claim has witnessed as its one strength.
    part def EvidenceScale {
        attribute strengths : String = "not covered, observed, witnessed, bounded, proved";
        attribute strengthCount : Integer = 5;
        attribute claims : String = "none, holds, violated, sensitive, value, table, outcomes, satisfiable, unsatisfiable, unbounded";
        attribute claimCount : Integer = 10;
        attribute existentialsAreWitnessed : Boolean = true;
        attribute overclaimIsFault : Boolean = true;
    }

    // The engines one binary knows, held by their owner rather than in a global.
    part def EngineRegistry :> CodeUnit {
        attribute :>> goPackage = "internal/core/analysis";
        attribute engineCount : Integer = 5;
        attribute engineNames : String = "check, explore, run, solve, sweep";
        attribute perOwner : Boolean = true;
        attribute refusesDuplicateNames : Boolean = true;
        attribute listsInNameOrder : Boolean = true;
    }

    // How a question reaches its engines: auto consults the strongest covering one and
    // moves on from a not-covered answer, a name consults one, all composes every answer.
    part def Dispatcher :> CodeUnit {
        attribute :>> goPackage = "internal/core/analysis";
        attribute selections : String = "auto, all, <engine>";
        attribute defaultSelection : String = "auto";
        attribute strongestFirst : Boolean = true;
        // The default engines declaring each kind: outcomes has two, explore ranked over check.
        attribute candidatesPerKind : String = "evaluate: 1, holds: 1, outcomes: 2, satisfiable: 1, sweep: 1";
        attribute advancesOnNotCovered : Boolean = true;
        attribute stopsOnFault : Boolean = true;
        attribute recordsEveryStep : Boolean = true;
        attribute witnessRefutesUniversal : Boolean = true;
        in item question : AnalysisQuestion;
        out item plan : AnalysisPlan;
    }

    // The budget one plan runs under: jobs is the width of the fleet, the rest
    // bound one run in the engine's own unit.
    part def PlanBudget {
        attribute fields : String = "Deadline, Jobs, Runs, Depth, Steps, Solver, Memory";
        attribute fieldCount : Integer = 7;
        attribute jobsEnvVar : String = "OPENSYSML_JOBS";
        attribute jobsFlag : String = "-jobs";
        attribute jobsCommand : String = "%jobs";
        attribute defaultJobsIsCpuCount : Boolean = true;
        attribute rejectsNonPositiveJobs : Boolean = true;
    }

    // A plan's workers, one per job and built on first use, each with model-derived
    // state of its own over the shared frozen index; every run gets a fresh context.
    part def WorkerFleet :> CodeUnit {
        attribute :>> goPackage = "internal/core/analysis";
        attribute onePerJob : Boolean = true;
        attribute builtOnFirstUse : Boolean = true;
        attribute ownsRuntimeModel : Boolean = true;
        attribute sharesFrozenIndex : Boolean = true;
        attribute freshContextPerRun : Boolean = true;
        attribute recordsWarming : Boolean = true;
    }

    // One contract for every way of answering a question about a model, the engines
    // that implement it, and the dispatch that turns a question into a plan.
    #Analysis part def AnalysisFramework :> Stage {
        attribute :>> goPackage = "internal/core/analysis";
        attribute resultIndependentOfJobs : Boolean = true;

        part questions : QuestionKinds;
        part evidence : EvidenceScale;
        part registry : EngineRegistry;
        part dispatch : Dispatcher;
        part budget : PlanBudget;
        part workers : WorkerFleet;

        part run : RunEngine;
        part explore : ExploreEngine;
        part check : CheckEngine;
        part sweep : SweepEngine;
        part solve : SolveEngine;

        in item question : AnalysisQuestion;
        in item trace : ExecutionTrace;
        out item plan : AnalysisPlan;
        out item result : AnalysisResult;
        port questionsIn : RequestPort;
        port runtimeOut : RequestPort;
        port solverOut : RequestPort;
        port tracesIn : ArtifactPort;
        port resultsOut : ArtifactPort;
    }

    // --- The assembled pipeline ------------------------------------------

    part def AnalysisPipeline {
        part sources : SourceStore;
        part lexer : Lexer;
        part parser : Parser;
        part tree : SyntaxTreeStore;
        part symbols : SymbolBuilder;
        part resolver : Resolver;
        part semantics : SemanticModel;
        part passes : PassRegistry;
        part lowering : Lowering;
        part runtime : Runtime;
        part workspace : Workspace;
        part stdlib : StandardLibrary;
        part solver : Solver;
        part engines : AnalysisFramework;

        port textIn : ArtifactPort;
        port diagnosticsOut : DiagnosticPort;
        port questionsIn : RequestPort;

        connection textChannel : ArtifactChannel connect sources.textOut to lexer.textIn;
        connection tokenChannel : ArtifactChannel connect lexer.tokensOut to parser.tokensIn;
        connection treeChannel : ArtifactChannel connect parser.treeOut to tree.treeIn;
        connection symbolChannel : ArtifactChannel connect tree.treeOut to symbols.treeIn;
        connection indexChannel : ArtifactChannel connect symbols.indexOut to resolver.indexIn;
        connection stdlibChannel : ArtifactChannel connect stdlib.indexOut to resolver.indexIn;
        connection bindingChannel : ArtifactChannel connect resolver.bindingsOut to semantics.bindingsIn;
        connection typeChannel : ArtifactChannel connect semantics.typesOut to passes.typesIn;
        connection loweringChannel : ArtifactChannel connect semantics.typesOut to lowering.typesIn;
        connection irChannel : ArtifactChannel connect lowering.irOut to runtime.irIn;
        connection diagnosticChannel : DiagnosticChannel connect passes.diagnostics to workspace.diagnostics;
        connection questionChannel : RequestChannel connect questionsIn to engines.questionsIn;
        connection runChannel : RequestChannel connect engines.runtimeOut to runtime.requests;
        connection solveChannel : RequestChannel connect engines.solverOut to solver.requests;
        connection traceChannel : ArtifactChannel connect runtime.traceOut to engines.tracesIn;

        flow of SourceText from sources.text to lexer.text;
        flow of TokenStream from lexer.tokens to parser.tokens;
        flow of SyntaxTree from parser.tree to tree.tree;
        flow of SyntaxTree from tree.tree to symbols.tree;
        flow of SymbolIndex from symbols.index to resolver.index;
        flow of SymbolIndex from stdlib.index to resolver.index;
        flow of ResolutionTable from resolver.bindings to semantics.bindings;
        flow of TypeTable from semantics.types to passes.typed;
        flow of TypeTable from semantics.types to lowering.types;
        flow of ActionGraph from lowering.actions to runtime.actionGraph;
        flow of StateGraph from lowering.states to runtime.stateGraph;
        flow of ExecutionTrace from runtime.trace to engines.trace;
    }
}
