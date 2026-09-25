package Scoped {
	private import DiagramLayout::*;

	// A part def holding a nested view with a free note of its own, and two
	// members a positioned drawing places unevenly.
	part def Housing {
		part placed;
		part loose;
		view detail {
			expose Housing;
			@Note { text = "nested"; x = 5; y = 5; }
		}
	}
}

package Dup {
	part def Whole {
		part member;
	}
}

package Named {
	package A {
		part def X;
	}
	package B {
		part def X;
	}
}

package Elsewhere {
	part def Y;
}

package PositionedViews {
	private import Views::*;
	private import DiagramLayout::*;

	// Draws the note the nested view states for itself nowhere.
	view outerView {
		expose Scoped::Housing;
	}

	// Places one member and leaves one without a position; the note anchored to
	// the unplaced member draws no box under UnplacedOmit.
	view partialView {
		expose Scoped::Housing;
		metadata Layout about Scoped::Housing::placed { x = 10; y = 10; width = 100; height = 50; }
		metadata Note about Scoped::Housing::loose { text = "stray"; x = 0; y = 0; }
	}

	// Exposes a member beside its owner: the tree draws member under Whole,
	// not a second time as a root.
	view dupView {
		expose Dup::Whole;
		expose Dup::Whole::member;
		metadata Layout about Dup::Whole { x = 0; y = 0; width = 300; height = 200; }
		metadata Layout about Dup::Whole::member { x = 20; y = 20; width = 80; height = 40; }
	}

	// Names its scattered roots by their shortest distinguishing suffixes.
	view cameoView {
		expose Named::A::X;
		expose Named::B::X;
		expose Elsewhere::Y;
		metadata Layout about Named::A::X { x = 0; y = 0; }
		metadata Layout about Named::B::X { x = 200; y = 0; }
		metadata Layout about Elsewhere::Y { x = 400; y = 0; }
	}
}
