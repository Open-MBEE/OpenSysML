// The operators StringFunctions declares: '+' concatenates its two String
// operands, which is how a model builds a message.
package test {
	private import ScalarValues::*;

	calc def greeting {
		in name : String;
		return : String = "Hello, " + name + "!";
	}
}
