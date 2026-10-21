// A filter naming a metadata type through an `alias` member selects the
// elements the aliased type annotates: the alias is another name for one Type.
package Meta {
	metadata def Safety;
	metadata def Comfort;
}
package Lib {
	#Meta::Safety part belt;
	#Meta::Comfort part radio;
}
package Names {
	alias S for Meta::Safety;
}
package ByImportFilter {
	private import Lib::*[@Names::S];
	part a :> belt;
	part b :> radio;
}
package ByFilterMember {
	private import Lib::*;
	filter @Names::S;
	part a :> belt;
	part b :> radio;
}
package ByExpose {
	view v {
		expose Lib::**[@Names::S];
		part a :> belt;
		part b :> radio;
	}
}
