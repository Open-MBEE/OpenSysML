// The Feature metaclass family read by specification name: typing, featuring,
// redefinition, subsetting, multiplicity and the ordering and composition flags.
package FeatureFamily {
    private import ScalarValues::*;
    part def Base {
        attribute mass : Real;
        part items : Item[*];
    }
    part def Derived :> Base {
        attribute :>> mass;
        part primary :> items : Item[1] ordered;
        ref part shared : Item;
    }
    part def Item;
}
