// An action-flow view positioned the way a migrated Cameo activity diagram is
// when its initial and final node symbols carry no Layout: a nested action's
// start and final have a Route, or a successor, and nothing else to place them.
// The unplaced start and done keep the colour and the notes their symbols carry.
package Imaging {
	action def Acquire {
		action position;
		action initiate;
		decide placed;
		action final terminate;

		action 'Position Not Correct' {
			succession 'start to adjust' first start then adjust;
			action final terminate;
			action adjust;
			succession 'adjust to final' first adjust then final;
		}

		action 'Vacuum Fault' {
			action troubleshoot;
			succession 'start to troubleshoot' first start then troubleshoot;
			succession 'troubleshoot to done' first troubleshoot then done;
		}

		succession 'start to position' first start then position;
		succession 'position to placed' first position then placed;
		succession 'placed to initiate' first placed then initiate;
		succession 'initiate to final' first initiate then final;
	}
}

package ImagingViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view acquireFlow : ActionFlowView {
		expose Imaging::Acquire;
		@Canvas { width = 500; height = 320; }
		metadata Layout about Imaging::Acquire::position { x = 21; y = 77; width = 142; height = 33; }
		metadata Layout about Imaging::Acquire::placed { x = 82; y = 170; width = 20; height = 20; }
		metadata Layout about Imaging::Acquire::initiate { x = 21; y = 220; width = 142; height = 33; }
		metadata Layout about Imaging::Acquire::final { x = 82; y = 290; width = 20; height = 20; }
		metadata Layout about Imaging::Acquire::'Position Not Correct'::adjust { x = 320; y = 118; width = 142; height = 33; }
		metadata Layout about Imaging::Acquire::'Position Not Correct'::final { x = 381; y = 180; width = 20; height = 20; }
		metadata Layout about Imaging::Acquire::'Vacuum Fault'::troubleshoot { x = 320; y = 240; width = 142; height = 33; }
		metadata Route about Imaging::Acquire::'start to position' { points = (92, 65, 92, 77); }
		metadata Route about Imaging::Acquire::'position to placed' { points = (92, 110, 92, 170); }
		metadata Route about Imaging::Acquire::'placed to initiate' { points = (92, 190, 92, 220); }
		metadata Route about Imaging::Acquire::'initiate to final' { points = (92, 253, 92, 290); }
		metadata Route about Imaging::Acquire::'Position Not Correct'::'start to adjust' { points = (386, 102, 386, 118); }
		metadata Route about Imaging::Acquire::'Position Not Correct'::'adjust to final' { points = (391, 151, 391, 180); }
		metadata Style about Imaging::Acquire::'Vacuum Fault'::start { fill = "#C00000"; }
		metadata Note about Imaging::Acquire::'Vacuum Fault'::start { text = "resumes after the fault"; x = 200; y = 200; width = 100; height = 30; }
		metadata Note about Imaging::Acquire::'Vacuum Fault'::done { text = "back to the flow"; x = 200; y = 290; width = 100; height = 30; }
	}
}
