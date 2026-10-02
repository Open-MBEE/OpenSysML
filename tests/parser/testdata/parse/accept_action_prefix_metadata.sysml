// An accept node takes prefix metadata like any usage (SysML.xtext
// PrefixMetadataMember): named or anonymous, with a body or without, and
// through `$::` where a package shadows the metadata's library.
package AcceptPrefixMetadata {
	metadata def M;
	item def Scene;
	port def Camera;
	action def Watch {
		port lens : Camera;
		#M action framed accept scene : Scene via lens;
		#M action <f2> framed2 accept scene : Scene;
		#M accept scene : Scene;
		#$::AcceptPrefixMetadata::M action marked accept scene : Scene { }
		then action done accept scene : Scene;
	}
}
