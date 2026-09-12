// Invalid: A feature direction is written at most once; 'inout' is the single keyword for both (KerML.xtext:514 BasicFeaturePrefix, :506 FeatureDirection).
package K23RepeatedDirection {
    class Q {
        in out feature x;
    }
}
