// Deliberately invalid: `~rail.'Frame *rail*'` is a conjugated feature-chain typing, which SysML has no form for and which names no member — the error is the point, pinning that a typing the renderer cannot resolve is still spelled as written and lists no resolved typing.
package Spelled {
	port def Link;

	part def 'Frame *rail*';

	part def Rig {
		part rail : 'Frame *rail*';
		port mirrored : ~rail.'Frame *rail*';
	}
}

package SpelledViews {
	private import Views::*;

	view rigView {
		expose Spelled::Rig;
	}
}
