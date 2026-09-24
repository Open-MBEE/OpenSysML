package SatisfyChainSubject {
	part def Part;
	part def Config { part child : Part; }
	part def Assembly { part config : Config; }
	part config : Config;
	part assembly : Assembly;
	requirement r;

	satisfy r by config.child;
	satisfy r by assembly.config.child;
	assert satisfy r by config.child;
	assert not satisfy r by assembly.config.child;
	satisfy requirement conformance by config.child;
}
