// Invalid: an end feature cannot have a direction (KerML 8.3.3.3.4 validateFeatureEndNoDirection; pilot validateFeatureEndNoDirection).
package S52EndWithDirection {
    part def A;
    connection def K1 { end in a : A; end b : A; }
    connection def K2 { end out a : A; end b : A; }
    part def G { end inout a : A; }
}
