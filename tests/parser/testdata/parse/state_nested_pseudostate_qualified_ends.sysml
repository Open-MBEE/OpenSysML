state def Line {
	entry; then Idle;
	state Idle;
	state Work {
		entry action { }
		then Prep;
		junction start;
		junction leave;
		fork spread;
		join gather;
		state Prep;
		state Run parallel {
			state a { entry; then a1; state a1; transition first a1 then gather; }
			state b { entry; then b1; state b1; transition first b1 then gather; }
		}
		transition first start then Run;
		transition first spread then Run::a::a1;
		transition first spread then Run::b::b1;
		transition first Prep accept Next then leave;
	}
	transition first Idle accept Go then Work::start;
	transition first Idle accept Split then Work::spread;
	transition first Work::leave do action effect { } then Idle;
	transition first Work::gather then Idle;
}
