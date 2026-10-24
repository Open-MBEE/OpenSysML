// A filter naming a metadata type through a re-exporting import selects the
// elements the imported type annotates: the import surfaces the same Type.
package ReexportMeta {
	metadata def Safety;
	metadata def Comfort;
}
package ReexportLib {
	#ReexportMeta::Safety part belt;
	#ReexportMeta::Comfort part radio;
}
package ReexportNames {
	public import ReexportMeta::Safety;
}
package ReexportByImportFilter {
	private import ReexportLib::*[@ReexportNames::Safety];
	part a :> belt;
	part b :> radio;
}
package ReexportByFilterMember {
	private import ReexportLib::*;
	filter @ReexportNames::Safety;
	part a :> belt;
	part b :> radio;
}
