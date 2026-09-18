// The runtime from the inside: the instance layer that turns a type into objects one
// read at a time, the context and scheduler around it, and one action executed through them.
package OpenSysMLExecution {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;
    private import OpenSysMLPipeline::*;
    private import OpenSysMLSurfaces::*;

    // --- What the layer is made of -----------------------------------------

    // The members a type's specializations and redefinitions leave it with, as
    // the semantic model answers them; the schema below is built from it.
    item def TypeShape :> SideTable;

    // One entry of a type's flattened schema (runtime.EffectiveFeature): name, symbol, owner,
    // type, multiplicity, the stated value and its declaration, and whether it is a set, unique.
    item def EffectiveFeature {
        attribute fieldCount : Integer = 9;
        attribute inheritsDefault : Boolean = true;
        attribute inheritsMultiplicity : Boolean = true;
    }

    // A materialized object (runtime.Instance): an identity, the type it is of,
    // one feature value per schema entry, and the behaviors its type exhibits.
    item def Instance {
        attribute identityIsInteger : Boolean = true;
        attribute oneValuePerSchemaEntry : Boolean = true;
        attribute knowsItsOwner : Boolean = true;
    }

    // What one feature of one object holds (runtime.FeatureValue): the schema
    // entry, the scalar or the collection, and the flags saying how it got there.
    item def FeatureValue {
        attribute materialized : Boolean default = false;
        attribute written : Boolean default = false;
        attribute bindingDerived : Boolean default = false;
        attribute assumed : Boolean default = false;
    }

    item def ActionRequest;
    item def ActionDeclaration;
    item def ActionResults;
    item def ChoicePoint;
    item def Alternative;
    item def Expression;
    item def FeatureRead;
    item def FeatureWrite;
    item def HeldValue;

    // --- The units of the layer -----------------------------------------------

    // FeaturesOf: a type's schema, built once per model from the semantic shape and
    // shared by every object of the type; redefinitions stay in it, connector ends join it.
    part def FeatureSchema :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute :>> memoized = true;
        attribute keepsRedefinedFeatures : Boolean = true;
        attribute appendsConnectorEnds : Boolean = true;
        attribute excludesInputParameters : Boolean = true;
        in item typeShape : TypeShape;
        out item features : EffectiveFeature[*];
    }

    // materialize: an identity claimed, a feature value per schema entry with only
    // constant defaults folded, redefinitions aliased, the object registered and begun.
    part def Allocator :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute chargesStep : Boolean = true;
        attribute foldsConstantDefaults : Boolean = true;
        attribute aliasesRedefinitions : Boolean = true;
        attribute recordsTrace : Boolean = true;
        in item features : EffectiveFeature[*];
        out item object : Instance;
    }

    // GetFeatureValue: a value is made the first time it is read, by the case its
    // feature is in (OpenSysMLBehavior::ReadFeatureValue); composites are made the same way.
    part def LazyReader :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute :>> lazy = true;
        attribute refusesUnknownFeature : String = "object has no such feature";
        attribute refusesUnselectedVariation : String = "variation has no variant selected";
        attribute materializesCompositesRecursively : Boolean = true;
        attribute stopsShortOfOpenCollections : Boolean = true;
        in item object : Instance;
        in item bound : HeldValue;
        out item value : HeldValue;
    }

    // Bindings and connectors, lowered once per type: a bound value is read from
    // its far end and marked as propagated, never as written.
    part def BindingPropagator :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute :>> memoized = true;
        attribute lowersBindingsPerType : Boolean = true;
        attribute marksBindingDerived : Boolean = true;
        attribute ignoresZeroMultiplicityEnds : Boolean = true;
        out item value : HeldValue;
    }

    // A `=` value lists itself on every value it reads; a write to one of them
    // unmaterializes it, transitively, to derive again on its next read.
    part def DependencyTracker :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute recordsReads : Boolean = true;
        attribute invalidatesTransitively : Boolean = true;
        attribute rederivesWhenStale : Boolean = true;
        attribute exemptsWrittenValues : Boolean = true;
        attribute exemptsBoundValues : Boolean = true;
        in item value : FeatureValue;
    }

    // What a feature comes to hold passes multiplicity, type and uniqueness first,
    // and is spelt as the declared type prefers.
    part def Admission :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute multiplicityRefusal : String = "multiplicity violation";
        attribute typeRefusal : String = "type mismatch";
        attribute uniquenessRefusal : String = "uniqueness violation";
        attribute spellsPreferredUnit : Boolean = true;
        attribute holdsEnumerated : Boolean = true;
        attribute chargesCollectionElements : Boolean = true;
        in item value : HeldValue;
        out item admitted : FeatureValue;
    }

    // The instance layer: the schema of a type once, objects of it on demand, and
    // every value they hold read lazily, propagated, admitted and tracked.
    part def InstanceModel :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute :>> lazy = true;
        attribute lazyFeatureValues : Boolean = true;
        attribute schemaOncePerType : Boolean = true;

        part schema : FeatureSchema;
        part allocator : Allocator;
        part reader : LazyReader;
        part bindings : BindingPropagator;
        part admission : Admission;
        part dependencies : DependencyTracker;

        in item typeShape : TypeShape;
        out item instances : InstanceGraph;

        flow of EffectiveFeature from schema.features to allocator.features;
        flow of Instance from allocator.object to reader.object;
        flow of HeldValue from bindings.value to reader.bound;
        flow of HeldValue from reader.value to admission.value;
        flow of FeatureValue from admission.admitted to dependencies.value;
    }

    // --- Around the layer ------------------------------------------------------

    // runtime.Context: what one run makes — its objects, budgets, clock, trace and
    // the choices it took — over a model shared with every other run of the worker.
    part def RunContext :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute ownsObjects : Boolean = true;
        attribute ownsBudgets : Boolean = true;
        attribute recordsChoices : Boolean = true;
        attribute sharesModel : Boolean = true;
    }

    // Where a step has several enabled alternatives the policy picks one and the
    // run records a choice point, which a replay or an exploration takes differently.
    part def Scheduler :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute choiceKindCount : Integer = 9;
        attribute defaultPolicy : String = "reverse";
        attribute policySpellings : String = "declared, reverse, seed:<n>, explore[:runs=<n>,depth=<d>], replay:<file>";
        attribute recordsChoicePoints : Boolean = true;
    }

    // --- One action executed ---------------------------------------------------

    // A surface asks for an action: the context lowers it to a graph an executor walks, its
    // choices through the scheduler, guards through the evaluator, reads and writes the instances.
    occurrence def ExecuteAction {
        part surface : Repl {
            event occurrence requested;
            then event occurrence answered;
        }

        part context : RunContext {
            event occurrence received;
            then event occurrence lowering;
            then event occurrence lowered;
            then event occurrence started;
            then event occurrence completed;
            then event occurrence resultsCollected;
            then event occurrence answering;
        }

        part lowering : Lowering {
            event occurrence declarationTaken;
            then event occurrence graphBuilt;
        }

        part executor : ActionExecutor {
            event occurrence initialized;
            then event occurrence stepping;
            then event occurrence alternativeTaken;
            then event occurrence guardAsked;
            then event occurrence valueTaken;
            then event occurrence assigned;
            then event occurrence tokensAdvanced;
            then event occurrence finished;
        }

        part scheduler : Scheduler {
            event occurrence choiceOffered;
            then event occurrence choiceMade;
        }

        part evaluator : Evaluator {
            event occurrence evaluating;
            then event occurrence reading;
            then event occurrence valueRead;
            then event occurrence evaluated;
        }

        part instances : InstanceModel {
            event occurrence readAsked;
            then event occurrence materialized;
            then event occurrence written;
            then event occurrence invalidated;
        }

        message executeAction of ActionRequest from surface.requested to context.received;
        message toActionGraph of ActionDeclaration from context.lowering to lowering.declarationTaken;
        message actionGraph of ActionGraph from lowering.graphBuilt to context.lowered;
        message initialize of ActionGraph from context.started to executor.initialized;
        message resolveChoice of ChoicePoint from executor.stepping to scheduler.choiceOffered;
        message alternative of Alternative from scheduler.choiceMade to executor.alternativeTaken;
        message evaluate of Expression from executor.guardAsked to evaluator.evaluating;
        message getFeatureValue of FeatureRead from evaluator.reading to instances.readAsked;
        message featureValue of FeatureValue from instances.materialized to evaluator.valueRead;
        message value of HeldValue from evaluator.evaluated to executor.valueTaken;
        message setFeatureValue of FeatureWrite from executor.assigned to instances.written;
        message invalidateDependents of FeatureWrite from instances.written to instances.invalidated;
        message trace of ExecutionTrace from executor.tokensAdvanced to context.completed;
        message results of ActionResults from executor.finished to context.resultsCollected;
        message answer of ActionResults from context.answering to surface.answered;

        // A token advances once its write has landed and the values reading it are stale.
        succession first instances.invalidated then executor.tokensAdvanced;
    }
}
