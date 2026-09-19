// A kindless `name = value;` in a definition or usage body declares a reference
// usage of the object (SysML.xtext DefaultReferenceUsage), so the object carries
// it as a feature; only inside an enumeration body is such a member a literal.
package test {
    private import ScalarValues::*;

    part def Beam {
        attribute span : Real;
        doubled = span * 2.0;
        part end { offset = Beam::span; }
    }

    part def Rig {
        part beam : Beam { :>> span = 4.0; }
    }
}
