// Invalid: A feature direction is written at most once; 'inout' is the single keyword for both (SysML.xtext:556 RefPrefix, :554 FeatureDirection).
package G73RepeatedDirection {
    port def Q {
        in out item x;
    }
}
