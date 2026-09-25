// The empty string is a String like any other: it has no character, concatenates
// to itself, and a substring selecting no character is it.
package test {
	private import ScalarValues::*;
	private import StringFunctions::*;

	calc def emptyIsAString {
		in blank : String;
		return : Boolean = Length(blank) == 0 and blank + blank == blank
			and Substring(blank, 1, 0) == blank;
	}
}
