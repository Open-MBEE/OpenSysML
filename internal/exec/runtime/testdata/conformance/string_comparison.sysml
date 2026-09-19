// The comparison operators StringFunctions declares order strings by their
// characters, and the empty string precedes every other.
package test {
	private import ScalarValues::*;

	calc def precedes {
		in earlier : String;
		in later : String;
		return : Boolean = earlier < later and "" < earlier and earlier <= earlier;
	}
}
