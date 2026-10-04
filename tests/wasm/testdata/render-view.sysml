package RenderDemo {
	port def Signal;

	part def Sender {
		port out1 : Signal;
	}

	part def Receiver {
		port in1 : ~Signal;
		port spare : Signal;
	}

	part def Link {
		part sender : Sender;
		part receiver : Receiver;
		interface wire connect sender.out1 to receiver.in1;
	}
}

package RenderViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view link {
		expose RenderDemo::Link;
		render asInterconnectionDiagram;
	}

	view geometryView : GeometryView {
		expose RenderDemo::Link;
	}
}
