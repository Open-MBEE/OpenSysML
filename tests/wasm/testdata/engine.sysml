// A model the engine gate executes: an action with an output to read, and a
// two-state machine to run.
package enginedemo {
	private import ScalarValues::*;
	private import SI::*;

	enum def Color { red; green; }

	attribute speed : ISQ::SpeedValue = 10.0 [SI::m] / 2.0 [SI::s];

	attribute datum : ISQ::CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
	attribute posed : ISQ::CartesianSpatial3dCoordinateFrame {
		:>> mRefs = (mm, mm, mm);
		:>> transformation : MeasurementReferences::TranslationRotationSequence {
			:>> source = datum;
			:>> elements = (new MeasurementReferences::Translation((10.0, 0.0, 0.0) [datum]),
				new MeasurementReferences::Rotation((0.0, 0.0, 1.0) [datum], 90 ['°']));
		}
	}

	action Double {
		in x : Integer;
		out y : Integer;

		first start;
		action work { assign y := x * 2; }
		done;

		succession first start then work;
		succession first work then done;
	}

	state def Switch {
		entry; then off;

		state off;
		state on;

		transition off_to_on first off then on;
	}
}
