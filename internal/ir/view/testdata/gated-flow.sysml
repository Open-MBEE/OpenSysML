package Gated {
	private import ScalarValues::*;

	action def Pass {
		attribute go : Boolean = true;

		first start;
		action producer { out value : Integer; assign value := 1; }
		action consumer { in value : Integer; }

		succession first start then producer;
		first producer if go then f;
		succession flow f of Integer from producer.value to consumer.value;
	}
}

package GatedViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view passView : ActionFlowView {
		expose Gated::Pass;
		render asInterconnectionDiagram;
	}
}
