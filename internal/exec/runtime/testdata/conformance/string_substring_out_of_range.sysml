// A Substring position naming no character is reported rather than clamped to
// the string: "abc" has three characters, so there is no ninth.
package test {
	private import ScalarValues::*;
	private import StringFunctions::*;

	calc def pastTheEnd {
		in text : String;
		return : String = Substring(text, 1, 9);
	}
}
