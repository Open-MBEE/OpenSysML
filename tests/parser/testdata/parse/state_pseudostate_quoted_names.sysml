state def Cycle {
	attribute fork : ScalarValues::Integer;
	entry; then Idle;
	state Idle;
	fork 'spread 2';
	join 'gather 2';
	junction 'route 2';
	choice 'pick 2';
	history 'checkpoint 2';
	deep history 'deepest 2';
	transition first Idle then 'spread 2';
	transition first 'gather 2' then 'checkpoint 2';
	transition first 'deepest 2' then Idle;
}
