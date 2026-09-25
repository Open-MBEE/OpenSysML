// Invalid: A connector end requires a referenced feature (KerML.xtext:854 ConnectorEnd).
package K18ConnectorEndWithoutReference {
    class C {
        feature a : C;
        feature b : C;
        connector c from a to ;
    }
}
