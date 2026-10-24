package Prose {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	package Optics {
		part mirror;
		part mount;
	}

	part def AsciiDocReport :> Document {
		:>> title = "AsciiDoc Report";

		rep language "asciidoc" /* Plain _em_ *strong* `co\de` link:https://x.io/a[lbl] https://y.io
		 * sysml:ref[Optics::mirror] sysml:ref[Optics::mount, the mount] <<summary,the summary>> <<notes>> stem:[m \propto D^2] (C) end.
		 *
		 * * one
		 * * two _soft_
		 *
		 * NOTE: Mind the *gap*.
		 *
		 * . first
		 * . second
		 *
		 * .Masses
		 * [cols="3,1"]
		 * |===
		 * |Part |Mass
		 *
		 * |Mirror |12 kg
		 * |Mount |4 kg
		 * |===
		 *
		 * .Scaling
		 * [stem]
		 * ++++
		 * m \propto D^2
		 * ++++
		 *
		 * .Optics
		 * image::figures/optics.png[The optics]
		 */

		part summary : Paragraph {
			:>> text = "A classic paragraph beside the body.";
		}
		part notes : Paragraph {
			rep language "asciidoc" /* The *named* paragraph keeps its anchor; see <<summary>> and sysml:ref[Optics::mirror, the mirror]. */
		}
	}
}
