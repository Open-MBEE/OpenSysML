// Deliberately invalid: `~Rail::'Frame *rail*'` is a conjugated port typing that names no member — the error is the point, pinning that a typing the renderer cannot resolve is still spelled as written and lists no resolved typing.
package Spelled {
	port def Link;

	part def 'Frame *rail*';

	part def Rig {
		part rail : 'Frame *rail*';
		port mirrored : ~Rail::'Frame *rail*';
	}
}

package SpelledViews {
	private import Views::*;

	view rigView {
		expose Spelled::Rig;
	}
}
