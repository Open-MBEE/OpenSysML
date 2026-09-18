state def EmptyBlocks {
	state s1;
	state s2;

	// An effect and a trailing body may each be braces holding nothing.
	transition first s1 do { } then s2;
	transition first s1 then s2 { }
	transition first s1 do { } then s2 { }
}
