// A calc, constraint or state usage a part definition owns is a feature of the
// object, and its own nested features — a calc's inputs and body locals, a
// constraint's inputs and body locals, a state's entry action and substates — are
// features of it (SysML v2 §7.19.2, §7.20.2, §7.18.2), read through a feature chain
// as any other usage's members are (KerML 1.0 §7.3.4.6, §7.4.9.3): `rig.area.w` is the input
// the evaluation bound, `rig.fits.limit` the input the check binds, `rig.sm.e` the
// entry action the state holds. The usage itself and its `result` keep reading as
// the computation and the verdict.
package test {
	private import ScalarValues::*;

	part def Rig {
		attribute id : Integer = 7;
		calc area {
			in w : Real = 2.0;
			in h : Real = 3.0;
			attribute scale : Real = 10.0;
			return : Real = w * h;
		}
		constraint fits {
			in limit : Real = 100.0;
			attribute margin : Real = 5.0;
			area() < limit
		}
		state def M;
		state sm : M {
			entry action e { attribute who = this; }
			state idle;
			state busy;
			transition idle then busy;
		}
		attribute areaW = area.w;
		attribute areaH = area.h;
		attribute areaScale = area.scale;
		attribute areaResult = area.result;
		attribute fitsLimit = fits.limit;
		attribute fitsMargin = fits.margin;
		attribute fitsResult = fits.result;
	}
	part def Bench {
		part rig : Rig;
		attribute rigAreaW = rig.area.w;
		attribute rigAreaH = rig.area.h;
		attribute rigAreaScale = rig.area.scale;
		attribute rigAreaResult = rig.area.result;
		attribute rigFitsLimit = rig.fits.limit;
		attribute rigFitsMargin = rig.fits.margin;
		attribute rigFitsResult = rig.fits.result;
		attribute rigArea = rig.area;
		attribute rigFits = rig.fits;
	}
}
