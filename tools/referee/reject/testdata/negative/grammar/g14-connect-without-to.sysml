// Invalid: BinaryConnectorPart requires 'to' between the two ends (SysML.xtext BinaryConnectorPart).
package G14ConnectWithoutTo {
    part def P {
        part a;
        part b;
        connect a b;
    }
}
