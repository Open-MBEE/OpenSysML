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

package Compartment {
	part def Owner {
		attribute a : Real;
		attribute b : Real;
		part leg;
	}
	part def Outer;
}

package Quoted {
	action def 'First Line\nSecond Line';
	part def 'It\'s';
	part def 'back\\slash';
	part def 'ctl\bx\fy\rz';
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

	// Places one X and leaves the other without a position: the omitted node
	// does not force a qualifier on the name of the one drawn.
	view cameoPartialView {
		expose Named::A::X;
		expose Named::B::X;
		metadata Layout about Named::A::X { x = 0; y = 0; }
	}

	// Boxes a and b inside Owner's box, where they draw as compartment rows;
	// leg stands outside it, and Outer is a root of its own.
	view compartmentView {
		expose Compartment::Owner;
		expose Compartment::Owner::a;
		expose Compartment::Owner::b;
		expose Compartment::Owner::leg;
		expose Compartment::Outer;
		metadata Layout about Compartment::Owner { x = 0; y = 0; width = 300; height = 200; }
		metadata Layout about Compartment::Owner::a { x = 10; y = 30; width = 80; height = 16; }
		metadata Layout about Compartment::Owner::b { x = 10; y = 50; width = 80; height = 16; }
		metadata Layout about Compartment::Owner::leg { x = 350; y = 20; width = 60; height = 30; }
		metadata Layout about Compartment::Outer { x = 450; y = 20; width = 80; height = 40; }
	}

	// The same owner and members with no position anywhere.
	view unplacedCompartmentView {
		expose Compartment::Owner;
		expose Compartment::Owner::a;
		expose Compartment::Owner::b;
	}

	// A stated note box round a node paints behind it; the small note stays on top.
	view noteFrameView {
		expose Compartment::Outer;
		metadata Layout about Compartment::Outer { x = 20; y = 20; width = 60; height = 40; }
		@Note { text = "frame"; x = 0; y = 0; width = 200; height = 120; }
		@Note { text = "small"; x = 400; y = 400; width = 50; height = 30; }
	}

	// An inner frame note declared before the outer one still paints after it.
	view nestedNoteFrameView {
		expose Compartment::Outer;
		metadata Layout about Compartment::Outer { x = 40; y = 40; width = 20; height = 20; }
		@Note { text = "inner"; x = 20; y = 20; width = 100; height = 100; }
		@Note { text = "outer"; x = 0; y = 0; width = 200; height = 200; }
	}

	// Quoted names whose escapes decode in the label.
	view escapedView {
		expose Quoted::'First Line\nSecond Line';
		expose Quoted::'It\'s';
		expose Quoted::'back\\slash';
		expose Quoted::'ctl\bx\fy\rz';
	}

	// Leaves the first exposed element unplaced: the frame header names the
	// first root drawn, not the first exposed.
	view cameoUnplacedFirstView {
		expose Named::B::X;
		expose Named::A::X;
		metadata Layout about Named::A::X { x = 0; y = 0; }
	}
}
