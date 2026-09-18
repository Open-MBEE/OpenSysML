// Declaration-shaped members whose only keyword is prefix metadata, or which
// carry no keyword at all: the member takes the kind of the usage that follows
// the annotations, and a declaration may be a value, a specialization or a
// redefinition on its own.
package PrefixMetadataAndKeywordlessMembers {
	metadata def M;
	metadata def N;
	metadata def service;
	metadata def original;

	port def PortDef;
	requirement def Req1;

	part a;
	part b;

	#M part x;
	#M #N part y;
	ref #M z1;
	abstract #M z2;
	#M connect a to b;

	part def P {
		#service sd : PortDef;
	}

	connection def D {
		end #original r1 : Req1;
	}

	attribute scalarQuantities;
	attribute distance;
	attribute volume;

	distancePerVolume :> scalarQuantities = distance / volume;
	T1 = 10.0;

	attribute def Collection {
		attribute elements : ScalarValues::Integer[*];
	}

	attribute def Bounded :> Collection {
		value :>> elements : ScalarValues::Integer;
	}

	enum def Speeds {
		enum slow;
		= 60.0;
	}

	locale "en_US" /* an anonymous comment carrying a locale */

	attribute def V {
		attribute m;
	}

	analysis def A {
		subject v : V;
		v.m
	}
}
