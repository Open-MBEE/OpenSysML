// A keyword left in a parameter's name position names the parameter, quoted as
// KerML requires or bare, the way it does in any other declaration.
package ParameterKeywordNames {
	part def Anything;
	action def Classify {
		in 'type' : Anything;
		in ref state : Anything;
		out 'in' : Anything;
		in item scene : Anything;
		in seq : Anything[0..*] ordered nonunique;
	}
}
