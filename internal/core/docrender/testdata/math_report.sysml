package Optics {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Relation {
		attribute latex : String;
		attribute note : String;
	}

	part telescope {
		part focalRatio : Relation {
			attribute redefines latex = "N = \\frac{f}{D}";
			attribute redefines note = "f-number";
		}
		part resolution : Relation {
			attribute redefines latex = "\\theta = 1.22\\,\\frac{\\lambda}{D}";
			attribute redefines note = "Rayleigh criterion";
		}
	}

	calc def Relations :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = WhereType(source = OwnedElements(source = root), type = "Optics::Relation"),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("note", "latex")
		)
	}

	part def OpticsReport :> Document {
		attribute redefines title = "Optics Design Relations";

		part overview : Section {
			attribute redefines title = "Overview";

			part intro : Paragraph {
				part lead : Span {
					attribute redefines text = "The mirror's mass scales as";
				}
				part scaling : Span {
					attribute redefines text = " m \\propto D^{2.5}_{\\text{eff}} ";
					attribute redefines style = "math";
				}
				part cost : Span {
					attribute redefines text = "and each $ of budget buys about 1 cm^2 of aperture;";
				}
				part subscript : Span {
					attribute redefines text = "see";
					attribute redefines style = "emphasis";
				}
				part see : Ref {
					ref redefines target = mirrorArea;
				}
				part tail : Span {
					attribute redefines text = "and the";
				}
				part spec : Link {
					attribute redefines text = "aperture memo";
					attribute redefines target = "https://example.com/aperture";
				}
			}

			part mirrorArea : Formula {
				attribute redefines source = "A = \\pi \\left(\\frac{D}{2}\\right)^2\n\n  = \\frac{\\pi D^2}{4}\n";
				attribute redefines caption = "Collecting area of a circular mirror";
			}

			part dollars : Formula {
				attribute redefines source = "\\text{cost} = 10^6\\,\\$ \\times D^{2.5} + $";
			}
		}

		part relations : Section {
			attribute redefines title = "Design relations";

			part list : List {
				calc rows : Relations {
					in root = telescope;
				}
				part note : SpanColumn {
					attribute redefines column = "note";
				}
				part expr : SpanColumn {
					attribute redefines column = "latex";
					attribute redefines style = "math";
				}
			}
		}
	}
}
