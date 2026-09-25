// The SysML counterpart of f93_element_filter.kerml. `@SysML::PartDefinition`
// selects by metaclass here (the SysML kinds are mapped), so what is left is the
// metaclass *feature* read: `Element::name` is a feature of the candidate's
// metaclass, not of metadata annotating it (SysML v2 7.4.4, KerML 1.1 §8.2.4).
package F93Filtering {
	private import KerML::*;

	package DesignModel {
		part def System;
	}

	package ByMetaclass {
		private import DesignModel::*[@SysML::PartDefinition];

		part def Test :> System;
	}

	package ByMetaFeature {
		private import DesignModel::*;
		filter Element::name == "System" and not Type::isAbstract;

		part def Test :> System;
	}
}
