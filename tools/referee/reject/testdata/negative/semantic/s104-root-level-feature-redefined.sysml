// Invalid: a feature owned by the root namespace is package-level and cannot be redefined by another (KerML 7.4.7 validateRedefinitionFeaturingTypes; pilot validateRedefinitionFeaturingTypes).
part def Engine;
part eng : Engine;
part eng2 : Engine :>> eng;
