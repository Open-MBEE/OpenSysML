library package StateMachines {
	doc
	/*
	 * NON-NORMATIVE OpenSysML vocabulary for state-machine notation the SysML v2
	 * grammar has no production for: pseudostates and deferred events. Each gets a
	 * definition specialising its native SysML type, a base usage, and a
	 * semantic-metadata keyword applying both at once: `#choice state pick;`
	 * declares a choice pseudostate, `#deferred ref : Ping;` an event the state
	 * defers while it is active.
	 */

	private import Metaobjects::SemanticMetadata;

	/* Pseudostates, by the kind of transient vertex they declare. */

	abstract state def Pseudostate {
		doc /* A Pseudostate is a transient vertex of a state machine: control passes
		 * through it on the way between states, so a transition reaching one leaves
		 * it in the same step. */
	}

	state def ChoicePseudostate :> Pseudostate {
		doc /* A ChoicePseudostate selects one of its outgoing transitions by the
		 * guard that holds where it is reached. */
	}

	state def JunctionPseudostate :> Pseudostate {
		doc /* A JunctionPseudostate chains transitions together, evaluating each
		 * guard as the chain is taken. */
	}

	state def ShallowHistoryPseudostate :> Pseudostate {
		doc /* A ShallowHistoryPseudostate re-enters the configuration its owning
		 * state was last in, at the depth the transition enters it. */
	}

	state def DeepHistoryPseudostate :> Pseudostate {
		doc /* A DeepHistoryPseudostate re-enters the configuration its owning
		 * state was last in, at every depth of the nesting below it. */
	}

	abstract state choicePseudostates : ChoicePseudostate[*] nonunique;

	abstract state junctionPseudostates : JunctionPseudostate[*] nonunique;

	abstract state shallowHistoryPseudostates : ShallowHistoryPseudostate[*] nonunique;

	abstract state deepHistoryPseudostates : DeepHistoryPseudostate[*] nonunique;

	/* Deferred events */

	abstract ref deferredEvents[*] nonunique {
		doc /* The events a state retains while it is active rather than dropping,
		 * delivered once a state that handles them is entered. */
	}

	/* Semantic metadata: the keywords spelling a state machine's transient
	 * vertices and deferred events as annotations on its members. */

	metadata def <choice> ChoiceMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::StateUsage;
		:>> baseType = choicePseudostates meta SysML::Usage;
	}

	metadata def <junction> JunctionMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::StateUsage;
		:>> baseType = junctionPseudostates meta SysML::Usage;
	}

	metadata def <shallowHistory> ShallowHistoryMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::StateUsage;
		:>> baseType = shallowHistoryPseudostates meta SysML::Usage;
	}

	metadata def <deepHistory> DeepHistoryMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::StateUsage;
		:>> baseType = deepHistoryPseudostates meta SysML::Usage;
	}

	metadata def <deferred> DeferredMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::ReferenceUsage;
		:>> baseType = deferredEvents meta SysML::Usage;
	}
}
