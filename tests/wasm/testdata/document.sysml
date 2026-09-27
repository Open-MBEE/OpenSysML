// The smallest document that renders, as docs/manual/getting-started.md gives it.
// -doc-form pdf drives an external converter, which is what the run gate checks a
// WebAssembly build refuses by name rather than failing obscurely.
package Hello {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	part def HelloReport :> Document {
		attribute redefines title = "Hello, Documents";

		part greeting : Paragraph {
			attribute redefines text = "This document was generated from a SysML v2 model.";
		}
	}
}
