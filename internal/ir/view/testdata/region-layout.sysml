package RegionLayout {
	private import DiagramLayout::*;
	private import StandardViewDefinitions::*;

	state def Machine {
		entry; then Active;
		state Active parallel {
			state r1 {
				entry; then inside;
				state inside;
			}
		}
		state Busy parallel {
			state left {
				entry; then leftState;
				state leftState;
			}
			state right {
				entry; then rightState;
				state rightState;
			}
		}
	}

	state def Ctl parallel {
		state a {
			entry; then aState;
			state aState;
		}
		state b {
			entry; then bState;
			state bState;
		}
	}

	state def Parallel parallel {
		state r1 {
			entry; then inheritedInner;
			state inheritedInner;
		}
	}
	state usage : Parallel;

	view parallelLayouts : StateTransitionView {
		expose Machine;
		expose Ctl;
		metadata Layout about Machine::Active::r1 { x = 10; y = 20; width = 300; height = 100; }
		metadata Layout about Machine::Active::r1::inside { x = 12; y = 34; width = 56; height = 78; }
		metadata Layout about Machine::Busy::left { x = 110; y = 120; width = 310; height = 140; }
		metadata Layout about Machine::Busy::right { x = 210; y = 220; width = 320; height = 150; }
		metadata Layout about Ctl::a { x = 310; y = 320; width = 330; height = 160; }
		metadata Layout about Ctl::b { x = 410; y = 420; width = 340; height = 170; }
		metadata Style about Machine::Busy::left { fill = "#ff0000"; }
		metadata Note about Machine::Busy::left { text = "left region"; x = 450; y = 460; }
	}

	view inheritedLayout : StateTransitionView {
		expose usage;
		metadata Layout about usage::r1 { x = 510; y = 520; width = 350; height = 180; }
	}
}
