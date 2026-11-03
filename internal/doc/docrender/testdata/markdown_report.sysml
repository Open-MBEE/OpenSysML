package Prose {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Telescope {
		part mount;
	}
	part telescope : Telescope;

	part def ProseReport :> Document {
		:>> title = "Prose Report";

		rep language "markdown" /* This report is *generated* by `sysml -render-document`
		 * ([OpenSysML](https://opensysml.org/)); see [](sysml:breakdown), the
		 * [mount](sysml:telescope.mount) and [the type](sysml:Telescope.metadata).
		 * Mass scales as $m \propto D^{2.5}_{\text{eff}}$ and each $ of budget
		 * buys about 1 cm^2; a literal \*star\* and &amp; stay prose, as does a | pipe.
		 *
		 * - first item with **strong** text
		 * - second item with a [link](<https://example.com/a b>)
		 *
		 * 1. one
		 * 2. two
		 */

		part breakdown : Section {
			:>> title = "Subsystem Masses";

			part reading : Paragraph {
				rep language "markdown" /* The **named** paragraph keeps its anchor; see [](sysml:classic). */
			}
			part classic : Paragraph {
				:>> text = "A classic paragraph beside the bodies.";
			}
			rep language "markdown" /* A section body closes the section. */
		}
	}
}
