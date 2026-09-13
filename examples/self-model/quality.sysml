// The architecture invariants as requirements over the model, the test gates
// that verify them, and the allocation of logical stages onto the source tree.
package OpenSysMLInvariants {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;
    private import OpenSysMLPipeline::*;
    private import OpenSysMLSurfaces::*;
    private import VerificationCases::VerdictKind;

    #Invariant requirement def ImmutableSyntaxTree {
        doc /* The tree is syntax-only and never mutated after parsing. */
        subject tree : SyntaxTreeStore;
        require constraint {
            tree.immutableAfterParse and not tree.carriesSemantics
        }
    }

    #Invariant requirement def ParserNeverFails {
        doc /* Malformed input yields error nodes and diagnostics, never a panic. */
        subject parser : Parser;
        require constraint {
            parser.neverPanics and parser.emitsErrorNodes
        }
    }

    #Invariant requirement def LazyMemoizedSemantics {
        doc /* Name resolution computes on demand and caches. */
        subject resolver : Resolver;
        require constraint {
            resolver.lazy and resolver.memoized
        }
    }

    #Invariant requirement def TieredGating {
        doc /* Four tiers, and a pass that runs above a failed tier gates itself
             * per subject. */
        subject registry : PassRegistry;
        require constraint {
            registry.tierCount == 4 and registry.sortsByLevel
                and registry.skipsDocumentScopedAboveFailure
                and registry.notation.level == "syntax"
                and registry.actionEndpoints.elementScoped
        }
    }

    #Invariant requirement def LosslessLowering {
        doc /* Guards, triggers, effects and pseudostate edges survive lowering. */
        subject lowering : Lowering;
        require constraint {
            lowering.lossless
        }
    }

    #Invariant requirement def BoundedExecution {
        doc /* Every kind of work the runtime does has its own budget, and a run
             * that exhausts one reports which rather than hanging. */
        subject runtime : Runtime;
        require constraint {
            runtime.stepBudgeted and runtime.budgetCount == 7
                and runtime.evaluationSteps.defaultBound > 0
                and runtime.actionSteps.defaultBound > 0
                and runtime.stateEvents.defaultBound > 0
                and runtime.doSteps.defaultBound > 0
                and runtime.elements.defaultBound > 0
                and runtime.calcDepth.defaultBound > 0
                and runtime.sweepRuns.defaultBound > 0
        }
    }

    #Invariant requirement def LibraryConformance {
        doc /* Every bundled standard library file parses clean. */
        subject stdlib : StandardLibrary;
        require constraint {
            stdlib.bundledFileCount == 100
        }
    }

    #Invariant requirement def DerivedLibrarySnapshot {
        doc /* The embedded snapshot is derived from the library files, checked
             * against them before use, and never the only way to load them. */
        subject stdlib : StandardLibrary;
        require constraint {
            stdlib.snapshotEmbedded and stdlib.snapshotChecksummed
                and stdlib.fallsBackToParsing
        }
    }

    #Invariant requirement def ReferenceEvaluator {
        doc /* A compiled calc is an optimization of the evaluator, never a second
             * semantics: it can be switched off, and what it cannot take the evaluator runs. */
        subject evaluator : Evaluator;
        require constraint {
            evaluator.compilesPureCalcs implies
                (evaluator.compiled.memoized and evaluator.compiled.fallsBackToEvaluator)
        }
    }

    #Invariant requirement def RoundTrippableExport {
        doc /* A model written back out reloads as the same model. */
        subject exporter : Exporter;
        require constraint {
            exporter.roundTrips
        }
    }

    #Invariant requirement def OneAnalysisContract {
        doc /* Every question a surface asks goes through one contract: a registry the
             * owner holds, engines that declare what they answer and refuse before any
             * work, and dispatch that records every engine consulted in the plan. */
        subject framework : AnalysisFramework;
        require constraint {
            framework.registry.perOwner and framework.registry.refusesDuplicateNames
                and framework.registry.engineCount == 5
                and framework.dispatch.strongestFirst and framework.dispatch.advancesOnNotCovered
                and framework.dispatch.stopsOnFault and framework.dispatch.recordsEveryStep
                and framework.dispatch.defaultSelection == "auto"
        }
    }

    #Invariant requirement def HonestEvidence {
        doc /* An answer's strength never exceeds its engine's authority, an existential
             * claim is only ever witnessed, and a witness stands over the universal
             * claim it refutes. */
        subject framework : AnalysisFramework;
        require constraint {
            framework.evidence.strengthCount == 5 and framework.evidence.claimCount == 10
                and framework.evidence.existentialsAreWitnessed and framework.evidence.overclaimIsFault
                and framework.dispatch.witnessRefutesUniversal
                and framework.run.authority == "observed" and framework.sweep.authority == "observed"
                and framework.explore.authority == "proved" and framework.solve.authority == "proved"
        }
    }

    #Invariant requirement def IsolatedParallelRuns {
        doc /* Runs go concurrently on workers that share the frozen index and nothing
             * mutable: each worker has model-derived state of its own, each run a fresh
             * context, and the result of a check is the same at any job count. */
        subject framework : AnalysisFramework;
        require constraint {
            framework.resultIndependentOfJobs
                and framework.workers.onePerJob and framework.workers.ownsRuntimeModel
                and framework.workers.sharesFrozenIndex and framework.workers.freshContextPerRun
                and framework.budget.fieldCount == 8 and framework.budget.defaultJobsIsCpuCount
                and framework.budget.rejectsNonPositiveJobs
        }
    }

    #Invariant requirement def SnapshotsAreRunState {
        doc /* A snapshot captures what a run has made between two steps and no part of
             * the model it runs over; one asked for mid-step is refused, not approximated. */
        subject runtime : Runtime;
        require constraint {
            runtime.splitsModelFromRun
                and runtime.marks.capturesRunState and runtime.marks.excludesModelState
                and runtime.exploration.independentOfJobs and runtime.exploration.freshContextPerRun
        }
    }

    // Bound to the parts of the modelled toolchain, so the conditions are
    // evaluated against it rather than left abstract.
    requirement treeIsImmutable : ImmutableSyntaxTree {
        subject tree = opensysml.pipeline.tree;
    }

    requirement parserRecovers : ParserNeverFails {
        subject parser = opensysml.pipeline.parser;
    }

    requirement resolutionIsLazy : LazyMemoizedSemantics {
        subject resolver = opensysml.pipeline.resolver;
    }

    requirement tiersAreGated : TieredGating {
        subject registry = opensysml.pipeline.passes;
    }

    requirement loweringIsLossless : LosslessLowering {
        subject lowering = opensysml.pipeline.lowering;
    }

    requirement executionIsBounded : BoundedExecution {
        subject runtime = opensysml.pipeline.runtime;
    }

    requirement libraryIsClean : LibraryConformance {
        subject stdlib = opensysml.pipeline.stdlib;
    }

    requirement snapshotIsDerived : DerivedLibrarySnapshot {
        subject stdlib = opensysml.pipeline.stdlib;
    }

    requirement evaluatorIsReference : ReferenceEvaluator {
        subject evaluator = opensysml.pipeline.runtime.evaluator;
    }

    requirement exportRoundTrips : RoundTrippableExport {
        subject exporter = opensysml.exporter;
    }

    requirement questionsHaveOneContract : OneAnalysisContract {
        subject framework = opensysml.pipeline.engines;
    }

    requirement evidenceIsHonest : HonestEvidence {
        subject framework = opensysml.pipeline.engines;
    }

    requirement runsAreIsolated : IsolatedParallelRuns {
        subject framework = opensysml.pipeline.engines;
    }

    requirement snapshotsAreRunState : SnapshotsAreRunState {
        subject runtime = opensysml.pipeline.runtime;
    }

    concern def KeystrokeLatency {
        doc /* An editor keystroke re-analyses its own document immediately. */
        subject server : LanguageServer;
        require constraint {
            server.incrementalSync
        }
    }

    concern def ReferenceParity {
        doc /* Divergence from the OMG pilot is recorded in a baseline. */
        subject oracle : DifferentialOracle;
    }
}

// Each gate is the test run that decides whether an invariant still holds.
package OpenSysMLGates {
    private import ScalarValues::*;
    private import OpenSysMLInvariants::*;
    private import OpenSysMLPipeline::*;
    private import OpenSysMLSurfaces::*;
    private import VerificationCases::VerdictKind;

    verification def ParserGate {
        doc /* go test -run 'TestGolden|TestNegative' ./internal/core/parser */
        subject parser : Parser;
        objective {
            verify parserRecovers;
        }
        return attribute verdict : VerdictKind;
    }

    verification def StdlibGate {
        doc /* go test -run TestStdlibConformance ./internal/core/libs */
        subject stdlib : StandardLibrary;
        objective {
            verify libraryIsClean;
        }
        return attribute verdict : VerdictKind;
    }

    verification def SnapshotGate {
        doc /* make stdlib-snapshot-check; go test -run 'Snapshot' ./internal/core/libs ./internal/core/symbols */
        subject stdlib : StandardLibrary;
        objective {
            verify snapshotIsDerived;
        }
        return attribute verdict : VerdictKind;
    }

    verification def CalcDifferential {
        doc /* go test -run 'TestCompiledCalc' ./internal/core/runtime */
        subject evaluator : Evaluator;
        objective {
            verify evaluatorIsReference;
        }
        return attribute verdict : VerdictKind;
    }

    verification def RuntimeGate {
        doc /* go test -run 'TestExecutionConformance|TestRuntimeRobustness' ./internal/core/runtime */
        subject runtime : Runtime;
        objective {
            verify executionIsBounded;
        }
        return attribute verdict : VerdictKind;
    }

    verification def ExportGate {
        doc /* go test ./internal/core/export */
        subject exporter : Exporter;
        objective {
            verify exportRoundTrips;
        }
        return attribute verdict : VerdictKind;
    }

    verification def AnalysisGate {
        doc /* go test -race ./internal/core/analysis */
        subject framework : AnalysisFramework;
        objective {
            verify questionsHaveOneContract;
            verify evidenceIsHonest;
            verify runsAreIsolated;
        }
        return attribute verdict : VerdictKind;
    }

    verification def RunStateGate {
        doc /* go test -run 'TestSnapshot|TestExploreWith' ./internal/core/runtime */
        subject runtime : Runtime;
        objective {
            verify snapshotsAreRunState;
        }
        return attribute verdict : VerdictKind;
    }

    verification parserGate : ParserGate {
        subject parser = opensysml.pipeline.parser;
    }

    verification stdlibGate : StdlibGate {
        subject stdlib = opensysml.pipeline.stdlib;
    }

    verification snapshotGate : SnapshotGate {
        subject stdlib = opensysml.pipeline.stdlib;
    }

    verification calcDifferential : CalcDifferential {
        subject evaluator = opensysml.pipeline.runtime.evaluator;
    }

    verification runtimeGate : RuntimeGate {
        subject runtime = opensysml.pipeline.runtime;
    }

    verification exportGate : ExportGate {
        subject exporter = opensysml.exporter;
    }

    verification analysisGate : AnalysisGate {
        subject framework = opensysml.pipeline.engines;
    }

    verification runStateGate : RunStateGate {
        subject runtime = opensysml.pipeline.runtime;
    }

    // The contributor's loop, and who takes part in it.
    use case def LandAChange {
        subject change : SourceUnit;
        actor contributor : Contributor;
        actor ci : ContinuousIntegration;

        objective {
            doc /* The change builds, the gates stay green and the baselines
                 * move only deliberately. */
        }
    }

    part def Contributor;
    part def ContinuousIntegration;
    part def SourceUnit;
}

// Where the logical units actually live in the source tree.
package OpenSysMLCodebase {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;
    private import OpenSysMLPipeline::*;
    private import OpenSysMLSurfaces::*;

    part def Directory {
        attribute path : String;
    }

    part def SourceTree {
        part core : Directory {
            attribute :>> path = "internal/core";
        }
        part internals : Directory {
            attribute :>> path = "internal";
        }
        part commands : Directory {
            attribute :>> path = "cmd";
        }
        part clientLibraries : Directory {
            attribute :>> path = "clients";
        }
        part editors : Directory {
            attribute :>> path = "editors";
        }
    }

    part tree : SourceTree;

    allocation def UnitToDirectory {
        end unit : CodeUnit;
        end directory : Directory;
    }

    allocate opensysml.pipeline.lexer to tree.core;
    allocate opensysml.pipeline.parser to tree.core;
    allocate opensysml.pipeline.resolver to tree.core;
    allocate opensysml.pipeline.semantics to tree.core;
    allocate opensysml.pipeline.passes to tree.core;
    allocate opensysml.pipeline.lowering to tree.core;
    allocate opensysml.pipeline.runtime to tree.core;
    allocate opensysml.pipeline.engines to tree.core;
    allocate opensysml.pipeline.runtime.evaluator.compiled to tree.core;
    allocate opensysml.pipeline.stdlib to tree.core;
    allocate opensysml.pipeline.stdlib.encoding to tree.core;
    allocate opensysml.pipeline.stdlib.trees to tree.core;
    allocate opensysml.pipeline.stdlib.generator to tree.core;
    allocate opensysml.repl to tree.internals;
    allocate opensysml.lsp to tree.internals;
    allocate opensysml.service to tree.internals;
    allocate opensysml.python to tree.clientLibraries;
    allocate opensysml.rust to tree.clientLibraries;
    allocate opensysml.vscode to tree.editors;
}
