package Fixture {
	port def P;
	part def Whole {
		part a {
			port pa : P;
		}
		part b {
			port pb : P;
		}
		connection c connect a.pa to b.pb;
	}
}

package FixtureViews {
	private import Views::*;
	private import DiagramLayout::*;

	view dupIBD {
		expose Fixture::Whole;
		expose Fixture::Whole::a;
		expose Fixture::Whole::a::pa;
		expose Fixture::Whole::b::pb;
		render asInterconnectionDiagram;
		metadata Layout about Fixture::Whole { x = 0; y = 0; width = 400; height = 300; }
		metadata Layout about Fixture::Whole::a { x = 20; y = 40; width = 150; height = 100; }
		metadata Layout about Fixture::Whole::a::pa { x = 160; y = 80; width = 20; height = 20; }
		metadata Layout about Fixture::Whole::b::pb { x = 220; y = 80; width = 20; height = 20; }
	}

	view portsOnly {
		expose Fixture::Whole::a::pa;
		expose Fixture::Whole::b::pb;
		expose Fixture::Whole::c;
		render asInterconnectionDiagram;
		metadata Layout about Fixture::Whole::a::pa { x = 160; y = 80; width = 20; height = 20; }
		metadata Layout about Fixture::Whole::b::pb { x = 220; y = 80; width = 20; height = 20; }
	}

	view packageIBD {
		expose Fixture;
		expose Fixture::Whole;
		render asInterconnectionDiagram;
		metadata Layout about Fixture::Whole { x = 0; y = 0; width = 400; height = 300; }
		metadata Layout about Fixture::Whole::a { x = 20; y = 40; width = 150; height = 100; }
		metadata Layout about Fixture::Whole::a::pa { x = 160; y = 80; width = 20; height = 20; }
		metadata Layout about Fixture::Whole::b::pb { x = 220; y = 80; width = 20; height = 20; }
	}

	view orderedIBD {
		expose Fixture::Whole::a::pa;
		expose Fixture::Whole::b::pb;
		expose Fixture::Whole::a;
		expose Fixture::Whole;
		render asInterconnectionDiagram;
		metadata Layout about Fixture::Whole { x = 0; y = 0; width = 400; height = 300; }
		metadata Layout about Fixture::Whole::a { x = 20; y = 40; width = 150; height = 100; }
		metadata Layout about Fixture::Whole::a::pa { x = 160; y = 80; width = 20; height = 20; }
		metadata Layout about Fixture::Whole::b::pb { x = 220; y = 80; width = 20; height = 20; }
	}
}
