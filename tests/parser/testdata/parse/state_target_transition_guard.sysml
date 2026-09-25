state def GuardedTargets {
	attribute ready : ScalarValues::Boolean;
	state s1;
	if ready then s2;
	state s2;
	transition if not ready do action reset then s1;
	state s3;
	transition then s1;
}
