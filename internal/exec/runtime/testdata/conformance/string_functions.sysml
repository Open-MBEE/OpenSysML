// StringFunctions::Length and Substring count characters rather than bytes, so
// the first two characters of a string holding a multi-byte one are both of them:
// "héllo" is 5 characters and 6 bytes.
package test {
	private import ScalarValues::*;
	private import StringFunctions::*;

	calc def firstTwo {
		in text : String;
		return : String = Substring(text, 1, Length("hé"));
	}
}
