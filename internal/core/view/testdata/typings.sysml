package Spelled {
	port def Link;

	part def 'Frame *rail*';

	part def Mount;

	part def Cart;

	part def Rig {
		port plug : ~Link;
		part rail : 'Frame *rail*';
		part base typed by Mount, Cart;
		part root : $::Spelled::Mount;
	}
}

package SpelledViews {
	private import Views::*;

	view rigView {
		expose Spelled::Rig;
	}
}
