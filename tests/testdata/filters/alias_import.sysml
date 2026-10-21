// A filter naming a metadata type through a re-exporting import selects the
// elements the imported type annotates: the import surfaces the same Type.
package Meta {
	metadata def Safety;
	metadata def Comfort;
}
package Lib {
	#Meta::Safety part belt;
	#Meta::Comfort part radio;
}
package Reexport {
	public import Meta::Safety;
}
package ByImportFilter {
	private import Lib::*[@Reexport::Safety];
	part a :> belt;
	part b :> radio;
}
package ByFilterMember {
	private import Lib::*;
	filter @Reexport::Safety;
	part a :> belt;
	part b :> radio;
}
