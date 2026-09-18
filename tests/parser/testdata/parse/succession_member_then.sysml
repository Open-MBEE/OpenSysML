package MemberAttachedSuccession {
	action def Step;

	// A `then` prefixing a member sequences the member before it with the member
	// after it, so it is desugared into the same edge represented by
	// `succession first a then b;`. Here the sequencing is the reverse of declaration order.
	action def Behavioural {
		action c : Step;
		action b : Step;
		then action a : Step;
	}

	// The keyword may be written on its own line without changing which pair of
	// members it sequences.
	action def Wrapped {
		action one : Step;
		then
		action two : Step;
	}

	// A structural body takes the same prefix before an occurrence usage
	// (SysML.xtext DefinitionBodyItem), alongside the edge form it desugars to.
	part def Structural {
		part a;
		then part b;
		part c;
		succession first b then c;
	}
}
