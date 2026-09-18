// A KerML type or feature named `chain` keeps its name before every relationship
// the grammar spells with a keyword: none of those words can name a type, so
// `chain` is not read as the feature chain modifier there.
package P {
	class Base;
	class chain specializes Base;
	struct chainStruct;
	struct chain specializes chainStruct;
	class chain conjugates Base;
	class chain disjoint from Base;
	class chain unions Base;
	feature other;
	feature chain chains other;
	feature chain ordered nonunique;
	feature chain link;
}
