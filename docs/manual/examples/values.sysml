package Values {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	enum def Phase { design; flight; }
	part def Mount { attribute mass :> ISQ::mass = 80 [kg]; }
	part def Telescope {
		part mount : Mount;
		attribute aperture : Real = 2.4;
		attribute designation : String = "Hubble";
		attribute flying : Boolean = true;
		attribute phase : Phase = Phase::flight;
		attribute bands : String[0..*] = ("optical", "ultraviolet");
	}
	part telescope : Telescope;

	part def ValuesReport :> Document {
		:>> title = "Telescope Values";

		rep language "markdown" /* The [](sysml:value:telescope.designation) telescope has a
		 * [](sysml:value:telescope.aperture) m aperture, is in its [](sysml:value:telescope.phase)
		 * phase (flying: [](sysml:value:telescope.flying)) and observes the
		 * [](sysml:value:telescope.bands) bands.
		 *
		 * - [Mount mass:](sysml:value:telescope.mount.mass)
		 * - [Nominal aperture:](sysml:value:Telescope::aperture)
		 */
	}
}
