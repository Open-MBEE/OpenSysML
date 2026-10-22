// This document's Meta::Safety annotates belt; a filter written here selects it.
package Meta { metadata def Safety; }
package LibA { #Meta::Safety part belt; }
package ClientA {
	private import LibA::*[@Meta::Safety];
	part a :> belt;
}
