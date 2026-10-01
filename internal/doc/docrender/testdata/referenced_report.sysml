package Referenced {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	part def Mount;
	part telescope {
		part mount : Mount;
	}

	calc def Parts :> Query {
		in root : Element;
		Project(
			source = OwnedElements(source = root),
			properties = ("name")
		)
	}

	part def ReferencedReport :> Document {
		attribute redefines title = "Cross-References";

		part overview : Section {
			attribute redefines title = "Overview";

			part prose : Paragraph {
				part lead : Span {
					attribute redefines text = "The mount is aligned in";
				}
				part alignment : Ref {
					ref redefines target = procedures.alignment;
				}
				part conjunction : Span {
					attribute redefines text = "and listed in";
				}
				part parts : Ref {
					ref redefines target = procedures.alignment.parts;
				}
				part shown : Span {
					attribute redefines text = "and shown in";
				}
				part photo : Ref {
					ref redefines target = procedures.alignment.photo;
				}
				part open : Span {
					attribute redefines text = "(";
				}
				part mount : Ref {
					ref redefines target = telescope.mount;
				}
				part close : Span {
					attribute redefines text = "), a";
				}
				part kind : Ref {
					ref redefines target = Mount.metadata;
				}
				part stop1 : Span {
					attribute redefines text = ".";
				}
				part named : Ref {
					attribute redefines text = "The procedures";
					ref redefines target = procedures;
				}
				part stop : Span {
					attribute redefines text = "follow.";
				}
			}
		}

		part procedures : Section {
			attribute redefines title = "Procedures";

			part alignment : Section {
				attribute redefines title = "Alignment";

				part parts : Table {
					attribute redefines caption = "Parts aligned";
					calc rows : Parts {
						in root = telescope;
					}
				}

				part photo : Image {
					attribute redefines location = "images/mount.png";
					attribute redefines caption = "The mount, aligned";
				}
			}
		}
	}
}
