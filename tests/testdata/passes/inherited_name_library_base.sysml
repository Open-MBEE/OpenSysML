// A member reusing a name the one library definition its owner conforms to
// already supplies: `start` is StatePerformances::StateAction::start and `done`
// is Actions::Action::done, so each declaration below is indistinguishable from
// the feature it inherits (KerML 8.2.4 distinguishability).
package InheritedNameLibraryBase {
	state S {
		state start;
	}

	action def A {
		action done;
	}
}
