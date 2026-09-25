// A migrated model, as the SysML v1 migrator writes it: elements its source
// left unnamed carry names the migration made up, listed by one
// MigrationMetadata::SynthesizedName usage per body.
package Migrated {
	private import ScalarValues::*;

	action def doTracking;

	action def Track {
		in target : Real;
		succession 'start to call' first start then call;
		action call : doTracking;
		succession 'call to fork' first call then 'fork';
		fork 'fork';
		succession 'fork to log' first 'fork' then log;
		action log;
		succession 'fork to final' first 'fork' then final;
		succession finish first log then final;
		action final terminate;
		metadata MigrationMetadata::SynthesizedName about 'start to call', 'call to fork', 'fork', 'fork to log', 'fork to final', final;
	}

	item def Go;
	item def Stop;

	state def Modes {
		entry; then Idle;
		state Idle;
		state Running;
		transition 'Idle accept Go then Running' first Idle accept Go then Running;
		transition halt first Running accept Stop then Idle;
		metadata MigrationMetadata::SynthesizedName about 'Idle accept Go then Running';
	}

	part def Engine;
	part def Wheel;

	part def Vehicle {
		part engine : Engine;
		part 'wheel' : Wheel;
		connection 'engine to wheel' connect engine to 'wheel';
		connection drive connect engine to 'wheel';
		metadata MigrationMetadata::SynthesizedName about 'engine to wheel', 'wheel';
	}
}

package MigratedViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view trackView : ActionFlowView {
		expose Migrated::Track;
		render asInterconnectionDiagram;
	}

	view modesView : StateTransitionView {
		expose Migrated::Modes;
	}

	view vehicleView {
		expose Migrated::Vehicle;
		render asInterconnectionDiagram;
	}

	view treeView {
		expose Migrated;
		render asTreeDiagram;
	}
}
