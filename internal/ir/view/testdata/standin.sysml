// Positioned action views around the nodes a migration makes up: a join it
// wrote several edges through, which stands for no source node, against a fork
// the source had but left unnamed; and pins that several routes meet or that
// no diagram positions.
package StandIns {
	private import ScalarValues::*;

	action def Emit {
		out value : Integer;
	}

	action def Take {
		in value : Integer;
	}

	action def Threaded {
		action a;
		action b;
		action c;
		join wait;
		succession 'a to wait' first a then wait;
		succession 'b to wait' first b then wait;
		succession 'wait to c' first wait then c;
		metadata MigrationMetadata::SynthesizedName about wait, 'a to wait', 'b to wait', 'wait to c';
		metadata MigrationMetadata::StandIn about wait;
	}

	action def Forked {
		action a;
		action b;
		action c;
		fork 'fork';
		succession 'a to fork' first a then 'fork';
		succession 'fork to b' first 'fork' then b;
		succession 'fork to c' first 'fork' then c;
		metadata MigrationMetadata::SynthesizedName about 'fork', 'a to fork', 'fork to b', 'fork to c';
	}

	action def Shared {
		action emit : Emit;
		action left : Take;
		action right : Take;
		action loose : Take;
		flow 'emit to left' from emit.value to left.value;
		flow 'emit to right' from emit.value to right.value;
		flow 'emit to loose' from emit.value to loose.value;
		metadata MigrationMetadata::SynthesizedName about 'emit to right';
	}
}

package StandInViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view threadedView : ActionFlowView {
		expose StandIns::Threaded;
		@Canvas { width = 300; height = 300; }
		metadata Layout about StandIns::Threaded { x = 0; y = 0; width = 300; height = 300; }
		metadata Layout about StandIns::Threaded::a { x = 20; y = 20; width = 80; height = 40; }
		metadata Layout about StandIns::Threaded::b { x = 160; y = 20; width = 80; height = 40; }
		metadata Layout about StandIns::Threaded::c { x = 20; y = 200; width = 80; height = 40; }
		metadata Route about StandIns::Threaded::'a to wait' { points = (60, 60, 60, 120); }
		metadata Route about StandIns::Threaded::'wait to c' { points = (60, 120, 60, 200); }
	}

	view forkedView : ActionFlowView {
		expose StandIns::Forked;
		@Canvas { width = 300; height = 300; }
		metadata Layout about StandIns::Forked { x = 0; y = 0; width = 300; height = 300; }
		metadata Layout about StandIns::Forked::a { x = 20; y = 20; width = 80; height = 40; }
		metadata Layout about StandIns::Forked::b { x = 20; y = 200; width = 80; height = 40; }
		metadata Layout about StandIns::Forked::c { x = 160; y = 200; width = 80; height = 40; }
		metadata Route about StandIns::Forked::'a to fork' { points = (60, 60, 60, 120); }
		metadata Route about StandIns::Forked::'fork to b' { points = (60, 120, 60, 200); }
	}

	view sharedView : ActionFlowView {
		expose StandIns::Shared;
		@Canvas { width = 400; height = 200; }
		metadata Layout about StandIns::Shared { x = 0; y = 0; width = 400; height = 200; }
		metadata Layout about StandIns::Shared::emit { x = 20; y = 20; width = 100; height = 40; }
		metadata Layout about StandIns::Shared::left { x = 20; y = 120; width = 100; height = 40; }
		metadata Layout about StandIns::Shared::right { x = 220; y = 120; width = 100; height = 40; }
		metadata Route about StandIns::Shared::'emit to left' { points = (50, 66, 50, 120); }
		metadata Route about StandIns::Shared::'emit to right' { points = (90, 66, 90, 90, 270, 90, 270, 120); }
	}
}
