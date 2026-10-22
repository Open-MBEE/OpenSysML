// Invalid: a subsetted feature must be accessible from the subsetting feature, whether the Subsetting is written inline or as a `specialization subset` member of its own (KerML 8.3.3.3.10 validateSubsettingFeaturingTypes; pilot validateSubsettingFeaturingTypes).
library package K77StandaloneSubsettingInaccessibleFeature {
    private import ScalarValues::Real;
    struct A { feature a : Real; }
    feature fu : A;
    specialization subset fu :> A::a;
}
