package Derivations {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import RequirementDerivation::*;

	requirement taggedParent { @original; }
	requirement plainParent;
	requirement childOfTagged;
	requirement childOfPlain;

	#derivation connection {
		end #original ::> taggedParent;
		end #derive ::> childOfTagged;
	}
	#derivation connection {
		end #original ::> plainParent;
		end #derive ::> childOfPlain;
	}

	calc def DerivedFrom :> Query {
		Project(
			source = WhereType(source = OwnedElements(source = Named(qualifiedName = "Derivations")), type = "RequirementUsage"),
			properties = ("name"),
			columns = (RelatedColumn(name = "derivedFrom", relationshipKind = "derivation", direction = "incoming", maxDepth = 1))
		)
	}
}
