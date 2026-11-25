package ActionForms {
	private import ScalarValues::*;

	attribute def Reading;
	attribute def Alarm;
	part def Sensor;
	part def Controller;

	action def Monitor {
		in sensor : Sensor;
		out alarm : Alarm;
		attribute count : Integer = 0;

		first start;
		action acceptReading accept reading : Reading;
		action sendAlarm send new Alarm() to sensor;
		action tally assign count := count + 1;
		action wait while count < 10 {
			action poll;
			action record;
			succession first poll then record;
		}
		action sweep for i in 1..3 {
			action step;
		}
		action choose if count > 5 {
			action raise;
		} else {
			action lower;
		}
		action final terminate;
		succession first start then acceptReading;
		succession first acceptReading then tally;
		succession first tally then sendAlarm;
		succession first sendAlarm then wait;
		succession first wait then sweep;
		succession first sweep then choose;
		succession first choose then final;
	}

	part def Plant {
		part sensor : Sensor {
			perform Monitor::acceptReading;
		}
		part controller : Controller {
			perform Monitor::tally;
		}
	}
}

package ActionFormViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view monitorView : ActionFlowView {
		expose ActionForms::Monitor;
		render asInterconnectionDiagram;
	}
}
