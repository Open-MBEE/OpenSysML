package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	ref report : SystemReport;
	ref appendix : 'Mass Appendix';

	part def 'Mass Appendix' :> Document {
		attribute redefines title = "Mass Appendix";
		part overview : Paragraph {
			part back : Ref {
				attribute redefines text = "the report";
				ref redefines target = report;
			}
		}
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part masses : Section {
				attribute redefines title = "Subsystem Masses";
				part body : Paragraph {
					attribute redefines text = "Mass detail by subsystem.";
				}
			}
		}
	}

	part def SystemReport :> Document {
		attribute redefines title = "System Report";
		part intro : Paragraph {
			part lead : Span {
				attribute redefines text = "Full mass detail is tabulated in";
			}
			part see : Ref {
				ref redefines target = appendix.tables.masses;
			}
			part tail : Span {
				attribute redefines text = "of";
			}
			part whole : Ref {
				ref redefines target = appendix;
			}
		}
		part summary : Section {
			attribute redefines title = "Summary";
			part local : Paragraph {
				part here : Ref {
					ref redefines target = notes;
				}
			}
		}
		part notes : Section {
			attribute redefines title = "Notes";
			part body : Paragraph {
				attribute redefines text = "Margins are provisional.";
			}
		}
	}
}
