// A filter naming a metadata type through an `alias` member selects the
// elements the aliased type annotates: the alias is another name for one Type.
package AliasMeta {
	metadata def Safety;
	metadata def Comfort;
}
package AliasLib {
	#AliasMeta::Safety part belt;
	#AliasMeta::Comfort part radio;
}
package AliasNames {
	alias S for AliasMeta::Safety;
}
package AliasByImportFilter {
	private import AliasLib::*[@AliasNames::S];
	part a :> belt;
	part b :> radio;
}
package AliasByFilterMember {
	private import AliasLib::*;
	filter @AliasNames::S;
	part a :> belt;
	part b :> radio;
}
package AliasByExpose {
	view v {
		expose AliasLib::**[@AliasNames::S];
		part a :> belt;
		part b :> radio;
	}
}
