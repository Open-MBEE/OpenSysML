// Invalid: A cross feature's prefix writes a direction at most once; 'inout' is the single keyword for both (SysML.xtext:588 OwnedCrossFeature, :554 RefPrefix).
package G79CrossFeatureRepeatedDirection {
    part def A; part def B;
    connection def Conn {
        end in out x1 : A [0..1] item x : A;
        end y1 [0..1] item y : B;
    }
}
