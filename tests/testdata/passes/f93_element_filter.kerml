// A reduction of kerml-examples `Simple Tests/Filtering.kerml` keeping its
// import and filter structure: the pinned validate-kerml is silent on it, while
// ours drops every candidate of the two filters below. `@Structure` asks what
// the candidate *is*, and `Element::name`/`Type::isAbstract`/`Feature::isComposite`
// read features of its metaclass (KerML 1.1 §8.2.4 ElementFilterMembership).
package F93Filtering {
	private import ScalarValues::*;

	package Annotations {
		metaclass ApprovalAnnotation {
			approved : Boolean;
			level : Natural;
		}
	}

	package DesignModel {
		private import Annotations::*;
		struct System {
			@ApprovalAnnotation {
				approved = true;
				level = 2;
			}
		}
		composite feature system : System;
	}

	private import KerML::*;

	package ByMetaclass {
		private import DesignModel::**[@Structure];

		struct Test :> System;
	}

	package ByMetaFeature {
		private import DesignModel::*;
		filter (Element::name == "System" and not Type::isAbstract) or
		       Feature::isComposite;

		struct Test :> System;
		feature :> system;
	}
}
