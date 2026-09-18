package ThenPastNonFeatures {
	action def Step;
	part def Widget;
	metadata def Note;
	package Library {
		attribute def Marker;
	}

	// A positional `then` sequences from the nearest feature before it, so a
	// member that is not a feature is read past to the feature before it.
	action def AfterDoc {
		action a : Step;
		doc /* a is followed by b */
		then action b : Step;
	}

	action def AfterComment {
		action a : Step;
		comment /* a is followed by b */
		then action b : Step;
	}

	action def AfterCommentAbout {
		action a : Step;
		comment about a /* the first step */
		then action b : Step;
	}

	action def AfterRepresentation {
		action a : Step;
		rep inOtherWords language "text" /* a then b */
		then action b : Step;
	}

	action def AfterImport {
		action a : Step;
		private import Library::*;
		then action b : Step;
	}

	action def AfterAlias {
		action a : Step;
		alias Bump for Step;
		then action b : Step;
	}

	action def AfterNestedPartDef {
		action a : Step;
		part def Inner;
		then action b : Step;
	}

	action def AfterNestedActionDef {
		action a : Step;
		action def InnerStep;
		then action b : Step;
	}

	action def AfterNestedPackage {
		action a : Step;
		package Inner;
		then action b : Step;
	}

	action def AfterMultiplicity {
		action a : Step;
		multiplicity m [1];
		then action b : Step;
	}

	// Several non-feature members in a row are all read past.
	action def AfterSeveral {
		action a : Step;
		doc /* a is followed by b */
		part def Inner;
		private import Library::*;
		comment about a /* the first step */
		then action b : Step;
	}

	// A metadata usage is a feature, so it stays the source, as an attribute
	// or a part does.
	action def AfterMetadataAbout {
		action a : Step;
		metadata Note about a;
		then action b : Step;
	}

	action def AfterMetadataPrefix {
		action a : Step;
		@Note;
		then action b : Step;
	}

	action def AfterAttribute {
		action a : Step;
		attribute k : ScalarValues::Integer;
		then action b : Step;
	}

	action def AfterPart {
		action a : Step;
		part p : Widget;
		then action b : Step;
	}

	action def AfterAction {
		action a : Step;
		then action b : Step;
	}

	// The same rule holds in a state body, where a deferral is not a feature either,
	// and in a structural body.
	state def Lamp {
		state on;
		doc /* on precedes off */
		then state off;
		defer Ping;
		then state broken;
	}

	part def Structural {
		part a : Widget;
		doc /* a precedes b */
		part def Inner;
		then part b : Widget;
	}

	// A `then` that names its target is read by the same rule.
	action def ThenAlone {
		action a : Step;
		doc /* a is followed by b */
		then b;
		action b : Step;
	}
}
