// Deliberately non-conforming notation: the pilot grammar requires a visibility
// indicator before `import`, so each bare import here must be an error (D2).
package Lib {
	part def Widget;
}

package App {
	import Lib::*;
	part w : Widget;

	part def Holder {
		import Lib::Widget;
	}
}
