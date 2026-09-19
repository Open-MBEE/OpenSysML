package InheritedNameLibraryBaseClean {
	private import ModelingMetadata::Rationale;

	state S {
		state begin;
	}

	action def A {
		action redefines done;
		action finish;
	}

	part engine;
	metadata why : Rationale about engine {
		text = "the engine is the one the trade study selected";
	}
}
