// A subrequirement bound to no value, one value and several (SysML v2 §7.21.2, §8.4.17.1).
package BoundSubrequirementValues {
	requirement def Limit { subject v; attribute max; require constraint { v < max } }
	requirement def Container {
		subject v;
		requirement none[0..1] : Limit = ();
		requirement absent[0..1] : Limit = null;
		requirement one[0..1] : Limit = new Limit(max = 1);
		requirement several[*] : Limit = (new Limit(max = 1), new Limit(max = 2));
		requirement : Limit = ();
	}
}
