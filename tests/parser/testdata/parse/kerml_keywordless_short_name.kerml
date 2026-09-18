// A keyword-less feature (KerML.xtext Feature over Identification) may be
// identified by a short name; a SysML-only literal is an ordinary name in a
// KerML file, as a short name or a name.
package KeywordlessShortNames {
	<a> alpha = 1;
	<b> :> alpha = 2;
	<s> part = 1;
	<attribute> y = 2;
	<action> :> alpha = 3;
	class Tank {
		<l> limit : ScalarValues::Real = 10.0;
		<t> perform : ScalarValues::Integer = 4;
		var <v> port = 5;
	}
}
