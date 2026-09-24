// Invalid: A connector must have at least two related elements (KerML 8.3.3.1 validateConnectorRelatedElements; pilot validateConnectorRelatedFeatures, Connector_Invalid.sysml.xt).
package P20ConnectionWithOneEnd {
    part def B;
    part b0 : B;
    part p {
        connection { end ::> b0; }
    }
}
