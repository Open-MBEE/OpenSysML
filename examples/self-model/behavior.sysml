// How the pipeline behaves: one analysis run, the tier gating that decides how
// far it gets, the two execution engines the runtime drives, and the analysis
// framework that turns a question into a plan.
package OpenSysMLBehavior {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;

    attribute def EditEvent;
    attribute def TokenArrival;
    attribute def SignalEvent;

    // One document analysed end to end. The tiers decide whether lowering and
    // execution are reached at all.
    action def AnalyzeDocument {
        attribute syntaxErrors : Integer = 0;
        attribute semanticErrors : Integer = 0;
        attribute executable : Boolean = true;

        in item text : SourceText;
        out item findings : Diagnostic;
        out item trace : ExecutionTrace;

        first start;

        action scan;
        action parse;
        action buildScopes;
        action resolveNames;
        action buildSemanticModel;

        action runSyntaxTier;
        action runNameTier;
        action runTypeTier;
        action runConstraintTier;

        action lower;
        action execute;
        action publish;

        succession first start then scan;
        succession first scan then parse;
        succession first parse then buildScopes;
        succession first buildScopes then resolveNames;
        succession first resolveNames then buildSemanticModel;
        succession first buildSemanticModel then runSyntaxTier;
        succession first runSyntaxTier then syntaxGate;

        // A failed tier stops the tiers above it; diagnostics are still published.
        decide syntaxGate;
        if syntaxErrors == 0 then runNameTier;
        else publish;

        succession first runNameTier then nameGate;

        decide nameGate;
        if semanticErrors == 0 then runTypeTier;
        else publish;

        succession first runTypeTier then typeGate;

        decide typeGate;
        if semanticErrors == 0 then runConstraintTier;
        else publish;

        succession first runConstraintTier then executionGate;

        decide executionGate;
        if executable then lower;
        else publish;

        succession first lower then execute;
        succession first execute then publish;
        succession first publish then done;
    }

    // The editor path: an edit reanalyses its own document immediately and
    // sweeps the other open ones once the burst settles.
    action def ServeEdit {
        in item edit : SourceText;
        out item findings : Diagnostic;

        first start;

        action applyEdit;
        action reanalyzeEdited;
        action publishEdited;
        action debounce;
        action sweepOpenDocuments;

        succession first start then applyEdit;
        succession first applyEdit then reanalyzeEdited;
        succession first reanalyzeEdited then split;

        fork split;
        succession first split then publishEdited;
        succession first split then debounce;
        succession first debounce then sweepOpenDocuments;
        succession first publishEdited then sync;
        succession first sweepOpenDocuments then sync;

        join sync;
        succession first sync then done;
    }

    // The library loaded once per process: the snapshot when it was written from
    // these files and decodes intact, the files otherwise.
    action def LoadLibrary {
        attribute snapshotCurrent : Boolean = true;
        attribute snapshotIntact : Boolean = true;

        in item snapshotBlob : LibrarySnapshot;
        out item index : SymbolIndex;

        first start;

        action digestFiles;
        action readHeader;
        action verifyChecksum;
        action decodeSnapshot;
        action parseFiles;
        action indexFiles;
        action expandImports;
        action freeze;
        action share;

        succession first start then digestFiles;
        succession first digestFiles then readHeader;
        succession first readHeader then digestGate;

        // A snapshot of other files, another format or an override parses.
        decide digestGate;
        if snapshotCurrent then verifyChecksum;
        else parseFiles;

        succession first verifyChecksum then checksumGate;

        decide checksumGate;
        if snapshotIntact then decodeSnapshot;
        else parseFiles;

        succession first parseFiles then indexFiles;
        succession first indexFiles then expandImports;
        succession first expandImports then freeze;
        succession first decodeSnapshot then share;
        succession first freeze then share;
        succession first share then done;
    }

    // The tier ladder as states: each tier is entered only from a clean tier
    // below it, and any failure halts short of execution.
    state def TierProgression {
        attribute failures : Integer = 0;
        attribute elementScopedPending : Boolean = false;

        entry; then syntaxTier;

        state syntaxTier;
        state nameTier;
        state typeTier;
        state constraintTier {
            entry; then documentScoped;

            state documentScoped;
            state elementScoped;

            // An element-scoped pass still runs for subjects that are not
            // downstream of the failure.
            transition documentScoped_to_elementScoped
                first documentScoped
                if elementScopedPending
                then elementScoped;
        }
        state executing;
        state halted;

        transition first syntaxTier if failures == 0 then nameTier;
        transition first syntaxTier if failures > 0 then halted;
        transition first nameTier if failures == 0 then typeTier;
        transition first nameTier if failures > 0 then halted;
        transition first typeTier if failures == 0 then constraintTier;
        transition first typeTier if failures > 0 then halted;
        transition first constraintTier if failures == 0 then executing;
        transition first constraintTier if failures > 0 then halted;
    }

    // One calc invocation: a scalar body — locals, returns, conditionals, library
    // intrinsics, named arguments — runs compiled unless the run is traced or an
    // argument is not a scalar or leaves a parameter unbound; the answer is the same.
    action def InvokeCalc {
        attribute tierEnabled : Boolean = true;
        attribute untraced : Boolean = true;
        attribute bodyIsScalar : Boolean = true;
        attribute scalarArguments : Boolean = true;

        first start;

        action bindArguments;
        action compileBody;
        action runCompiled;
        action boxResult;
        action evaluateBody;

        succession first start then bindArguments;
        succession first bindArguments then traceGate;

        // A traced run records every sub-expression, which only the evaluator does.
        decide traceGate;
        if tierEnabled and untraced then bodyGate;
        else evaluateBody;

        decide bodyGate;
        if bodyIsScalar then compileBody;
        else evaluateBody;

        succession first compileBody then argumentGate;

        decide argumentGate;
        if scalarArguments then runCompiled;
        else evaluateBody;

        succession first runCompiled then boxResult;
        succession first boxResult then done;
        succession first evaluateBody then done;
    }

    // The runtime tiers, from an effective feature list to a simulated model.
    state def RuntimeTiers {
        entry; then featureFlattening;

        state featureFlattening;
        state instanceModel;
        state expressionEvaluation;
        state behavioralModel;
        state behavioralExecution;

        transition first featureFlattening then instanceModel;
        transition first instanceModel then expressionEvaluation;
        transition first expressionEvaluation then behavioralModel;
        transition first behavioralModel then behavioralExecution;
    }

    // Token flow: a step moves every enabled token, and a round with no
    // movement is a deadlock rather than a hang. The budget is the action-step one.
    state def TokenFlow {
        attribute progressed : Boolean = true;
        attribute budgetLeft : Integer = 1000000;

        entry; then idle;

        state idle;
        state stepping;
        state blocked;
        state completed;

        transition first idle accept TokenArrival then stepping;
        transition first stepping if progressed and budgetLeft > 0 then idle;
        transition first stepping if not progressed then blocked;
        transition first stepping if budgetLeft <= 0 then blocked;
        transition first idle if not progressed then completed;
    }

    // Event dispatch: run-to-completion, with unhandled events deferred while a
    // deferring state is active.
    state def EventDispatch {
        attribute handled : Boolean = true;
        attribute deferring : Boolean = false;

        entry; then waiting;

        state waiting;
        state dispatching;
        state deferred;
        state settled;

        transition first waiting accept SignalEvent then dispatching;
        transition first dispatching if handled then settled;
        transition first dispatching if not handled and deferring then deferred;
        transition first deferred then waiting;
        transition first settled then waiting;
    }

    // Six budgets, each exhausted on its own: the first to run out ends the run
    // with the error that names it, and the others are not consulted.
    state def BudgetExhaustion {
        attribute stepsLeft : Integer = 10000000;
        attribute actionStepsLeft : Integer = 1000000;
        attribute stateEventsLeft : Integer = 1000000;
        attribute doStepsLeft : Integer = 5000000;
        attribute elementsLeft : Integer = 1000000;
        attribute calcDepthLeft : Integer = 10000;

        entry; then running;

        state running;
        state stepLimitExceeded;
        state actionStepLimitExceeded;
        state stateEventLimitExceeded;
        state doStepLimitExceeded;
        state elementLimitExceeded;
        state calcRecursionLimitExceeded;

        transition first running if stepsLeft <= 0 then stepLimitExceeded;
        transition first running if actionStepsLeft <= 0 then actionStepLimitExceeded;
        transition first running if stateEventsLeft <= 0 then stateEventLimitExceeded;
        transition first running if doStepsLeft <= 0 then doStepLimitExceeded;
        transition first running if elementsLeft <= 0 then elementLimitExceeded;
        transition first running if calcDepthLeft <= 0 then calcRecursionLimitExceeded;
    }

    // What a step costs, for the action-step budget the runtime enforces.
    calc def StepBudget {
        in attribute rounds : Integer;
        in attribute tokensPerRound : Integer;
        return attribute steps : Integer = rounds * tokensPerRound;
    }

    // One question answered: the selection decides which engines are consulted,
    // coverage is asked before any work, and every step lands in the plan.
    action def AnswerQuestion {
        attribute selectsAuto : Boolean = true;
        attribute selectsAll : Boolean = false;
        // The engines declaring the question's kind: one in the default registry,
        // where every engine declares a kind of its own.
        attribute candidates : Integer = 1;
        attribute enginesLeft : Integer = 0;
        attribute steps : Integer = 0;
        attribute covered : Boolean = true;
        attribute concluded : Boolean = true;
        attribute faulted : Boolean = false;

        in item question : AnalysisQuestion;
        out item plan : AnalysisPlan;
        out item result : AnalysisResult;

        first start;

        action rankByAuthority { assign enginesLeft := candidates; }
        action listByName { assign enginesLeft := candidates; }
        action pickNamed { assign enginesLeft := 1; }
        action askCoverage { assign enginesLeft := enginesLeft - 1; }
        action recordRefusal { assign steps := steps + 1; }
        action runEngine;
        action recordResult { assign steps := steps + 1; }
        action nextEngine;
        action compose;
        action recordFault { assign steps := steps + 1; }

        succession first start then selectionGate;

        // Auto ranks the engines declaring the kind strongest first; all consults
        // each of them in name order; a name consults exactly one.
        decide selectionGate;
        if selectsAuto then rankByAuthority;
        if selectsAll and not selectsAuto then listByName;
        else pickNamed;

        succession first rankByAuthority then askCoverage;
        succession first listByName then askCoverage;
        succession first pickNamed then askCoverage;
        succession first askCoverage then coverageGate;

        decide coverageGate;
        if covered then runEngine;
        else recordRefusal;

        succession first runEngine then faultGate;

        decide faultGate;
        if faulted then recordFault;
        else recordResult;

        succession first recordFault then done;
        succession first recordResult then stepRecorded;
        succession first recordRefusal then stepRecorded;

        merge stepRecorded;
        succession first stepRecorded then standingGate;

        // Auto stops at the first answer that is a claim and moves on from one that is
        // not; all goes on to every engine and composes at the end; a name is done.
        decide standingGate;
        if enginesLeft > 0 and (selectsAll or (selectsAuto and not concluded)) then nextEngine;
        else compose;

        succession first nextEngine then askCoverage;
        succession first compose then done;
    }

    // One run of the explore engine: a queue of schedule prefixes worked by a fleet of
    // workers, each run on a context of its own, until the queue empties or a budget cuts.
    action def ExploreOutcomes {
        attribute jobs : Integer = 1;
        attribute runsLeft : Integer = 1024;
        attribute depth : Integer = 64;
        attribute queued : Integer = 1;
        // The choice tree the runs walk: the choices still ahead, each met by the run
        // below the alternative the run before it left, and the alternatives each offers.
        attribute choicesAhead : Integer = 0;
        attribute alternatives : Integer = 2;
        attribute met : Integer = 0;
        // The prefixes the run just made leaves to explore, one per alternative it
        // did not take; a run below every choice leaves none.
        attribute unexplored : Integer = 0;
        attribute depthHit : Boolean = false;
        attribute complete : Boolean = true;

        in item question : AnalysisQuestion;
        out item result : AnalysisResult;

        first start;

        action warmWorker;
        action takePrefix { assign queued := queued - 1; }
        action freshContext;
        action runLinearization { assign runsLeft := runsLeft - 1; }
        action meetChoice {
            assign unexplored := alternatives - 1;
            assign met := met + 1;
            assign choicesAhead := choicesAhead - 1;
        }
        action passBeyond {
            assign depthHit := true;
            assign met := met + choicesAhead;
            assign choicesAhead := 0;
        }
        action runLeaf;
        action queueChoices {
            assign queued := queued + unexplored;
            assign unexplored := 0;
        }
        action sortOutcomes { assign complete := queued == 0 and not depthHit; }
        action claimProved;
        action claimObserved;

        succession first start then warmWorker;
        succession first warmWorker then takePrefix;
        succession first takePrefix then freshContext;
        succession first freshContext then runLinearization;
        succession first runLinearization then choiceGate;

        // A run meets the next choice within the depth bound and leaves its other
        // alternatives; beyond the bound it takes the first of every choice left.
        decide choiceGate;
        if choicesAhead > 0 and met < depth then meetChoice;
        if choicesAhead > 0 and met >= depth then passBeyond;
        else runLeaf;

        succession first meetChoice then runMade;
        succession first passBeyond then runMade;
        succession first runLeaf then runMade;

        merge runMade;
        succession first runMade then queueChoices;
        succession first queueChoices then queueGate;

        // The queue starts with the empty prefix and grows by what each run leaves
        // unexplored; exploration ends when it drains or the runs are spent.
        decide queueGate;
        if queued == 0 or runsLeft <= 0 then sortOutcomes;
        else takePrefix;

        succession first sortOutcomes then completionGate;

        // Every prefix visited within the bounds is a proof over every schedule; a
        // bound reached is the concrete runs that were made and no more.
        decide completionGate;
        if complete then claimProved;
        else claimObserved;

        succession first claimProved then done;
        succession first claimObserved then done;
    }

    // The evidence scale as a ladder: a result only ever moves up it, and a
    // universal claim with a bound reached comes back down to bounded.
    state def EvidenceLadder {
        attribute refused : Boolean = false;
        attribute concrete : Boolean = true;
        attribute existential : Boolean = false;
        attribute boundReached : Boolean = false;
        attribute exhaustive : Boolean = false;

        entry; then asked;

        state asked;
        state notCovered;
        state observed;
        state witnessed;
        state bounded;
        state proved;

        transition first asked if refused then notCovered;
        transition first asked if not refused and concrete and not existential then observed;
        transition first asked if not refused and existential then witnessed;
        transition first asked if not refused and not concrete and boundReached then bounded;
        transition first asked if not refused and not concrete and not boundReached and exhaustive then proved;
    }

    // A plan's fleet: a worker is built for a job on first use and kept for every later
    // run under that job; each run gets a fresh context on its worker.
    state def WorkerLifecycle {
        attribute built : Boolean = false;
        attribute runsPending : Integer = 1;

        entry; then idle;

        state idle;
        state warming;
        state ready;
        state running;
        state released;

        transition first idle if not built then warming;
        transition first idle if built then ready;
        transition first warming then ready;
        transition first ready if runsPending > 0 then running;
        transition first running do assign runsPending := runsPending - 1 then ready;
        transition first ready if runsPending <= 0 then released;
    }

    // A snapshot is a mark between steps: restore brings the run back to it as often as
    // asked until release, and a mark asked for mid-step or over a paused body is refused.
    state def SnapshotLifecycle {
        attribute betweenSteps : Boolean = true;
        attribute bodyPaused : Boolean = false;

        entry; then running;

        state running;
        state marked;
        state refusedMidStep;
        state refusedPausedBody;
        state released;

        transition first running if betweenSteps and not bodyPaused then marked;
        transition first running if not betweenSteps then refusedMidStep;
        transition first running if betweenSteps and bodyPaused then refusedPausedBody;
        transition first marked then running;
        transition first marked then released;
    }
}
