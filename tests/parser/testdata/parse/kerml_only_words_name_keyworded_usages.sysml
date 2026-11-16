// SysML v2 8.2.2.1.2 does not reserve the KerML declaration keywords, so after a
// SysML kind keyword each is the usage's name whatever follows it
// (`action behavior subsets Parts::Part::ownedActions`, 8.4.13.6).
package P {
	part def T {
		attribute a;
		action behavior subsets a { }
		part class :> a;
		ref item struct redefines a;
		attribute feature = 1;
		part step [2];
		in item connector :>> a;
		individual part function subsets a;
		perform action predicate subsets a;
		exhibit state interaction subsets a;
		event occurrence datatype subsets a;
		snapshot classifier subsets a;
		timeslice type subsets a;
	}
	part def V {
		variation part p {
			variant part metaclass subsets p;
		}
	}
}
