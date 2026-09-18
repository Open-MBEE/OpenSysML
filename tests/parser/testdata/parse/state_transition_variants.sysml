state def TransitionVariants {
	state s1;
	state s2;
	state s3;
	
	// transition with trigger
	transition first s1 when event1 then s2;
	
	// `first` is optional, so a bare source may be followed by `then`
	transition s2 then s3;
	
	// General member (tests Phase B2 fallback)
	attribute stateData : String;
}
