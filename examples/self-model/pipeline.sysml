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

    // Where a unit sits in the architecture, for filtered views.
    metadata def Frontend;
    metadata def SemanticEngine;
    metadata def Backend;
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
        attribute passCount : Integer = 54;
        attribute sortsByLevel : Boolean = true;
        attribute skipsDocumentScopedAboveFailure : Boolean = true;

        part syntax : SyntaxCheck { attribute :>> goType = "SyntaxPass"; }
        part importVisibility : SyntaxCheck { attribute :>> goType = "ImportVisibilityPass"; }
        part enumerationBody : SyntaxCheck { attribute :>> goType = "EnumerationBodyPass"; }
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
        part relatedElements : ConstraintCheck { attribute :>> goType = "W10BRelatedElementsPass"; }
        part individualTyping : TypeCheck { attribute :>> goType = "W10BIndividualTypingPass"; }
        part endKinds : TypeCheck { attribute :>> goType = "W10BEndKindPass"; }
        part structuralElements : TypeCheck { attribute :>> goType = "W10BStructuralPass"; }
        part portionOwners : ConstraintCheck { attribute :>> goType = "W10BPortionOwnerPass"; }
        part verificationCases : ConstraintCheck { attribute :>> goType = "W8DVerificationPass"; }
        part viewRenderings : ConstraintCheck { attribute :>> goType = "W8DViewRenderingPass"; }
        part metadataUsages : TypeCheck { attribute :>> goType = "W8DMetadataUsagePass"; }
        part identityMetadata : ConstraintCheck { attribute :>> goType = "IdentityMetadataPass"; }
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
        attribute bundledFileCount : Integer = 98;
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

    // Petri-net token flow over the lowered action graph.
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
    // context into closures held in a side table; the evaluator stays the reference.
    part def CalcCompiler :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute :>> memoized = true;
        attribute overrideEnvVar : String = "OPENSYSML_CALC_COMPILE";
        attribute fallsBackToEvaluator : Boolean = true;
    }

    // Literals and invocation targets are memoized per context in side tables
    // keyed by syntax node; the tree itself carries nothing.
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

    // One bound on one run. Each is exhausted on its own, with a typed error
    // that names it, and each is overridable through its environment variable.
    part def Budget {
        attribute counts : String;
        attribute envVar : String;
        attribute defaultBound : Integer;
        attribute exhaustion : String;
    }

    #Backend part def Runtime :> Stage {
        attribute :>> goPackage = "internal/core/runtime";
        attribute stepBudgeted : Boolean = true;
        attribute budgetCount : Integer = 7;

        part instances : InstanceModel;
        part evaluator : Evaluator;
        part actions : ActionExecutor;
        part states : StateExecutor;

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
    }

    // Discharges constraint sets through an external SMT solver.
    #Backend part def Solver :> Stage {
        attribute :>> goPackage = "internal/core/solve";
        attribute engines : String = "z3, cvc5";
        port requests : RequestPort;
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

        port textIn : ArtifactPort;
        port diagnosticsOut : DiagnosticPort;

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
    }
}
